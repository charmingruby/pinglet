package pingpong_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/pingpong"
	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestConfig(id string) *config.Config {
	return &config.Config{
		Port:             "4000",
		ID:               id,
		IsAvailable:      true,
		InjectStatusCode: 500,
		MaxChainDepth:    10,
		HopTimeout:       5 * time.Second,
	}
}

func decodeBody(t *testing.T, res *http.Response) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))

	return body
}

func stubDownstream(t *testing.T, status int, body string, hits *atomic.Int64) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func callPing(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/ping", strings.NewReader(body))
	rec := httptest.NewRecorder()

	pingpong.Ping(newTestConfig("caller-a"), sim.New())(rec, req)

	return rec
}

func TestPing(t *testing.T) {
	var pongHits atomic.Int64
	srv := stubDownstream(t, http.StatusOK, `{"message":"pong","receiver_id":"pong-node"}`, &pongHits)

	t.Run("success relays pong with caller and receiver ids", func(t *testing.T) {
		rec := callPing(t, `{"to":"`+srv.URL+`"}`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var out pingpong.PingResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
		assert.Equal(t, "pong", out.Message)
		assert.Equal(t, "caller-a", out.CallerID)
		assert.Equal(t, "pong-node", out.ReceiverID)
	})

	t.Run("ping delay is applied before calling pong", func(t *testing.T) {
		cfg := newTestConfig("caller-a")
		cfg.PingDelay = 200 * time.Millisecond

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/ping",
			strings.NewReader(`{"to":"`+srv.URL+`"}`),
		)
		rec := httptest.NewRecorder()

		start := time.Now()
		pingpong.Ping(cfg, sim.New())(rec, req)
		assert.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond)

		assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
	})

	t.Run("unavailable never reaches pong", func(t *testing.T) {
		cfg := newTestConfig("caller-a")
		cfg.IsAvailable = false

		before := pongHits.Load()

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/ping",
			strings.NewReader(`{"to":"`+srv.URL+`"}`),
		)
		rec := httptest.NewRecorder()

		pingpong.Ping(cfg, sim.New())(rec, req)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.Equal(t, before, pongHits.Load())
		assert.Contains(t, decodeBody(t, res)["reason"], "unavailable")
	})

	t.Run("pong error surfaces as service unavailable", func(t *testing.T) {
		brokenSrv := stubDownstream(t, http.StatusInternalServerError, `{"reason":"boom"}`, nil)

		rec := callPing(t, `{"to":"`+brokenSrv.URL+`"}`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
		assert.Equal(t, "Pong service is unavailable", decodeBody(t, res)["message"])
	})

	t.Run("invalid destination is rejected", func(t *testing.T) {
		rec := callPing(t, `{"to":"not-a-url"}`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("invalid payload is rejected", func(t *testing.T) {
		rec := callPing(t, `{"to":`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}
