package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/platform/validator"
)

const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	log := logging.InitLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config: error loading config", "error", err)
		return err
	}

	srv, router := httpx.NewServer(
		cfg.Port,
		validator.New(),
		cfg.IsAvailable,
		cfg.InjectStatusCode,
	)
	pinglet.New(router, cfg)

	if !cfg.IsAvailable {
		log.Info("server: manual failure injection detected")
	}

	log.Info("server: running...", "port", cfg.Port)

	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		log.Info("shutdown: signal received, starting graceful shutdown...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		errCh <- srv.Close(shutdownCtx)
	}()

	if err := srv.Start(); err != nil {
		log.Error("server: start error", "error", err)
		stop()
		return err
	}

	if err := <-errCh; err != nil {
		log.Error("shutdown: shutdown error", "error", err)
		return err
	}

	log.Info("shutdown: gracefully shutdown")
	return nil
}
