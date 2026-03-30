package llm

import "context"

// ImagePart is raw image bytes for vision models (OpenAI-style image_url with data URI).
type ImagePart struct {
	MIME string // e.g. image/jpeg; empty defaults to image/jpeg in the OpenAI provider
	Data []byte
}

type Message struct {
	Role    string
	Content string
	// Images is only used for role "user" in the OpenAI-compatible path; leave nil for text-only.
	Images []ImagePart
}

type StreamChunk struct {
	Content string
	Err     error
	Done    bool
}

type Provider interface {
	Complete(ctx context.Context, messages []Message) (string, error)
	CompleteStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}
