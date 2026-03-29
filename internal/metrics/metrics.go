package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Collector struct {
	messagesTotal atomic.Int64
	errorsTotal   atomic.Int64
	latencySum    atomic.Int64
	latencyCount  atomic.Int64
	startTime     time.Time

	mu        sync.Mutex
	eventFile *os.File
	users     map[string]time.Time // user_id -> last seen (no content stored)
}

type Event struct {
	Timestamp string `json:"ts"`
	Type      string `json:"type"`
	UserID    string `json:"user_id,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

func NewCollector(dataDir string) (*Collector, error) {
	c := &Collector{
		startTime: time.Now(),
		users:     make(map[string]time.Time),
	}

	if dataDir != "" {
		path := filepath.Join(dataDir, "events.jsonl")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("open events log: %w", err)
		}
		c.eventFile = f
	}

	c.emit(Event{Type: "daemon_start"})
	return c, nil
}

func (c *Collector) IncrementMessages() { c.messagesTotal.Add(1) }
func (c *Collector) IncrementErrors()   { c.errorsTotal.Add(1) }

func (c *Collector) RecordLatency(d time.Duration) {
	c.latencySum.Add(d.Milliseconds())
	c.latencyCount.Add(1)
}

func (c *Collector) RecordRequest(userID string, latency time.Duration, err error) {
	c.messagesTotal.Add(1)
	c.RecordLatency(latency)

	c.mu.Lock()
	c.users[userID] = time.Now()
	c.mu.Unlock()

	evt := Event{
		Type:      "message",
		UserID:    userID,
		LatencyMs: latency.Milliseconds(),
	}
	if err != nil {
		c.errorsTotal.Add(1)
		evt.Type = "message_error"
		evt.Error = err.Error()
	}
	c.emit(evt)
}

func (c *Collector) UniqueUsers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.users)
}

func (c *Collector) Summary() string {
	msgs := c.messagesTotal.Load()
	errs := c.errorsTotal.Load()
	count := c.latencyCount.Load()
	uptime := time.Since(c.startTime).Round(time.Second)
	users := c.UniqueUsers()
	var avgMs int64
	if count > 0 {
		avgMs = c.latencySum.Load() / count
	}
	return fmt.Sprintf("uptime=%s messages=%d errors=%d users=%d avg_latency=%dms",
		uptime, msgs, errs, users, avgMs)
}

func (c *Collector) Close() {
	c.emit(Event{Type: "daemon_stop"})
	if c.eventFile != nil {
		_ = c.eventFile.Close()
	}
}

func (c *Collector) emit(evt Event) {
	evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	if c.eventFile == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, _ := json.Marshal(evt)
	data = append(data, '\n')
	_, _ = c.eventFile.Write(data)
}
