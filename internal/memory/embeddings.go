package memory

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

const ollamaBaseURL = "http://localhost:11434/v1"

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

type OpenAIEmbedder struct {
	client *openai.Client
	model  openai.EmbeddingModel
}

func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client: openai.NewClient(apiKey),
		model:  openai.EmbeddingModel(model),
	}
}

func NewOllamaEmbedder(model string) *OpenAIEmbedder {
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = ollamaBaseURL
	return &OpenAIEmbedder{
		client: openai.NewClientWithConfig(cfg),
		model:  openai.EmbeddingModel(model),
	}
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: e.model,
	})
	if err != nil {
		return nil, fmt.Errorf("create embedding: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	f32 := resp.Data[0].Embedding
	f64 := make([]float64, len(f32))
	for i, v := range f32 {
		f64[i] = float64(v)
	}
	return f64, nil
}
