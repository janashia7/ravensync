package llm

import (
	"context"
	"fmt"

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

func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	msgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		msgs[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

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
