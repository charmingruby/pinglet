package pinglet

import (
	"context"
	"net/http"
	"time"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/sim"
)

type PingRequest struct {
	URL  string `json:"url"`
	Path string `json:"path"`
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

		if !applyDelay(ctx, cfg.PingDelayMs) {
			return
		}

		data := sim.Data{
			Input: sim.Input{
				FailureRate: cfg.PingFailureRate,
				IsAvailable: cfg.IsAvailable,
			},
		}

		if _, err := networkSim.Run(ctx, &data); err != nil {
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
			log.Error("failure injected on ping",
				"outcome", data.Outcome,
				"reason", data.Reason,
			)

			httpx.WriteFailureInjection(w, cfg.InjectStatusCode, data.Reason)
			return
		}

		cl := NewClient(req.URL)

		log.Info("trying to call pong",
			"caller_id", cfg.ID,
		)

		defaultPongTimeout := 5 * time.Second
		pongTimeout := defaultPongTimeout + time.Duration(cfg.PongDelayMs)*time.Millisecond

		pongCtx, cancel := context.WithTimeout(ctx, pongTimeout)
		defer cancel()

		pong, err := cl.Pong(pongCtx, req.Path, cfg.ID)
		if err != nil {
			log.Error("error from pong",
				"message", err.Error(),
				"url", req.URL,
				"path", req.Path,
			)

			httpx.WriteResponse(w, http.StatusInternalServerError, map[string]string{
				"message": "Internal Server Error",
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
