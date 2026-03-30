package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ravensync/ravensync/internal/llm"
	"github.com/ravensync/ravensync/internal/memory"
	"github.com/ravensync/ravensync/internal/metrics"
	"github.com/ravensync/ravensync/internal/ui"
)

const (
	topK           = 5
	historySize    = 10
	maxStreamRetry = 2
	// maxVisionUserTurnsInContext is how many past user turns may still include raw
	// image bytes in a single API call (newest first). Older photo turns become text-only
	// in that request only, to limit payload size; in-memory history still keeps bytes
	// until the sliding window drops them.
	maxVisionUserTurnsInContext = 5

	systemPrompt = `You are Ravensync, a personal AI assistant with persistent memory. You remember things about the user across conversations and platforms (console, Telegram, etc).

MEMORY RULES:
- You will be given "Retrieved memories" below. These are REAL things you stored previously. TRUST THEM COMPLETELY.
- When the user asks a question and a retrieved memory contains the answer, respond CONFIDENTLY. Example: if a memory says "user's name is Alex" and they ask "what's my name?", answer "Your name is Alex!" — do NOT hedge or say "I think".
- NEVER say "I don't remember" or "you didn't tell me" if a retrieved memory has the answer.
- If NO retrieved memory matches the question, be honest: say you don't have that info yet and invite them to share it.

CONVERSATION:
- You will receive recent conversation history. ALWAYS use it to understand context.
- Short messages like "why?", "what do you think?", "really?" ALWAYS refer to the previous topic. Never ignore context and give a generic response.
- Stay on topic until the user clearly changes the subject.

GENERAL:
- Be concise, warm, and helpful.
- Use the user's name naturally if you know it from memories.
- Do NOT say "I'll remember that" or mention memory unless the user EXPLICITLY asks you to remember something. Just respond naturally.

VISION:
- When the user's message includes images (one or several across the thread), describe what you see accurately and answer questions. Compare images when the user sent more than one. Combine this with retrieved memories and recent conversation history.

STRUCTURED QUESTIONS:
- When you need the user to choose from options, format EXACTLY like this at the END of your message:
  [CHOOSE: option one | option two | option three]
- Only use this when choices genuinely help. Most responses should be normal text.`
)

type Agent struct {
	store    *memory.Store
	embedder memory.Embedder
	llm      llm.Provider
	metrics  *metrics.Collector
	logger   *slog.Logger
	events   *ui.EventBus

	mu      sync.Mutex
	history map[string][]llm.Message
}

func New(store *memory.Store, embedder memory.Embedder, llmProvider llm.Provider, collector *metrics.Collector, logger *slog.Logger, events *ui.EventBus) *Agent {
	return &Agent{
		store:    store,
		embedder: embedder,
		llm:      llmProvider,
		metrics:  collector,
		logger:   logger,
		events:   events,
		history:  make(map[string][]llm.Message),
	}
}

func (a *Agent) Store() *memory.Store       { return a.store }
func (a *Agent) Metrics() *metrics.Collector { return a.metrics }

func embedQueryText(text string, images []llm.ImagePart) string {
	t := strings.TrimSpace(text)
	if t != "" {
		return t
	}
	if len(images) > 0 {
		return "The user sent an image."
	}
	return ""
}

func userTurnForHistory(text string, images []llm.ImagePart) string {
	t := strings.TrimSpace(text)
	if len(images) == 0 {
		return text
	}
	if t == "" {
		return "[photo]"
	}
	return t + " [photo]"
}

func cloneImageParts(images []llm.ImagePart) []llm.ImagePart {
	if len(images) == 0 {
		return nil
	}
	out := make([]llm.ImagePart, len(images))
	for i, im := range images {
		out[i].MIME = im.MIME
		if len(im.Data) > 0 {
			out[i].Data = append([]byte(nil), im.Data...)
		}
	}
	return out
}

