package httpx

import (
	"net/http"

	"github.com/charmingruby/pinglet/internal/platform/logging"
	"github.com/charmingruby/pinglet/internal/platform/validator"
)

func withValidator(v *validator.Validator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := validator.WithValidator(r.Context(), v)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func withO11y(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logging.Log.With(
			"path", r.URL.Path,
			"method", r.Method,
		)

		ctx := logging.WithLogger(r.Context(), log)

		log.InfoContext(ctx, "request started")
		defer log.InfoContext(ctx, "request finished")

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
