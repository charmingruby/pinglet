package httpx

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	ErrCreateRequest        = errors.New("create request")
	ErrEncodeRequestBody    = errors.New("encode request body")
	ErrDoRequest            = errors.New("do request")
	ErrUnexpectedStatusCode = errors.New("unexpected status code")
	ErrDecodeResponse       = errors.New("decode response")
)

type Client struct {
	client  *http.Client
	baseURL string
}

type ClientOption func(*Client)

func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: baseURL,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.client.Timeout = d
	}
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.client = hc
	}
}

type Request struct {
	Body           any
	Headers        http.Header
	Method         string
	Path           string
	ExpectedStatus int
}

func Do[T any](
	ctx context.Context,
	c *Client,
	r Request,
) (*T, error) {
	url := c.baseURL + r.Path

	var bodyReader io.Reader
	if r.Body != nil {
		buf := &bytes.Buffer{}
		if err := json.MarshalWrite(buf, r.Body); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEncodeRequestBody, err)
		}
		bodyReader = buf
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateRequest, err)
	}

	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range r.Headers {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDoRequest, err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != r.ExpectedStatus {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf(
			"%w: got %d, want %d, body: %s",
			ErrUnexpectedStatusCode,
			res.StatusCode,
			r.ExpectedStatus,
			body,
		)
	}

	var result T
	if err := json.UnmarshalRead(res.Body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeResponse, err)
	}

	return &result, nil
}
