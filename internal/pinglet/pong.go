package pinglet

import (
	"net/http"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/sim"
)

type PongRequest struct {
	CallerID string `json:"caller_id"`
}

type PongResponse struct {
	Message    string `json:"message"`
	ReceiverID string `json:"receiver_id"`
}

func Pong(cfg *config.Config, machine *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[PongRequest](w, r)
		if err != nil {
			return
		}

		data := sim.Data{
			Input: sim.Input{
				DelayMs:     cfg.PongDelayMs,
				FailureRate: cfg.PongFailureRate,
				IsAvailable: cfg.IsAvailable,
			},
		}

		if _, err := machine.Run(ctx, &data); err != nil {
			log.Error("simulation failed",
				"message", err.Error(),
			)

			if ctx.Err() != nil {
				return
			}

			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, err.Error())
			return
		}

		if data.Outcome != sim.Proceed {
			log.Error("failure injected on pong",
				"outcome", data.Outcome,
				"reason", data.Reason,
			)

			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, data.Reason)
			return
		}

		log.Info("received pong request",
			"caller_id", req.CallerID,
		)

		httpx.WriteOKResponse(w, PongResponse{
			Message:    "pong",
			ReceiverID: cfg.ID,
		})
	}
}
