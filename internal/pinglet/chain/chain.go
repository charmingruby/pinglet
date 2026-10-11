package chain

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/charmingruby/fsm/fsm"
	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
)

// ChainStartRequest opens a multi-hop chain. Chain holds absolute relay URLs
// in visit order:
// {"chain": ["http://b:4000/api/relay", "http://c:4000/api/relay"]}.
type ChainStartRequest struct {
	Chain []string `json:"chain" validate:"required,min=1,dive,url"`
}

// RelayRequest is the body every downstream relay node expects. Chain holds
// the hops after the current one; Trace holds the hops visited so far.
type RelayRequest struct {
	CallerID string      `json:"caller_id" validate:"required"`
	Chain    []string    `json:"chain,omitempty" validate:"dive,url"`
	Trace    []HopResult `json:"trace,omitempty"`
}

func Chain(cfg *config.Config, networkSim *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[ChainStartRequest](w, r)
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
			log.Error("failure injected on chain start", "reason", simReason)

			httpx.WriteResponse(w, cfg.InjectStatusCode, ChainErrorBody{
				Reason:   simReason,
				FailedAt: cfg.ID,
				Trace:    []HopResult{},
			})

			return
		}

		chainErr := ""
		if len(req.Chain) > cfg.MaxChainDepth {
			chainErr = fmt.Sprintf("chain depth %d exceeds max %d", len(req.Chain), cfg.MaxChainDepth)
		} else {
			seen := make(map[string]struct{}, len(req.Chain))
			for _, entry := range req.Chain {
				u, err := url.ParseRequestURI(entry)
				if err != nil || u.Scheme == "" || u.Host == "" {
					chainErr = fmt.Sprintf("invalid chain entry %q: must be an absolute URL", entry)
					break
				}

				if _, dup := seen[entry]; dup {
					chainErr = fmt.Sprintf("invalid chain entry %q: duplicated URL (cycle)", entry)
					break
				}

				seen[entry] = struct{}{}
			}
		}

		if chainErr != "" {
			log.Error("invalid chain", "message", chainErr)

			httpx.WriteResponse(w, http.StatusBadRequest, map[string]string{
				"message": chainErr,
			})

			return
		}

		next, rest := req.Chain[0], req.Chain[1:]

		log.Info("starting chain",
			"caller_id", cfg.ID,
			"depth", len(req.Chain),
			"next", next,
		)

		fwdCtx, cancel := context.WithTimeout(ctx, cfg.HopTimeout*time.Duration(len(req.Chain)))
		defer cancel()

		res, err := Forward(fwdCtx, next, RelayRequest{
			CallerID: cfg.ID,
			Chain:    rest,
			Trace:    []HopResult{},
		})
		if err != nil {
			log.Error("chain call failed",
				"message", err.Error(),
				"next", next,
			)

			var ferr *ForwardError
			if errors.As(err, &ferr) {
				if ferr.StatusCode != 0 {
					trace := ferr.Trace
					if trace == nil {
						trace = []HopResult{}
					}

					httpx.WriteResponse(w, ferr.StatusCode, ChainErrorBody{
						Reason:   ferr.Reason,
						FailedAt: ferr.FailedAt,
						Trace:    trace,
					})

					return
				}

				httpx.WriteResponse(w, http.StatusBadGateway, ChainErrorBody{
					Reason:   ferr.Reason,
					FailedAt: cfg.ID,
					Trace:    []HopResult{},
				})

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
