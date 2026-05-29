package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the approval-hub install.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDoctor()
		},
	}
}

func runDoctor() error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	fmt.Printf("config path: %s\n", cfgPath)
	if _, err := os.Stat(cfgPath); err != nil {
		fmt.Println("  -> not found; run any command to bootstrap")
		return nil
	}
	fmt.Println("  -> ok")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cli := newClient(cfg)
	resp, err := cli.request(http.MethodGet, "/rules", nil)
	if err != nil {
		fmt.Printf("broker: not reachable on port %d\n", cfg.Port)
		fmt.Printf("  -> %v\n", err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	fmt.Printf("broker: reachable on port %d (status %d)\n",
		cfg.Port, resp.StatusCode)
	return nil
}
