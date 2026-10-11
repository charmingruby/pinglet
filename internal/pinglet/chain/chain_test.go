package chain_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmingruby/pinglet/config"
	"github.com/charmingruby/pinglet/internal/pinglet/chain"
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

func callChain(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/chain", strings.NewReader(body))
	rec := httptest.NewRecorder()

	chain.Chain(newTestConfig("node-a"), sim.New())(rec, req)

	return rec
}

func TestChain_SingleHop(t *testing.T) {
	nodeB := relayServer(t, "node-b")

	rec := callChain(t, `{"chain":["`+nodeB.URL+`"]}`)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var out chain.ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))

	assert.Equal(t, "node-a", out.CallerID)
	assert.Equal(t, "node-b", out.ReceiverID)
	require.Len(t, out.Trace, 1)
	assert.Equal(t, "node-b", out.Trace[0].ID)
}

func TestChain_TwoHops(t *testing.T) {
	nodeC := relayServer(t, "node-c")
	nodeB := relayServer(t, "node-b")

	rec := callChain(t, `{"chain":["`+nodeB.URL+`","`+nodeC.URL+`"]}`)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var out chain.ChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))

	assert.Equal(t, "node-a", out.CallerID)
	assert.Equal(t, "node-c", out.ReceiverID)
	require.Len(t, out.Trace, 2)
	assert.Equal(t, "node-b", out.Trace[0].ID)
	assert.Equal(t, "node-c", out.Trace[1].ID)
}

func TestChain(t *testing.T) {
	tests := []struct {
		check      func(t *testing.T, body map[string]any)
		name       string
		body       string
		wantStatus int
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
			rec := callChain(t, tc.body)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			assert.Equal(t, tc.wantStatus, res.StatusCode)
			tc.check(t, decodeBody(t, res))
		})
	}
}

func TestChain_SimFailureNeverReachesDownstream(t *testing.T) {
	var hits atomic.Int64
	srv := stubDownstream(
		t,
		http.StatusOK,
		`{"message":"pong","caller_id":"x","receiver_id":"node-b","trace":[]}`,
		&hits,
	)

	cfg := newTestConfig("node-a")
	cfg.IsAvailable = false

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/chain",
		strings.NewReader(`{"chain":["`+srv.URL+`"]}`),
	)
	rec := httptest.NewRecorder()

	chain.Chain(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Zero(t, hits.Load())

	body := decodeBody(t, res)
	assert.Contains(t, body["reason"], "unavailable")
	assert.Equal(t, "node-a", body["failed_at"])
}

func TestChain_DepthExceeded(t *testing.T) {
	nodeB := relayServer(t, "node-b")

	cfg := newTestConfig("node-a")
	cfg.MaxChainDepth = 1

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/api/chain",
		strings.NewReader(`{"chain":["`+nodeB.URL+`","http://c:4000/api/relay"]}`),
	)
	rec := httptest.NewRecorder()

	chain.Chain(cfg, sim.New())(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestChain_BubblesDownstreamError(t *testing.T) {
	srv := stubDownstream(
		t,
		http.StatusServiceUnavailable,
		`{"reason":"failure injected: unavailable","failed_at":"final-node","trace":[{"id":"node-b","message":"pong"}]}`,
		nil,
	)

	rec := callChain(t, `{"chain":["`+srv.URL+`"]}`)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusServiceUnavailable, res.StatusCode)

	var out chain.ChainErrorBody
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	assert.Equal(t, "final-node", out.FailedAt)
	assert.Equal(t, "failure injected: unavailable", out.Reason)
	require.Len(t, out.Trace, 1)
	assert.Equal(t, "node-b", out.Trace[0].ID)
}