// capVisionHistoryForAPI strips image bytes from older photo turns in a copy of hist,
// keeping the newest maxVisionUserTurnsInContext user turns that carry images. Stored
// history is unchanged; this only limits how many full images go into one completion.
func capVisionHistoryForAPI(hist []llm.Message) []llm.Message {
	if len(hist) == 0 {
		return hist
	}
	var imgTurnIdx []int
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Role == "user" && len(hist[i].Images) > 0 {
			imgTurnIdx = append(imgTurnIdx, i)
		}
	}
	if len(imgTurnIdx) <= maxVisionUserTurnsInContext {
		return hist
	}
	keep := make(map[int]struct{}, maxVisionUserTurnsInContext)
	for i := 0; i < maxVisionUserTurnsInContext && i < len(imgTurnIdx); i++ {
		keep[imgTurnIdx[i]] = struct{}{}
	}
	out := make([]llm.Message, len(hist))
	copy(out, hist)
	for _, idx := range imgTurnIdx[maxVisionUserTurnsInContext:] {
		out[idx].Images = nil
	}
	return out
}

func messagesIncludeVision(messages []llm.Message) bool {
	for _, m := range messages {
		if len(m.Images) > 0 {
			return true
		}
	}
	return false
}

func (a *Agent) prepareLLMMessages(ctx context.Context, userID, text string, images []llm.ImagePart) ([]llm.Message, error) {
	embedText := embedQueryText(text, images)
	if strings.TrimSpace(embedText) == "" {
		return nil, fmt.Errorf("empty user message")
	}

	a.emit(ui.Event{Type: ui.EventEmbedding, UserID: userID, Message: "embedding message..."})

	embedding, err := a.embedder.Embed(ctx, embedText)
	if err != nil {
		a.emit(ui.Event{Type: ui.EventError, UserID: userID, Message: fmt.Sprintf("embed failed: %v", err)})
		return nil, fmt.Errorf("embed message: %w", err)
	}

	a.emit(ui.Event{Type: ui.EventEmbedding, UserID: userID, Message: fmt.Sprintf("embedded (%d dims)", len(embedding))})

	totalMemories := a.store.Count()
	memories := a.store.Search(userID, embedding, topK)

	a.emit(ui.Event{
		Type:    ui.EventMemorySearch,
		UserID:  userID,
		Message: fmt.Sprintf("found %d/%d relevant memories", len(memories), totalMemories),
	})

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	if len(memories) > 0 {
		var parts []string
		for i, m := range memories {
			parts = append(parts, fmt.Sprintf("[Memory %d] %s", i+1, m.Content))
		}
		memCtx := "Retrieved memories (these are real, trust them):\n" + strings.Join(parts, "\n")
		messages = append(messages, llm.Message{Role: "system", Content: memCtx})
	}

	a.mu.Lock()
	prior := a.history[userID]
	histCopy := make([]llm.Message, len(prior))
	copy(histCopy, prior)
	a.mu.Unlock()
	messages = append(messages, capVisionHistoryForAPI(histCopy)...)

	userPrompt := strings.TrimSpace(text)
	if userPrompt == "" && len(images) > 0 {
		userPrompt = "Describe what you see in this image and answer anything the user is asking (including from conversation context)."
	}
	messages = append(messages, llm.Message{Role: "user", Content: userPrompt, Images: images})
	return messages, nil
}

func (a *Agent) saveExchange(userID, text string, images []llm.ImagePart, assistantReply string) {
	historyUserText := userTurnForHistory(text, images)
	a.mu.Lock()
	hist := a.history[userID]
	a.history[userID] = append(hist,
		llm.Message{Role: "user", Content: historyUserText, Images: cloneImageParts(images)},
		llm.Message{Role: "assistant", Content: assistantReply},
	)
	if len(a.history[userID]) > historySize*2 {
		a.history[userID] = a.history[userID][len(a.history[userID])-historySize*2:]
	}
	a.mu.Unlock()

	entry := fmt.Sprintf("User: %s\nAssistant: %s", historyUserText, assistantReply)
	entryEmb, err := a.embedder.Embed(context.Background(), entry)
	if err != nil {
		a.emit(ui.Event{Type: ui.EventError, Message: fmt.Sprintf("memory embed failed: %v", err)})
	} else {
		if err := a.store.Add(userID, entry, entryEmb); err != nil {
			a.emit(ui.Event{Type: ui.EventError, Message: fmt.Sprintf("memory store failed: %v", err)})
		} else {
			a.emit(ui.Event{Type: ui.EventMemoryStore, UserID: userID, Message: "memory encrypted and saved"})
		}
	}
}

