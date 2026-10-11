package chain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func relayServer(t *testing.T, id string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(Relay(newTestConfig(id), sim.New()))
	t.Cleanup(srv.Close)

	return srv
}

func TestRelay_ForwardsChain(t *testing.T) {
	var hits atomic.Int64
	srv := stubDownstream(
		t,
		http.StatusOK,
		`{"message":"pong","caller_id":"receiver-1","receiver_id":"final-node","trace":[{"id":"receiver-1","message":"pong"},{"id":"final-node","message":"pong"}]}`,
		&hits,
	)

	cfg := newTestConfig("receiver-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/relay",
		strings.NewReader(
			`{"caller_id":"origin","chain":["`+srv.URL+`"],"trace":[{"id":"origin-hop","message":"pong"}]}`,
		),
	)
	rec := httptest.NewRecorder()

	Relay(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, int64(1), hits.Load())

	var out ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	assert.Equal(t, "final-node", out.ReceiverID)
	require.Len(t, out.Trace, 2)
	assert.Equal(t, "receiver-1", out.Trace[0].ID)
}

func TestRelay_BubblesDownstreamError(t *testing.T) {
	srv := stubDownstream(
		t,
		http.StatusServiceUnavailable,
		`{"reason":"failure injected: unavailable","failed_at":"final-node","trace":[{"id":"receiver-1","message":"pong"}]}`,
		nil,
	)

	cfg := newTestConfig("receiver-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/relay",
		strings.NewReader(`{"caller_id":"origin","chain":["`+srv.URL+`"]}`),
	)
	rec := httptest.NewRecorder()

	Relay(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)

	var out ChainErrorBody
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	assert.Equal(t, "final-node", out.FailedAt)
	require.Len(t, out.Trace, 1)
}

func TestRelay_NetworkErrorKeepsTraceSoFar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	cfg := newTestConfig("receiver-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/relay",
		strings.NewReader(`{"caller_id":"origin","chain":["`+deadURL+`"],"trace":[{"id":"node-a","message":"pong"}]}`),
	)
	rec := httptest.NewRecorder()

	Relay(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusBadGateway, res.StatusCode)

	var out ChainErrorBody
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	assert.Equal(t, "receiver-1", out.FailedAt)
	require.Len(t, out.Trace, 2)
	assert.Equal(t, "node-a", out.Trace[0].ID)
	assert.Equal(t, "receiver-1", out.Trace[1].ID)
}

func TestRelay_Terminal(t *testing.T) {
	cfg := newTestConfig("receiver-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/relay",
		strings.NewReader(`{"caller_id":"origin","trace":[{"id":"node-a","message":"pong"}]}`),
	)
	rec := httptest.NewRecorder()

	Relay(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var out ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	assert.Equal(t, "origin", out.CallerID)
	assert.Equal(t, "receiver-1", out.ReceiverID)
	require.Len(t, out.Trace, 2)
	assert.Equal(t, "node-a", out.Trace[0].ID)
	assert.Equal(t, "receiver-1", out.Trace[1].ID)
}
