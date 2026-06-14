package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Event struct {
	Type      string      `json:"type"`
	Instance  string      `json:"instance"`
	IOA       uint32      `json:"ioa,omitempty"`
	Value     interface{} `json:"value,omitempty"`
	Timestamp int64       `json:"ts"`
}

type subscriber struct {
	ch chan Event
}

type Bus struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

func NewBus() *Bus {
	return &Bus{subs: make(map[*subscriber]struct{})}
}

func (b *Bus) Subscribe() (*subscriber, func()) {
	s := &subscriber{ch: make(chan Event, 256)}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s, func() {
		b.mu.Lock()
		delete(b.subs, s)
		b.mu.Unlock()
		close(s.ch)
	}
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subs {
		select {
		case s.ch <- e:
		default:
		}
	}
}

func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func (b *Bus) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"message\":\"SSE stream established\"}\n\n")
	flusher.Flush()

	sub, unsub := b.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-sub.ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
