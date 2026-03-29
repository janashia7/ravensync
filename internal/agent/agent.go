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
	topK        = 5
	historySize = 10 // max recent messages to keep per user

	systemPrompt = `You are Ravensync, a personal AI assistant with persistent memory. You remember things about the user across conversations and platforms (console, Telegram, etc).

MEMORY RULES:
- You will be given "Retrieved memories" below. These are REAL things you stored previously. TRUST THEM COMPLETELY.
- When the user asks a question and a retrieved memory contains the answer, respond CONFIDENTLY. Example: if a memory says "user's name is Giorgi" and they ask "what's my name?", answer "Your name is Giorgi!" — do NOT hedge or say "I think".
- NEVER say "I don't remember" or "you didn't tell me" if a retrieved memory has the answer.
- If NO retrieved memory matches the question, be honest: say you don't have that info yet and invite them to share it.

CONVERSATION:
- You will receive recent conversation history. ALWAYS use it to understand context.
- Short messages like "why?", "what do you think?", "really?" ALWAYS refer to the previous topic. Never ignore context and give a generic response.
- Stay on topic until the user clearly changes the subject.

GENERAL:
- Be concise, warm, and helpful.
- Use the user's name naturally if you know it from memories.
- Do NOT say "I'll remember that" or mention memory unless the user EXPLICITLY asks you to remember something. Just respond naturally.`
)

type Agent struct {
	store    *memory.Store
	embedder memory.Embedder
	llm      llm.Provider
	metrics  *metrics.Collector
	logger   *slog.Logger
	events   *ui.EventBus

	mu      sync.Mutex
	history map[string][]llm.Message // recent conversation per user
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

func (a *Agent) HandleMessage(ctx context.Context, userID, text string) (string, error) {
	start := time.Now()
	var requestErr error
	defer func() {
		a.metrics.RecordRequest(userID, time.Since(start), requestErr)
	}()

	a.emit(ui.Event{Type: ui.EventEmbedding, UserID: userID, Message: "embedding message..."})

	embedding, err := a.embedder.Embed(ctx, text)
	if err != nil {
		requestErr = err
		a.emit(ui.Event{Type: ui.EventError, UserID: userID, Message: fmt.Sprintf("embed failed: %v", err)})
		return "", fmt.Errorf("embed message: %w", err)
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
	messages = append(messages, a.history[userID]...)
	a.mu.Unlock()

	messages = append(messages, llm.Message{Role: "user", Content: text})

	a.emit(ui.Event{Type: ui.EventLLMCall, UserID: userID, Message: "generating response..."})
	llmStart := time.Now()

	response, err := a.llm.Complete(ctx, messages)
	if err != nil {
		requestErr = err
		a.emit(ui.Event{Type: ui.EventError, UserID: userID, Message: fmt.Sprintf("LLM failed: %v", err)})
		return "", fmt.Errorf("llm completion: %w", err)
	}

	response = strings.TrimSpace(response)

	a.mu.Lock()
	a.history[userID] = append(a.history[userID],
		llm.Message{Role: "user", Content: text},
		llm.Message{Role: "assistant", Content: response},
	)
	if len(a.history[userID]) > historySize*2 {
		a.history[userID] = a.history[userID][len(a.history[userID])-historySize*2:]
	}
	a.mu.Unlock()

	a.emit(ui.Event{
		Type:    ui.EventLLMResponse,
		UserID:  userID,
		Message: "response generated",
		Latency: time.Since(llmStart),
	})

	entry := fmt.Sprintf("User: %s\nAssistant: %s", text, response)
	entryEmb, err := a.embedder.Embed(ctx, entry)
	if err != nil {
		a.emit(ui.Event{Type: ui.EventError, Message: fmt.Sprintf("memory embed failed: %v", err)})
	} else {
		if err := a.store.Add(userID, entry, entryEmb); err != nil {
			a.emit(ui.Event{Type: ui.EventError, Message: fmt.Sprintf("memory store failed: %v", err)})
		} else {
			a.emit(ui.Event{Type: ui.EventMemoryStore, UserID: userID, Message: "memory encrypted and saved"})
		}
	}

	a.logger.Debug("processed message",
		"user_id", userID,
		"memories_used", len(memories),
		"latency", time.Since(start),
	)

	return response, nil
}

func (a *Agent) emit(evt ui.Event) {
	if a.events != nil {
		a.events.Emit(evt)
	}
}
