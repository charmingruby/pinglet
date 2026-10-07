package pinglet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePong struct {
	status int
	sleep  time.Duration
	hits   atomic.Int64
	caller atomic.Value
}

func (f *fakePong) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)

		var req PongRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.caller.Store(req.CallerID)

		if f.sleep > 0 {
			time.Sleep(f.sleep)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		_, _ = fmt.Fprintf(w, `{"message":"pong","receiver_id":"pong-node"}`)
	}
}

func pingBody(url string) string {
	return fmt.Sprintf(`{"url":%q,"path":"/api/pong"}`, url)
}

func TestPing(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(cfg *config.Config)
		pongStatus int
		pongSleep  time.Duration
		wantStatus int
		minElapse  time.Duration
		wantHits   int64
		check      func(t *testing.T, body map[string]any)
	}{
		{
			name:       "success relays pong with caller and receiver ids",
			mutate:     func(cfg *config.Config) {},
			pongStatus: http.StatusOK,
			wantStatus: http.StatusOK,
			wantHits:   1,
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "pong", body["message"])
				assert.Equal(t, "caller-a", body["caller_id"])
				assert.Equal(t, "pong-node", body["receiver_id"])
			},
		},
		{
			name: "ping delay is applied before calling pong",
			mutate: func(cfg *config.Config) {
				cfg.PingDelay = 200 * time.Millisecond
			},
			pongStatus: http.StatusOK,
			wantStatus: http.StatusOK,
			minElapse:  200 * time.Millisecond,
			wantHits:   1,
			check:      func(t *testing.T, body map[string]any) {},
		},
		{
			name: "unavailable never reaches pong",
			mutate: func(cfg *config.Config) {
				cfg.IsAvailable = false
			},
			pongStatus: http.StatusOK,
			wantStatus: http.StatusInternalServerError,
			wantHits:   0,
			check: func(t *testing.T, body map[string]any) {
				assert.Contains(t, body["reason"], "unavailable")
			},
		},
		{
			name: "ping failure rate of one never reaches pong",
			mutate: func(cfg *config.Config) {
				cfg.PingFailureRate = 1
			},
			pongStatus: http.StatusOK,
			wantStatus: http.StatusInternalServerError,
			wantHits:   0,
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
			pongStatus: http.StatusOK,
			wantStatus: http.StatusServiceUnavailable,
			wantHits:   0,
			check:      func(t *testing.T, body map[string]any) {},
		},
		{
			name:       "pong error surfaces as internal server error",
			mutate:     func(cfg *config.Config) {},
			pongStatus: http.StatusInternalServerError,
			wantStatus: http.StatusInternalServerError,
			wantHits:   1,
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "Internal Server Error", body["message"])
			},
		},
		{
			name: "slow pong within configured delay is tolerated",
			mutate: func(cfg *config.Config) {
				cfg.PongDelay = 300 * time.Millisecond
			},
			pongStatus: http.StatusOK,
			pongSleep:  300 * time.Millisecond,
			wantStatus: http.StatusOK,
			minElapse:  300 * time.Millisecond,
			wantHits:   1,
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "pong", body["message"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.ID = "caller-a"
			tc.mutate(cfg)

			fake := &fakePong{status: tc.pongStatus, sleep: tc.pongSleep}
			srv := httptest.NewServer(fake.handler())
			t.Cleanup(srv.Close)

			req := httptest.NewRequest(http.MethodPost, "/api/ping", strings.NewReader(pingBody(srv.URL)))
			rec := httptest.NewRecorder()

			start := time.Now()
			Ping(cfg, sim.New())(rec, req)
			assert.GreaterOrEqual(t, time.Since(start), tc.minElapse)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			assert.Equal(t, tc.wantStatus, res.StatusCode)
			assert.Equal(t, tc.wantHits, fake.hits.Load())

			var body map[string]any
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			tc.check(t, body)

			if tc.wantHits > 0 {
				caller, _ := fake.caller.Load().(string)
				assert.Equal(t, "caller-a", caller)
			}
		})
	}
}
