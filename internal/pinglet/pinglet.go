package pinglet

import (
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
