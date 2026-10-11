package pingpong

import (
	"net/http"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
)

type PongRequest struct {
	CallerID string `json:"caller_id" validate:"required"`
}

type PongResponse struct {
	Message    string `json:"message"`
	ReceiverID string `json:"receiver_id"`
}

func Pong(cfg *config.Config, networkSim *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[PongRequest](w, r)
		if err != nil {
			return
		}

		reason, aborted := sim.Check(ctx, networkSim, cfg.PongDelay, sim.Input{
			FailureRate: cfg.PongFailureRate,
			IsAvailable: cfg.IsAvailable,
		})
		if aborted {
			return
		}

		if reason != "" {
			log.Error("failure injected on pong", "reason", reason)
			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, reason)

			return
		}

		log.Info("received pong request", "caller_id", req.CallerID)

		httpx.WriteOKResponse(w, PongResponse{
			Message:    "pong",
			ReceiverID: cfg.ID,
		})
	}
}
