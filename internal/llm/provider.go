package llm

import "context"

type Message struct {
	Role    string
	Content string
}

type Provider interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}
