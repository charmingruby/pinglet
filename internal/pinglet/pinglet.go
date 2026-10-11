package pinglet

import (
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/chain"
	"github.com/charmingruby/pinglet/internal/pinglet/pingpong"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/go-chi/chi/v5"
)

func New(r chi.Router, cfg *config.Config) {
	networkSim := sim.New()
	if err := networkSim.Validate(); err != nil {
		panic(err)
	}

	r.Post("/ping", pingpong.Ping(cfg, networkSim))
	r.Post("/pong", pingpong.Pong(cfg, networkSim))
	r.Post("/chain", chain.Chain(cfg, networkSim))
	r.Post("/relay", chain.Relay(cfg, networkSim))
}
