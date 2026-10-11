package pingpong

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	return &config.Config{
		Port:             "4000",
		ID:               "receiver-1",
		IsAvailable:      true,
		PingDelay:        0,
		PongDelay:        0,
		PingFailureRate:  0,
		PongFailureRate:  0,
		InjectStatusCode: 500,
		MaxChainDepth:    10,
		HopTimeout:       5 * time.Second,
	}
}

// fakeDownstream is a raw stub returning canned status/body and counting hits.
type fakeDownstream struct {
	status int
	body   string
	hits   atomic.Int64
}

func (f *fakeDownstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	}
}

func decodeBody(t *testing.T, res *http.Response) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))

	return body
}

func TestPong(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(cfg *config.Config)
		body       string
		wantStatus int
		minElapse  time.Duration
		check      func(t *testing.T, body map[string]any)
	}{
		{
			name:       "success returns pong with receiver id",
			mutate:     func(cfg *config.Config) {},
			body:       `{"caller_id":"caller-1"}`,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "pong", body["message"])
				assert.Equal(t, "receiver-1", body["receiver_id"])
			},
		},
		{
			name: "pong delay is applied before responding",
			mutate: func(cfg *config.Config) {
				cfg.PongDelay = 200 * time.Millisecond
			},
			body:       `{"caller_id":"caller-1"}`,
			wantStatus: http.StatusOK,
			minElapse:  200 * time.Millisecond,
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "pong", body["message"])
			},
		},
		{
			name: "unavailable injects configured status code",
			mutate: func(cfg *config.Config) {
				cfg.IsAvailable = false
			},
			body:       `{"caller_id":"caller-1"}`,
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, body map[string]any) {
				assert.Contains(t, body["reason"], "unavailable")
			},
		},
		{
			name: "failure rate of one always injects",
			mutate: func(cfg *config.Config) {
				cfg.PongFailureRate = 1
			},
			body:       `{"caller_id":"caller-1"}`,
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, body map[string]any) {
				assert.Contains(t, body["reason"], "failure rate")
			},
		},
		{
			name: "inject uses custom status code",
			mutate: func(cfg *config.Config) {
				cfg.IsAvailable = false
				cfg.InjectStatusCode = 503
			},
			body:       `{"caller_id":"caller-1"}`,
			wantStatus: http.StatusServiceUnavailable,
			check: func(t *testing.T, body map[string]any) {
				assert.Contains(t, body["reason"], "unavailable")
			},
		},
		{
			name:       "invalid payload is rejected",
			mutate:     func(cfg *config.Config) {},
			body:       `{"caller_id":`,
			wantStatus: http.StatusBadRequest,
			check:      func(t *testing.T, body map[string]any) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			tc.mutate(cfg)

			req := httptest.NewRequest(http.MethodPost, "/api/pong", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			start := time.Now()
			Pong(cfg, sim.New())(rec, req)
			assert.GreaterOrEqual(t, time.Since(start), tc.minElapse)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			assert.Equal(t, tc.wantStatus, res.StatusCode)

			var body map[string]any
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			tc.check(t, body)
		})
	}
}
