package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newRotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate",
		Short: "Rotate the broker token.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runRotate()
		},
	}
}

func runRotate() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cli := newClient(cfg)
	resp, err := cli.request(http.MethodPost, "/rotate-token", nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("broker returned %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	fmt.Printf("new token: %s\n", body["token"])
	fmt.Println("update plugin hooks.json with the new token.")
	return nil
}
