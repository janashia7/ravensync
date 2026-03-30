package ui

import (
	"fmt"
	"sync"
	"time"
)

type EventType int

const (
	EventMessageIn EventType = iota
	EventEmbedding
	EventMemorySearch
	EventLLMCall
	EventLLMResponse
	EventMemoryStore
	EventMessageOut
	EventInfo
	EventError
)

func (t EventType) Label() string {
	switch t {
	case EventMessageIn:
		return "MSG IN"
	case EventEmbedding:
		return "EMBED"
	case EventMemorySearch:
		return "SEARCH"
	case EventLLMCall:
		return "LLM"
	case EventLLMResponse:
		return "LLM"
	case EventMemoryStore:
		return "STORE"
	case EventMessageOut:
		return "SEND"
	case EventInfo:
		return "INFO"
	case EventError:
		return "ERROR"
	default:
		return "???"
	}
}

type Event struct {
	Time       time.Time
	Type       EventType
	UserID     string // display name (e.g. "tg:alice")
	InternalID string // actual memory key (e.g. "tg:123456789")
	Message    string
	Latency    time.Duration
}

func (e Event) FormatLog() string {
	ts := e.Time.Format("15:04:05")
	label := e.Type.Label()
	if e.Latency > 0 {
		return fmt.Sprintf("[%s] %-6s %s (%s)", ts, label, e.Message, e.Latency.Round(time.Millisecond))
	}
	return fmt.Sprintf("[%s] %-6s %s", ts, label, e.Message)
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers []chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{}
}

func (b *EventBus) Subscribe() <-chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	return ch
}

func (b *EventBus) Emit(evt Event) {
	if evt.Time.IsZero() {
		evt.Time = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
		}
	}
}
