package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/charmingruby/pinglet/internal/platform/validator"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	http.Server
}

func NewServer(port string, validator *validator.Validator, isAvailable bool, requestTimeout time.Duration, injectStatusCode int) (*Server, chi.Router) {
	addr := ":" + port

	r := chi.NewRouter()
	r.Use(withValidator(validator), withO11y)

	var apiRouter chi.Router
	r.Route("/api", func(router chi.Router) {
		apiRouter = router
	})

	registerProbes(apiRouter, isAvailable, injectStatusCode)

	if requestTimeout <= 0 {
		requestTimeout = 5 * time.Second
	}

	return &Server{
		Server: http.Server{
			WriteTimeout: requestTimeout,
			ReadTimeout:  requestTimeout,
			IdleTimeout:  120 * time.Second,
			Addr:         addr,
			Handler:      r,
		},
	}, apiRouter
}

func (s *Server) Start() error {
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

func registerProbes(r chi.Router, isAvailable bool, injectStatusCode int) {
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if !isAvailable {
			WriteFailureInjection(w, injectStatusCode, "failure injected manually")
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
