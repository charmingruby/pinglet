package pingpong

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func callPing(t *testing.T, cfgID, body string) *httptest.ResponseRecorder {
	t.Helper()

	cfg := testConfig()
	cfg.ID = cfgID

	req := httptest.NewRequest(http.MethodPost, "/api/ping", strings.NewReader(body))
	rec := httptest.NewRecorder()

	Ping(cfg, sim.New())(rec, req)

	return rec
}

func TestPing(t *testing.T) {
	pong := &fakeDownstream{
		status: http.StatusOK,
		body:   `{"message":"pong","receiver_id":"pong-node"}`,
	}
	srv := httptest.NewServer(pong.handler())
	t.Cleanup(srv.Close)

	t.Run("success relays pong with caller and receiver ids", func(t *testing.T) {
		rec := callPing(t, "caller-a", `{"to":"`+srv.URL+`"}`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var out PingResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
		assert.Equal(t, "pong", out.Message)
		assert.Equal(t, "caller-a", out.CallerID)
		assert.Equal(t, "pong-node", out.ReceiverID)
	})

	t.Run("ping delay is applied before calling pong", func(t *testing.T) {
		cfg := testConfig()
		cfg.ID = "caller-a"
		cfg.PingDelay = 200 * time.Millisecond

		req := httptest.NewRequest(http.MethodPost, "/api/ping", strings.NewReader(`{"to":"`+srv.URL+`"}`))
		rec := httptest.NewRecorder()

		start := time.Now()
		Ping(cfg, sim.New())(rec, req)
		assert.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond)

		assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
	})

	t.Run("unavailable never reaches pong", func(t *testing.T) {
		cfg := testConfig()
		cfg.ID = "caller-a"
		cfg.IsAvailable = false

		before := pong.hits.Load()

		req := httptest.NewRequest(http.MethodPost, "/api/ping", strings.NewReader(`{"to":"`+srv.URL+`"}`))
		rec := httptest.NewRecorder()

		Ping(cfg, sim.New())(rec, req)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.Equal(t, before, pong.hits.Load())
		assert.Contains(t, decodeBody(t, res)["reason"], "unavailable")
	})

	t.Run("pong error surfaces as service unavailable", func(t *testing.T) {
		broken := &fakeDownstream{status: http.StatusInternalServerError, body: `{"reason":"boom"}`}
		brokenSrv := httptest.NewServer(broken.handler())
		t.Cleanup(brokenSrv.Close)

		rec := callPing(t, "caller-a", `{"to":"`+brokenSrv.URL+`"}`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
		assert.Equal(t, "Pong service is unavailable", decodeBody(t, res)["message"])
	})

	t.Run("invalid destination is rejected", func(t *testing.T) {
		rec := callPing(t, "caller-a", `{"to":"not-a-url"}`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("invalid payload is rejected", func(t *testing.T) {
		rec := callPing(t, "caller-a", `{"to":`)

		res := rec.Result()
		defer func() { _ = res.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}
