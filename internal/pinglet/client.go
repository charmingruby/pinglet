package pinglet

import (
	"context"
	"net/http"
	"time"

	"github.com/charmingruby/pinglet/internal/platform/httpx"
)

type Client struct {
	client *httpx.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	c := httpx.NewClient(baseURL, httpx.WithTimeout(timeout))

	return &Client{
		client: c,
	}
}

func (c *Client) Pong(ctx context.Context, path, id string) (*PongResponse, error) {
	return httpx.Do[PongResponse](ctx, c.client, httpx.Request{
		Method:         http.MethodPost,
		Path:           path,
		ExpectedStatus: 200,
		Body: PongRequest{
			CallerID: id,
		},
	})
}
