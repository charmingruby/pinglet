package pinglet

import (
	"context"
	"net/http"

	"github.com/charmingruby/pinglet/internal/platform/httpx"
)

type Client struct {
	client *httpx.Client
}

func NewClient(baseURL string) *Client {
	c := httpx.NewClient(baseURL)

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
