package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SSEConsumer reads broker SSE events with automatic reconnect.
type SSEConsumer struct {
	url   string
	token string
}

// NewSSEConsumer creates a consumer that connects to url with the given token.
func NewSSEConsumer(url, token string) *SSEConsumer {
	return &SSEConsumer{url: url, token: token}
}

// SSEEvent carries the type tag and raw JSON payload.
type SSEEvent struct {
	Type string
	Data json.RawMessage
}

// Consume runs until ctx is canceled. On disconnect it sleeps backoff and
// retries with exponential growth capped at 30s. Errors are not surfaced to
// the caller; this is best-effort streaming.
func (c *SSEConsumer) Consume(ctx context.Context, out chan<- SSEEvent) {
	backoff := time.Second
	for {
		err := c.runOnce(ctx, out)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
	}
}

func (c *SSEConsumer) runOnce(ctx context.Context, out chan<- SSEEvent) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sse status %d", resp.StatusCode)
	}
	r := bufio.NewReader(resp.Body)
	var ev SSEEvent
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if ev.Type != "" || len(ev.Data) > 0 {
				select {
				case out <- ev:
				case <-ctx.Done():
					return ctx.Err()
				}
				ev = SSEEvent{}
			}
		case strings.HasPrefix(line, "event:"):
			ev.Type = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			ev.Data = json.RawMessage(strings.TrimSpace(line[len("data:"):]))
		}
	}
}
