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

type ChainStartRequest struct {
	Chain []string `json:"chain" validate:"required,min=1,dive,url"`
}

type RelayRequest struct {
	CallerID string      `json:"caller_id"       validate:"required"`
	Chain    []string    `json:"chain,omitempty" validate:"dive,url"`
	Trace    []HopResult `json:"trace,omitempty"`
}

type HopResult struct {
	ID      string `json:"id"      validate:"required"`
	Message string `json:"message" validate:"required"`
}

type ChainResponse struct {
	CallerID   string      `json:"caller_id"`
	ReceiverID string      `json:"receiver_id"`
	Trace      []HopResult `json:"trace"`
}

type ChainErrorBody struct {
	Reason   string      `json:"reason"`
	FailedAt string      `json:"failed_at"`
	Trace    []HopResult `json:"trace"`
}

func Chain(cfg *config.Config, networkSim *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[ChainStartRequest](w, r)
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
			log.Error("failure injected on chain start", "reason", reason)
			httpx.WriteResponse(w, cfg.InjectStatusCode, ChainErrorBody{
				Reason:   reason,
				FailedAt: cfg.ID,
				Trace:    []HopResult{},
			})

			return
		}

		if err := validateChain(req.Chain, cfg.MaxChainDepth); err != nil {
			log.Error("invalid chain", "message", err.Error())
			httpx.WriteError(w, http.StatusBadRequest, err.Error())

			return
		}

		log.Info("starting chain",
			"caller_id", cfg.ID,
			"depth", len(req.Chain),
			"next", req.Chain[0],
		)

		fwdCtx, cancel := context.WithTimeout(ctx, cfg.HopTimeout)
		defer cancel()

		res, err := httpx.Post[ChainResponse](fwdCtx, req.Chain[0], RelayRequest{
			CallerID: cfg.ID,
			Chain:    req.Chain[1:],
			Trace:    []HopResult{},
		})
		if err != nil {
			log.Error("chain call failed", "message", err.Error(), "next", req.Chain[0])

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
				Trace:    []HopResult{},
			})

			return
		}

		res.CallerID = cfg.ID

		log.Info("chain completed",
			"caller_id", cfg.ID,
			"receiver_id", res.ReceiverID,
			"hops", len(res.Trace),
		)

		httpx.WriteOKResponse(w, res)
	}
}

func validateChain(chain []string, maxDepth int) error {
	if len(chain) > maxDepth {
		return fmt.Errorf("chain depth %d exceeds max %d", len(chain), maxDepth)
	}

	seen := make(map[string]struct{}, len(chain))

	for _, entry := range chain {
		u, uerr := url.ParseRequestURI(entry)
		if uerr != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("invalid chain entry %q: must be an absolute URL", entry)
		}

		if _, dup := seen[entry]; dup {
			return fmt.Errorf("invalid chain entry %q: duplicated URL (cycle)", entry)
		}

		seen[entry] = struct{}{}
	}

	return nil
}