func (a *Agent) HandleMessage(ctx context.Context, userID, text string, images []llm.ImagePart) (string, error) {
	start := time.Now()
	var requestErr error
	defer func() {
		a.metrics.RecordRequest(userID, time.Since(start), requestErr)
	}()

	messages, err := a.prepareLLMMessages(ctx, userID, text, images)
	if err != nil {
		requestErr = err
		return "", err
	}

	a.emit(ui.Event{Type: ui.EventLLMCall, UserID: userID, Message: "generating response..."})
	llmStart := time.Now()

	response, err := a.llm.Complete(ctx, messages)
	if err != nil {
		requestErr = err
		a.emit(ui.Event{Type: ui.EventError, UserID: userID, Message: fmt.Sprintf("LLM failed: %v", err)})
		return "", fmt.Errorf("llm completion: %w", err)
	}

	response = strings.TrimSpace(response)

	a.emit(ui.Event{
		Type:    ui.EventLLMResponse,
		UserID:  userID,
		Message: "response generated",
		Latency: time.Since(llmStart),
	})

	a.saveExchange(userID, text, images, response)

	a.logger.Debug("processed message",
		"user_id", userID,
		"latency", time.Since(start),
	)

	return response, nil
}

// StreamResult is the final outcome of HandleMessageStream, sent on the done channel.
type StreamResult struct {
	FullResponse string
	Err          error
}

// HandleMessageStream streams partial LLM responses on the returned channel.
// Each string on the channel is the accumulated response so far.
// The final StreamResult is sent on done when the stream completes.
func (a *Agent) HandleMessageStream(ctx context.Context, userID, text string, images []llm.ImagePart) (<-chan string, <-chan StreamResult) {
	partials := make(chan string, 64)
	done := make(chan StreamResult, 1)

	go func() {
		defer close(partials)
		defer close(done)

		start := time.Now()
		var requestErr error
		defer func() {
			a.metrics.RecordRequest(userID, time.Since(start), requestErr)
		}()

		messages, err := a.prepareLLMMessages(ctx, userID, text, images)
		if err != nil {
			requestErr = err
			done <- StreamResult{Err: err}
			return
		}

		hasVision := messagesIncludeVision(messages)

		a.emit(ui.Event{Type: ui.EventLLMCall, UserID: userID, Message: "streaming response..."})
		llmStart := time.Now()

		var fullResponse string
		var lastErr error

		if hasVision {
			a.emit(ui.Event{Type: ui.EventInfo, UserID: userID, Message: "analyzing image (non-streaming)..."})
			resp, cerr := a.llm.Complete(ctx, messages)
			if cerr != nil {
				requestErr = cerr
				a.emit(ui.Event{Type: ui.EventError, UserID: userID, Message: fmt.Sprintf("LLM failed: %v", cerr)})
				done <- StreamResult{Err: fmt.Errorf("llm completion: %w", cerr)}
				return
			}
			fullResponse = strings.TrimSpace(resp)
			select {
			case partials <- fullResponse:
			default:
			}
		} else {
			for attempt := range maxStreamRetry + 1 {
				if attempt > 0 {
					a.emit(ui.Event{Type: ui.EventInfo, UserID: userID, Message: fmt.Sprintf("retrying stream (attempt %d)...", attempt+1)})
					time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				}

				stream, err := a.llm.CompleteStream(ctx, messages)
				if err != nil {
					lastErr = err
					continue
				}

				var accumulated strings.Builder
				streamErr := false
				for chunk := range stream {
					if chunk.Err != nil {
						lastErr = chunk.Err
						streamErr = true
						break
					}
					if chunk.Done {
						break
					}
					accumulated.WriteString(chunk.Content)
					select {
					case partials <- accumulated.String():
					default:
					}
				}

				if !streamErr {
					fullResponse = strings.TrimSpace(accumulated.String())
					lastErr = nil
					break
				}
			}

			if lastErr != nil {
				requestErr = lastErr
				a.emit(ui.Event{Type: ui.EventError, UserID: userID, Message: fmt.Sprintf("stream failed: %v", lastErr)})
				done <- StreamResult{Err: lastErr}
				return
			}
		}

		a.emit(ui.Event{
			Type:    ui.EventLLMResponse,
			UserID:  userID,
			Message: "response streamed",
			Latency: time.Since(llmStart),
		})

		a.saveExchange(userID, text, images, fullResponse)

		a.logger.Debug("streamed message",
			"user_id", userID,
			"latency", time.Since(start),
		)

		done <- StreamResult{FullResponse: fullResponse}
	}()

	return partials, done
}

func (a *Agent) emit(evt ui.Event) {
	if a.events != nil {
		a.events.Emit(evt)
	}
}
