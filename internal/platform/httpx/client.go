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

type StatusError struct {
	Body   []byte
	Status int
	Want   int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: got %d, want %d, body: %s",
		ErrUnexpectedStatusCode, e.Status, e.Want, e.Body)
}

func (e *StatusError) Unwrap() error { return ErrUnexpectedStatusCode }

type Client struct {
	client *http.Client
}

func NewClient() *Client {
	return &Client{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

var DefaultClient = NewClient()

type Request struct {
	Body           any
	Headers        http.Header
	Method         string
	URL            string
	ExpectedStatus int
}

func Post[T any](ctx context.Context, url string, body any) (*T, error) {
	return Do[T](ctx, DefaultClient, Request{
		Method:         http.MethodPost,
		URL:            url,
		ExpectedStatus: http.StatusOK,
		Body:           body,
	})
}

func Do[T any](
	ctx context.Context,
	c *Client,
	r Request,
) (*T, error) {
	var bodyReader io.Reader
	if r.Body != nil {
		buf := &bytes.Buffer{}
		if err := json.MarshalWrite(buf, r.Body); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEncodeRequestBody, err)
		}
		bodyReader = buf
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, bodyReader)
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
		return nil, &StatusError{Status: res.StatusCode, Want: r.ExpectedStatus, Body: body}
	}

	var result T
	if err := json.UnmarshalRead(res.Body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeResponse, err)
	}

	return &result, nil
}
