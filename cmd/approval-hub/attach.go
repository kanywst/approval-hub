package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/kanywst/approval-hub/internal/tui"
)

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach",
		Short: "Attach the TUI client to the broker.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAttach(cmd.Context())
		},
	}
}

func runAttach(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cli := newClient(cfg)

	sender := func(a tui.Action) tea.Cmd {
		return func() tea.Msg {
			if a.Kind != "resolve" {
				return nil
			}
			payload, err := json.Marshal(map[string]any{
				"decision":    a.Decision,
				"scope":       a.Scope,
				"ttl_seconds": a.TTL,
				"matcher":     a.Matcher,
			})
			if err != nil {
				return tui.ErrMsg{Err: err}
			}
			resp, err := cli.request(http.MethodPost,
				"/pending/"+a.ID+"/resolve",
				bytes.NewReader(payload))
			if err != nil {
				return tui.ErrMsg{Err: err}
			}
			_ = resp.Body.Close()
			return tui.RemovePendingMsg{ID: a.ID}
		}
	}

	model := tui.New(sender)
	prog := tea.NewProgram(model)

	sseCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go fetchInitialPending(cli, prog)
	go streamEvents(sseCtx, cfg.Port, cfg.Token, prog)

	_, err = prog.Run()
	return err
}

func fetchInitialPending(cli *brokerClient, prog *tea.Program) {
	resp, err := cli.request(http.MethodGet, "/pending", nil)
	if err != nil {
		prog.Send(tui.ErrMsg{Err: err})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	var ps []tui.Pending
	if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
		prog.Send(tui.ErrMsg{Err: err})
		return
	}
	prog.Send(tui.InitialPendingMsg(ps))
}

func streamEvents(ctx context.Context, port int, token string, prog *tea.Program) {
	url := fmt.Sprintf("http://127.0.0.1:%d/events", port)
	consumer := tui.NewSSEConsumer(url, token)
	events := make(chan tui.SSEEvent, 16)
	go consumer.Consume(ctx, events)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			dispatchSSE(prog, ev)
		}
	}
}

func dispatchSSE(prog *tea.Program, ev tui.SSEEvent) {
	switch ev.Type {
	case "pending_added":
		var p tui.Pending
		if err := json.Unmarshal(ev.Data, &p); err == nil {
			prog.Send(tui.AddPendingMsg(p))
		}
	case "pending_resolved", "pending_expired":
		var d struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			prog.Send(tui.RemovePendingMsg{ID: d.ID})
		}
	}
}
