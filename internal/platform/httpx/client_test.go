package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDo(t *testing.T) {
	tests := []struct {
		name           string
		serverStatus   int
		serverBody     string
		expectedStatus int
		body           any
		wantErr        error
		check          func(t *testing.T, res *map[string]string)
	}{
		{
			name:           "decodes successful response",
			serverStatus:   http.StatusOK,
			serverBody:     `{"message":"pong"}`,
			expectedStatus: http.StatusOK,
			check: func(t *testing.T, res *map[string]string) {
				assert.Equal(t, "pong", (*res)["message"])
			},
		},
		{
			name:           "unexpected status returns typed error",
			serverStatus:   http.StatusInternalServerError,
			serverBody:     `{"reason":"boom"}`,
			expectedStatus: http.StatusOK,
			wantErr:        ErrUnexpectedStatusCode,
		},
		{
			name:           "invalid json returns decode error",
			serverStatus:   http.StatusOK,
			serverBody:     `not-json`,
			expectedStatus: http.StatusOK,
			wantErr:        ErrDecodeResponse,
		},
		{
			name:           "unmarshalable body returns encode error",
			serverStatus:   http.StatusOK,
			serverBody:     `{}`,
			expectedStatus: http.StatusOK,
			body:           func() {},
			wantErr:        ErrEncodeRequestBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.serverStatus)
				_, _ = w.Write([]byte(tc.serverBody))
			}))
			t.Cleanup(srv.Close)

			client := NewClient(srv.URL)

			res, err := Do[map[string]string](context.Background(), client, Request{
				Method:         http.MethodPost,
				Path:           "/api/pong",
				ExpectedStatus: tc.expectedStatus,
				Body:           tc.body,
			})

			if tc.wantErr == nil {
				require.NoError(t, err)
				require.NotNil(t, res)
				tc.check(t, res)
				return
			}

			require.Error(t, err)
			assert.Nil(t, res)
			assert.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
		})
	}
}

func TestDo_UnreachableHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	client := NewClient(srv.URL)

	res, err := Do[map[string]string](context.Background(), client, Request{
		Method:         http.MethodPost,
		Path:           "/api/pong",
		ExpectedStatus: http.StatusOK,
	})

	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, ErrDoRequest), "expected ErrDoRequest, got %v", err)
}

func TestDo_CanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := Do[map[string]string](ctx, client, Request{
		Method:         http.MethodPost,
		Path:           "/api/pong",
		ExpectedStatus: http.StatusOK,
	})

	require.Error(t, err)
	assert.Nil(t, res)
}
