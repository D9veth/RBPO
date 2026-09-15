package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/D9veth/RBPO/internal/app"
	"github.com/D9veth/RBPO/internal/config"
	"github.com/D9veth/RBPO/internal/database"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://127.0.0.1:8080/readyz")
		if err != nil {
			os.Exit(1)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	setup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pool, err := database.Open(setup, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	defer pool.Close()
	if err = database.Migrate(setup, pool); err != nil {
		return err
	}
	api := app.New(pool, cfg, logger)
	server := &http.Server{Addr: cfg.Address, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	failed := make(chan error, 1)
	go func() { logger.Info("server ready", "address", cfg.Address); failed <- server.ListenAndServe() }()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup, cancel := context.WithTimeout(ctx, 10*time.Second)
				_, err := pool.Exec(cleanup, `DELETE FROM sessions WHERE expires_at<now(); DELETE FROM rate_limits WHERE window_start<now()-interval '1 day'`)
				cancel()
				if err != nil {
					logger.Warn("cleanup failed")
				}
			}
		}
	}()
	select {
	case err := <-failed:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}
