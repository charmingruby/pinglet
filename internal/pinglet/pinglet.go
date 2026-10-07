package pinglet

import (
	"context"
	"time"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/sim"
	"github.com/go-chi/chi/v5"
)

func New(r chi.Router, cfg *config.Config) error {
	sim := sim.New()

	if err := sim.Validate(); err != nil {
		return err
	}

	r.Post("/ping", Ping(cfg, sim))
	r.Post("/pong", Pong(cfg, sim))

	return nil
}

func applyDelay(ctx context.Context, delayMs int) bool {
	if delayMs <= 0 {
		return true
	}

	timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
