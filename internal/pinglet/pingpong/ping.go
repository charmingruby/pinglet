package pingpong

import (
	"context"
	"net/http"
	"net/url"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
)

// PingRequest calls a single downstream pong.
// {"to": "http://pong-node:4000/api/pong"}.
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

		if !sim.ApplyDelay(ctx, cfg.PingDelay) {
			return
		}

		simData := sim.Data{
			Input: sim.Input{
				FailureRate: cfg.PingFailureRate,
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
			log.Error("failure injected on ping", "reason", simReason)
			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, simReason)

			return
		}

		if u, err := url.ParseRequestURI(req.To); err != nil || u.Scheme == "" || u.Host == "" {
			httpx.WriteResponse(w, http.StatusBadRequest, map[string]string{
				"message": "invalid destination URL",
			})

			return
		}

		log.Info("trying to call pong", "caller_id", cfg.ID)

		pongCtx, cancel := context.WithTimeout(ctx, cfg.HopTimeout+cfg.PongDelay)
		defer cancel()

		pong, err := httpx.Do[PongResponse](pongCtx, httpx.NewClient(), httpx.Request{
			Method:         http.MethodPost,
			URL:            req.To,
			ExpectedStatus: http.StatusOK,
			Body: PongRequest{
				CallerID: cfg.ID,
			},
		})
		if err != nil {
			log.Error("error from pong",
				"message", err.Error(),
				"to", req.To,
			)

			httpx.WriteResponse(w, http.StatusServiceUnavailable, map[string]string{
				"message": "Pong service is unavailable",
			})

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
