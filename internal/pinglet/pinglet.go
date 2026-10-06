package pinglet

import (
	"github.com/charmingruby/pinglet/config"
	"github.com/go-chi/chi/v5"
)

func New(r chi.Router, cfg *config.Config) {
	PingRoute(r, cfg)
	PongRoute(r, cfg)
}
