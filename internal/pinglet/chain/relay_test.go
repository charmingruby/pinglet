package chain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relayServer spins a real Relay node behind an httptest server.
func relayServer(t *testing.T, id string) *httptest.Server {
	t.Helper()

	cfg := testConfig()
	cfg.ID = id

	srv := httptest.NewServer(Relay(cfg, sim.New()))
	t.Cleanup(srv.Close)

	return srv
}

func callChain(t *testing.T, callerID, body string) *httptest.ResponseRecorder {
	t.Helper()

	cfg := testConfig()
	cfg.ID = callerID

	req := httptest.NewRequest(http.MethodPost, "/api/chain", strings.NewReader(body))
	rec := httptest.NewRecorder()

	Chain(cfg, sim.New())(rec, req)

	return rec
}

func TestChain_SingleHop(t *testing.T) {
	nodeB := relayServer(t, "node-b")

	rec := callChain(t, "node-a", `{"chain":["`+nodeB.URL+`"]}`)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var out ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))

	assert.Equal(t, "pong", out.Message)
	assert.Equal(t, "node-a", out.CallerID)
	assert.Equal(t, "node-b", out.ReceiverID)
	require.Len(t, out.Trace, 1)
	assert.Equal(t, "node-b", out.Trace[0].ID)
}

func TestChain_TwoHops(t *testing.T) {
	nodeC := relayServer(t, "node-c")
	nodeB := relayServer(t, "node-b")

	rec := callChain(t, "node-a", `{"chain":["`+nodeB.URL+`","`+nodeC.URL+`"]}`)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var out ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))

	assert.Equal(t, "node-a", out.CallerID)
	assert.Equal(t, "node-c", out.ReceiverID)
	require.Len(t, out.Trace, 2)
	assert.Equal(t, "node-b", out.Trace[0].ID)
	assert.Equal(t, "node-c", out.Trace[1].ID)
}

func TestChain(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		check      func(t *testing.T, body map[string]any)
	}{
		{
			name:       "empty chain is rejected",
			body:       `{"chain":[]}`,
			wantStatus: http.StatusBadRequest,
			check:      func(t *testing.T, body map[string]any) {},
		},
		{
			name:       "duplicated chain is rejected",
			body:       `{"chain":["http://b:4000/api/relay","http://b:4000/api/relay"]}`,
			wantStatus: http.StatusBadRequest,
			check:      func(t *testing.T, body map[string]any) {},
		},
		{
			name:       "invalid url is rejected",
			body:       `{"chain":["not-a-url"]}`,
			wantStatus: http.StatusBadRequest,
			check:      func(t *testing.T, body map[string]any) {},
		},
		{
			name:       "unreachable downstream surfaces as bad gateway",
			body:       `{"chain":["http://127.0.0.1:1/api/relay"]}`,
			wantStatus: http.StatusBadGateway,
			check: func(t *testing.T, body map[string]any) {
				assert.Equal(t, "node-a", body["failed_at"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := callChain(t, "node-a", tc.body)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			assert.Equal(t, tc.wantStatus, res.StatusCode)
			tc.check(t, decodeBody(t, res))
		})
	}
}

func TestChain_SimFailureNeverReachesDownstream(t *testing.T) {
	downstream := &fakeDownstream{
		status: http.StatusOK,
		body:   `{"message":"pong","caller_id":"x","receiver_id":"node-b","trace":[]}`,
	}
	srv := httptest.NewServer(downstream.handler())
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.ID = "node-a"
	cfg.IsAvailable = false

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chain",
		strings.NewReader(`{"chain":["`+srv.URL+`"]}`),
	)
	rec := httptest.NewRecorder()

	Chain(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Zero(t, downstream.hits.Load())

	body := decodeBody(t, res)
	assert.Contains(t, body["reason"], "unavailable")
	assert.Equal(t, "node-a", body["failed_at"])
}

func TestChain_DepthExceeded(t *testing.T) {
	nodeB := relayServer(t, "node-b")

	cfg := testConfig()
	cfg.ID = "node-a"
	cfg.MaxChainDepth = 1

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chain",
		strings.NewReader(`{"chain":["`+nodeB.URL+`","http://c:4000/api/relay"]}`),
	)
	rec := httptest.NewRecorder()

	Chain(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestRelay_ForwardsChain(t *testing.T) {
	downstream := &fakeDownstream{
		status: http.StatusOK,
		body:   `{"message":"pong","caller_id":"receiver-1","receiver_id":"final-node","trace":[{"id":"receiver-1","message":"pong"},{"id":"final-node","message":"pong"}]}`,
	}
	srv := httptest.NewServer(downstream.handler())
	t.Cleanup(srv.Close)

	cfg := testConfig()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/relay",
		strings.NewReader(`{"caller_id":"origin","chain":["`+srv.URL+`"],"trace":[{"id":"origin-hop","message":"pong"}]}`),
	)
	rec := httptest.NewRecorder()

	Relay(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, int64(1), downstream.hits.Load())

	var out ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	assert.Equal(t, "final-node", out.ReceiverID)
	require.Len(t, out.Trace, 2)
	assert.Equal(t, "receiver-1", out.Trace[0].ID)
}

func TestRelay_BubblesDownstreamError(t *testing.T) {
	downstream := &fakeDownstream{
		status: http.StatusServiceUnavailable,
		body:   `{"reason":"failure injected: unavailable","failed_at":"final-node","trace":[{"id":"receiver-1","message":"pong"}]}`,
	}
	srv := httptest.NewServer(downstream.handler())
	t.Cleanup(srv.Close)

	cfg := testConfig()

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

	cfg := testConfig()

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
	cfg := testConfig()

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
