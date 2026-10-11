package chain

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmingruby/pinglet/config"
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
