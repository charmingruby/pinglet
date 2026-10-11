package pingpong

import (
	"net/http"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
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

		if !sim.ApplyDelay(ctx, cfg.PongDelay) {
			return
		}

		simData := sim.Data{
			Input: sim.Input{
				FailureRate: cfg.PongFailureRate,
				IsAvailable: cfg.IsAvailable,
			},
		}

		simReason := ""
		if _, err := networkSim.Run(ctx, &simData); err != nil {
			if ctx.Err() != nil {
				return
			}

			simReason = err.Error()
		} else if simData.Outcome != sim.Proceed {
			simReason = simData.Reason
		}

		if simReason != "" {
			log.Error("failure injected on pong", "reason", simReason)
			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, simReason)

			return
		}

		log.Info("received pong request", "caller_id", req.CallerID)

		httpx.WriteOKResponse(w, PongResponse{
			Message:    "pong",
			ReceiverID: cfg.ID,
		})
	}
}
