package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/kanywst/approval-hub/internal/config"
	"github.com/kanywst/approval-hub/internal/server"
	"github.com/kanywst/approval-hub/internal/store"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the broker daemon in the foreground.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context())
		},
	}
}

func runServe(ctx context.Context) error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	cfg, err := config.LoadOrInit(cfgPath)
	if err != nil {
		return err
	}
	stPath, err := storePath()
	if err != nil {
		return err
	}
	st, err := store.Open(stPath)
	if err != nil {
		return err
	}
	pidp, err := pidPath()
	if err != nil {
		return err
	}
	release, err := acquirePID(pidp)
	if err != nil {
		return err
	}
	defer release()

	srv := server.NewServer(cfg, cfgPath, st)
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "approval-hub: listening on %s (config: %s)\n",
		addr, cfgPath)
	if err := httpSrv.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// acquirePID writes the current PID to path, refusing if a live PID is already
// recorded there. Stale PID files (the process no longer exists) are taken
// over silently.
func acquirePID(path string) (func(), error) {
	if data, err := os.ReadFile(path); err == nil {
		if pid, perr := strconv.Atoi(string(data)); perr == nil && pid > 0 {
			if err := syscall.Kill(pid, 0); err == nil {
				return nil, fmt.Errorf("broker already running (pid %d)", pid)
			}
		}
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return nil, fmt.Errorf("write pid file: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}
