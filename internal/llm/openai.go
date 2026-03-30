package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const ollamaBaseURL = "http://localhost:11434/v1"

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func NewOllamaProvider(model string) *OpenAIProvider {
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = ollamaBaseURL
	return &OpenAIProvider{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

func toOpenAIMessage(m Message) openai.ChatCompletionMessage {
	if len(m.Images) == 0 {
		return openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	userText := strings.TrimSpace(m.Content)
	if userText == "" {
		userText = "Describe this image and answer any question in context."
	}
	parts := []openai.ChatMessagePart{
		{Type: openai.ChatMessagePartTypeText, Text: userText},
	}
	for _, img := range m.Images {
		mime := img.MIME
		if mime == "" {
			mime = "image/jpeg"
		}
		b64 := base64.StdEncoding.EncodeToString(img.Data)
		u := fmt.Sprintf("data:%s;base64,%s", mime, b64)
		parts = append(parts, openai.ChatMessagePart{
			Type:     openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{URL: u},
		})
	}
	return openai.ChatCompletionMessage{
		Role:         m.Role,
		MultiContent: parts,
	}
}

func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		out[i] = toOpenAIMessage(m)
	}
	return out
}

func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	msgs := toOpenAIMessages(messages)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: msgs,
	})
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) CompleteStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	msgs := toOpenAIMessages(messages)

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: msgs,
	})
	if err != nil {
		return nil, fmt.Errorf("create stream: %w", err)
	}

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		defer stream.Close()
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				ch <- StreamChunk{Done: true}
				return
			}
			if err != nil {
				ch <- StreamChunk{Err: fmt.Errorf("stream recv: %w", err)}
				return
			}
			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta.Content
				if delta != "" {
					ch <- StreamChunk{Content: delta}
				}
			}
		}
	}()

	return ch, nil
}
