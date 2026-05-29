package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/kanywst/approval-hub/internal/store"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List learned matcher rules.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runList()
		},
	}
}

func runList() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cli := newClient(cfg)
	resp, err := cli.request(http.MethodGet, "/rules", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("broker returned %d", resp.StatusCode)
	}
	var rules []store.Rule
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		return err
	}
	if len(rules) == 0 {
		fmt.Println("(no rules)")
		return nil
	}
	for _, r := range rules {
		ttl := "forever"
		if r.ExpiresAt != nil {
			ttl = r.ExpiresAt.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("%-20s %s\t%s\t(%s)\n", r.ID, r.Decision, r.Rule, ttl)
	}
	return nil
}
