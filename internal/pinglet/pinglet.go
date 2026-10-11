package pinglet

import (
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/chain"
	"github.com/charmingruby/pinglet/internal/pinglet/pingpong"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/go-chi/chi/v5"
)

func New(r chi.Router, cfg *config.Config) error {
	sim := sim.New()

	if err := sim.Validate(); err != nil {
		return err
	}

	r.Post("/ping", pingpong.Ping(cfg, sim))
	r.Post("/pong", pingpong.Pong(cfg, sim))
	r.Post("/chain", chain.Chain(cfg, sim))
	r.Post("/relay", chain.Relay(cfg, sim))

	return nil
}
