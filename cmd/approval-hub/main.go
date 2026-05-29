package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "approval-hub",
		Short:         "Aggregate Claude Code permission prompts into one TUI.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(
		newServeCmd(),
		newAttachCmd(),
		newListCmd(),
		newRevokeCmd(),
		newRotateCmd(),
		newDoctorCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
