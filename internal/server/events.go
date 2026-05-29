package server

import "sync"

// EventType is the kind of broker event distributed via SSE.
type EventType string

const (
	EventPendingAdded    EventType = "pending_added"
	EventPendingResolved EventType = "pending_resolved"
	EventPendingExpired  EventType = "pending_expired"
	EventRuleAdded       EventType = "rule_added"
	EventRuleRevoked     EventType = "rule_revoked"
)

// Event is one broker event.
type Event struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

// EventBus fans out broker events to registered subscribers. Slow subscribers
// drop events when their buffer is full so a stuck TUI cannot stall the broker.
type EventBus struct {
	mu          sync.Mutex
	subscribers map[int]chan Event
	next        int
	buf         int
}

func newEventBus(bufPerSub int) *EventBus {
	return &EventBus{
		subscribers: make(map[int]chan Event),
		buf:         bufPerSub,
	}
}

// Subscribe registers a new subscriber. Returns its channel and an unsubscribe
// func. The returned channel is closed by the unsubscribe func.
func (b *EventBus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	id := b.next
	b.next++
	ch := make(chan Event, b.buf)
	b.subscribers[id] = ch
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if c, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(c)
		}
	}
	return ch, unsub
}

// Publish sends ev to all subscribers; non-blocking, drops on full buffers.
func (b *EventBus) Publish(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Subscribers returns the current subscriber count (for tests/diagnostics).
func (b *EventBus) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subscribers)
}
