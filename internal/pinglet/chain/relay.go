package chain

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
)

func Relay(cfg *config.Config, networkSim *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[RelayRequest](w, r)
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

		trace := append(req.Trace, HopResult{ID: cfg.ID, Message: "ok"})

		if reason != "" {
			log.Error("failure injected on relay", "reason", reason)
			httpx.WriteResponse(w, cfg.InjectStatusCode, ChainErrorBody{
				Reason:   reason,
				FailedAt: cfg.ID,
				Trace:    trace,
			})

			return
		}

		if len(req.Chain) > cfg.MaxChainDepth {
			msg := fmt.Sprintf("chain depth %d exceeds max %d", len(req.Chain), cfg.MaxChainDepth)
			log.Error("invalid remaining chain on relay", "message", msg)
			httpx.WriteError(w, http.StatusBadRequest, msg)

			return
		}

		seen := make(map[string]struct{}, len(req.Chain))
		for _, entry := range req.Chain {
			u, uerr := url.ParseRequestURI(entry)
			if uerr != nil || u.Scheme == "" || u.Host == "" {
				msg := fmt.Sprintf("invalid chain entry %q: must be an absolute URL", entry)
				log.Error("invalid remaining chain on relay", "message", msg)
				httpx.WriteError(w, http.StatusBadRequest, msg)

				return
			}

			if _, dup := seen[entry]; dup {
				msg := fmt.Sprintf("invalid chain entry %q: duplicated URL (cycle)", entry)
				log.Error("invalid remaining chain on relay", "message", msg)
				httpx.WriteError(w, http.StatusBadRequest, msg)

				return
			}

			seen[entry] = struct{}{}
		}

		if len(req.Chain) == 0 {
			log.Info("relay terminal reached", "caller_id", req.CallerID)
			httpx.WriteOKResponse(w, ChainResponse{
				CallerID:   req.CallerID,
				ReceiverID: cfg.ID,
				Trace:      trace,
			})

			return
		}

		log.Info("forwarding chain",
			"caller_id", req.CallerID,
			"node_id", cfg.ID,
			"next", req.Chain[0],
			"remaining", len(req.Chain)-1,
		)

		fwdCtx, cancel := context.WithTimeout(ctx, cfg.HopTimeout)
		defer cancel()

		res, err := httpx.Post[ChainResponse](fwdCtx, req.Chain[0], RelayRequest{
			CallerID: cfg.ID,
			Chain:    req.Chain[1:],
			Trace:    trace,
		})
		if err != nil {
			log.Error("downstream call failed",
				"message", err.Error(),
				"node_id", cfg.ID,
				"next", req.Chain[0],
			)

			var statusErr *httpx.StatusError
			if errors.As(err, &statusErr) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusErr.Status)
				_, _ = w.Write(statusErr.Body)

				return
			}

			httpx.WriteResponse(w, http.StatusBadGateway, ChainErrorBody{
				Reason:   err.Error(),
				FailedAt: cfg.ID,
				Trace:    trace,
			})

			return
		}

		httpx.WriteOKResponse(w, res)
	}
}
