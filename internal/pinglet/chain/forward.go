package chain

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmingruby/pinglet/internal/platform/httpx"
)

// ForwardError is returned by Forward when the downstream call fails.
// StatusCode == 0 means the downstream never responded (network/timeout);
// otherwise it carries the downstream status so callers can bubble it up.
type ForwardError struct {
	StatusCode int
	Reason     string
	FailedAt   string
	Trace      []HopResult
}

func (e *ForwardError) Error() string {
	if e.StatusCode == 0 {
		return fmt.Sprintf("chain forward failed at %s: %s", e.FailedAt, e.Reason)
	}

	return fmt.Sprintf("chain forward failed at %s with status %d: %s", e.FailedAt, e.StatusCode, e.Reason)
}

func Forward(ctx context.Context, fullURL string, body any) (*ChainResponse, error) {
	u, err := url.ParseRequestURI(fullURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, &ForwardError{Reason: fmt.Sprintf("invalid downstream URL %q", fullURL), FailedAt: fullURL}
	}

	res, err := httpx.Do[ChainResponse](ctx, httpx.NewClient(), httpx.Request{
		Method:         http.MethodPost,
		URL:            fullURL,
		ExpectedStatus: http.StatusOK,
		Body:           body,
	})
	if err == nil {
		return res, nil
	}

	if errors.Is(err, httpx.ErrUnexpectedStatusCode) {
		var status int
		_, _ = fmt.Sscanf(err.Error(), "unexpected status code: got %d,", &status)

		var downstream ChainErrorBody
		decoded := false
		if msg := err.Error(); strings.Contains(msg, "body: ") {
			raw := strings.TrimSpace(msg[strings.Index(msg, "body: ")+len("body: "):])
			if raw != "" {
				if jsonErr := json.Unmarshal([]byte(raw), &downstream); jsonErr == nil {
					decoded = downstream.Reason != "" || downstream.FailedAt != "" || downstream.Trace != nil
				}
			}
		}

		if decoded {
			return nil, &ForwardError{
				StatusCode: status,
				Reason:     downstream.Reason,
				FailedAt:   downstream.FailedAt,
				Trace:      downstream.Trace,
			}
		}

		return nil, &ForwardError{
			StatusCode: status,
			Reason:     err.Error(),
			FailedAt:   fullURL,
		}
	}

	if ctx.Err() != nil {
		return nil, &ForwardError{Reason: fmt.Sprintf("downstream call to %s: %s", fullURL, ctx.Err()), FailedAt: fullURL}
	}

	return nil, &ForwardError{Reason: fmt.Sprintf("downstream call to %s failed: %s", fullURL, err), FailedAt: fullURL}
}
