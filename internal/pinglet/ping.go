package pinglet

import (
	"net/http"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/platform/httpx"
	"github.com/charmingruby/pinglet/internal/platform/logging"
)

type PingRequest struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

type PingResponse struct {
	Message    string `json:"message"`
	ReceiverID string `json:"receiver_id"`
	CallerID   string `json:"called_id"`
}

func Ping(cfg *config.Config) http.HandlerFunc {
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

		req, err := httpx.ParseRequest[PingRequest](w, r)
		if err != nil {
			return
		}

		cl := NewClient(req.URL)

		log.Info("trying to call pong",
			"caller_id", id,
		)

		pong, err := cl.Pong(ctx, req.Path, id)
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
			"caller_id", id,
			"receiver_id", pong.ReceiverID,
		)

		httpx.WriteOKResponse(w, PingResponse{
			Message:    pong.Message,
			ReceiverID: pong.ReceiverID,
			CallerID:   id,
		})
	}
}
