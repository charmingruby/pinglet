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

func Relay(cfg *config.Config, networkSim *fsm.FSM[sim.Data]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := logging.LoggerFromContext(ctx)

		req, err := httpx.ParseRequest[RelayRequest](w, r)
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

		traceSoFar := make([]HopResult, 0, len(req.Trace)+1)
		traceSoFar = append(traceSoFar, req.Trace...)
		traceSoFar = append(traceSoFar, HopResult{ID: cfg.ID, Message: "pong"})

		if simReason != "" {
			log.Error("failure injected on relay", "reason", simReason)

			httpx.WriteResponse(w, cfg.InjectStatusCode, ChainErrorBody{
				Reason:   simReason,
				FailedAt: cfg.ID,
				Trace:    traceSoFar,
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

		if chainErr != "" && len(req.Chain) > 0 {
			log.Error("invalid remaining chain on relay", "message", chainErr)

			httpx.WriteResponse(w, http.StatusBadRequest, map[string]string{
				"message": chainErr,
			})

			return
		}

		if len(req.Chain) == 0 {
			log.Info("relay terminal reached", "caller_id", req.CallerID)

			httpx.WriteOKResponse(w, ChainResponse{
				Message:    "pong",
				CallerID:   req.CallerID,
				ReceiverID: cfg.ID,
				Trace:      traceSoFar,
			})

			return
		}

		next, rest := req.Chain[0], req.Chain[1:]

		log.Info("forwarding chain",
			"caller_id", req.CallerID,
			"node_id", cfg.ID,
			"next", next,
			"remaining", len(rest),
		)

		fwdCtx, cancel := context.WithTimeout(ctx, cfg.HopTimeout*time.Duration(len(req.Chain)))
		defer cancel()

		res, err := Forward(fwdCtx, next, RelayRequest{
			CallerID: cfg.ID,
			Chain:    rest,
			Trace:    traceSoFar,
		})
		if err != nil {
			log.Error("downstream call failed",
				"message", err.Error(),
				"node_id", cfg.ID,
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
					Trace:    traceSoFar,
				})

				return
			}

			httpx.WriteResponse(w, http.StatusBadGateway, ChainErrorBody{
				Reason:   err.Error(),
				FailedAt: cfg.ID,
				Trace:    traceSoFar,
			})

			return
		}

		httpx.WriteOKResponse(w, res)
	}
}
