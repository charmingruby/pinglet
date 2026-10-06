package pinglet

import (
	"net/http"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
)

type PongRequest struct {
	CallerID string `json:"caller_id"`
}

type PongResponse struct {
	Message    string `json:"message"`
	ReceiverID string `json:"receiver_id"`
}

func Pong(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id := cfg.ID
		isAvailable := cfg.IsAvailable

		log := logging.LoggerFromContext(ctx)

		if !isAvailable {
			log.Error("pong is not available",
				"isAvailableVar", isAvailable,
			)

			httpx.WriteServiceUnavailableByManualInjection(w)
			return
		}

		req, err := httpx.ParseRequest[PongRequest](w, r)
		if err != nil {
			return
		}

		log.Info("received pong request",
			"caller_id", req.CallerID,
		)

		httpx.WriteOKResponse(w, PongResponse{
			Message:    "pong",
			ReceiverID: id,
		})
	}

}
