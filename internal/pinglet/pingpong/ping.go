package pingpong

import (
	"context"
	"net/http"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
)

type PingRequest struct {
	To string `json:"to" validate:"required,url"`
}

type PingResponse struct {
	Message    string `json:"message"`
	ReceiverID string `json:"receiver_id"`
	CallerID   string `json:"caller_id"`
}

func Ping(cfg *config.Config, networkSim *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[PingRequest](w, r)
		if err != nil {
			return
		}

		reason, aborted := sim.Check(ctx, networkSim, cfg.PingDelay, sim.Input{
			FailureRate: cfg.PingFailureRate,
			IsAvailable: cfg.IsAvailable,
		})
		if aborted {
			return
		}

		if reason != "" {
			log.Error("failure injected on ping", "reason", reason)
			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, reason)

			return
		}

		log.Info("trying to call pong", "caller_id", cfg.ID)

		pongCtx, cancel := context.WithTimeout(ctx, cfg.HopTimeout+cfg.PongDelay)
		defer cancel()

		pong, err := httpx.Post[PongResponse](pongCtx, req.To, PongRequest{CallerID: cfg.ID})
		if err != nil {
			log.Error("error from pong", "message", err.Error(), "to", req.To)
			httpx.WriteError(w, http.StatusServiceUnavailable, "Pong service is unavailable")

			return
		}

		log.Info("called pong successfully",
			"caller_id", cfg.ID,
			"receiver_id", pong.ReceiverID,
		)

		httpx.WriteOKResponse(w, PingResponse{
			Message:    pong.Message,
			ReceiverID: pong.ReceiverID,
			CallerID:   cfg.ID,
		})
	}
}
