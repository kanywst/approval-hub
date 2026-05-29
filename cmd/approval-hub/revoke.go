package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <rule-id>",
		Short: "Delete a learned rule.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRevoke(args[0])
		},
	}
}

func runRevoke(id string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cli := newClient(cfg)
	resp, err := cli.request(http.MethodDelete, "/rules/"+id, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNoContent {
		fmt.Printf("revoked %s\n", id)
		return nil
	}
	return fmt.Errorf("broker returned %d", resp.StatusCode)
}
