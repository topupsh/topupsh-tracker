// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package topupsh is the client services (mesher, tmphost, ...) call
// topup.sh's credit API with: Charge, Refund, Balance, Ledger and ViewToken.
//
// Every operation has a short timeout and retries network errors, 5xx and
// 429 answers within it. A charge keeps one request id across its retries,
// so topup.sh charges it at most once.
//
// The client fails closed: when topup.sh can't be reached, an operation
// returns an error wrapping ErrUnavailable and never pretends a charge went
// through. The caller decides what to do, e.g. refuse the operation or let
// it go ahead uncharged.
package topupsh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultTimeout bounds one operation, retries included.
	DefaultTimeout = 3 * time.Second
	// DefaultAttempts is how many times an operation is tried.
	DefaultAttempts = 3

	_retryDelay = 100 * time.Millisecond
)

var (
	// ErrUnavailable is wrapped by the error of an operation that could not
	// reach topup.sh: a network error, a timeout, or a 5xx or 429 answer on
	// every attempt.
	ErrUnavailable = errors.New("topup.sh is unavailable")
	// ErrInvalidConfig is returned by New for a missing or malformed base
	// URL or API key.
	ErrInvalidConfig = errors.New("invalid topup.sh client config")
)

// Config sets up a Client.
type Config struct {
	// BaseURL is topup.sh's address, e.g. https://topup.sh.
	BaseURL string
	// APIKey is the service's key (tpk_...).
	APIKey string
	// Timeout bounds one operation, retries included; DefaultTimeout when 0.
	Timeout time.Duration
	// Attempts is how many times an operation is tried; DefaultAttempts
	// when 0.
	Attempts int
	// HTTPClient sends the requests; http.DefaultClient when nil.
	HTTPClient *http.Client
}

// Client calls topup.sh. It is safe for concurrent use.
type Client struct {
	base     *url.URL
	apiKey   string
	timeout  time.Duration
	attempts int
	http     *http.Client
}

// APIError is topup.sh refusing a request, e.g. 400 for a malformed one or
// 409 for a request id reused with another charge.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("topup.sh: %d %s", e.StatusCode, e.Message)
}

// New returns a Client for cfg.
func New(cfg Config) (*Client, error) {
	base, err := url.Parse(strings.TrimSuffix(cfg.BaseURL, "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("%w: base URL %q", ErrInvalidConfig, cfg.BaseURL)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: missing API key", ErrInvalidConfig)
	}
	c := &Client{base: base, apiKey: cfg.APIKey, timeout: cfg.Timeout, attempts: cfg.Attempts, http: cfg.HTTPClient}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	if c.attempts <= 0 {
		c.attempts = DefaultAttempts
	}
	if c.http == nil {
		c.http = http.DefaultClient
	}
	return c, nil
}

// do sends one operation, retrying it until it gets an answer topup.sh
// meant, and decodes a 2xx answer, or one of the accepted statuses, into
// out. It returns the answer's status.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any, accepted ...int) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var payload []byte
	if body != nil {
		// Request bodies are plain structs that always encode.
		payload, _ = json.Marshal(body)
	}
	u := *c.base
	u.Path += path
	u.RawQuery = query.Encode()

	var lastErr error
	for attempt := 0; attempt < c.attempts; attempt++ {
		if attempt > 0 {
			t := time.NewTimer(_retryDelay << (attempt - 1))
			select {
			case <-ctx.Done():
				t.Stop()
				return 0, fmt.Errorf("%w: %w", ErrUnavailable, errors.Join(lastErr, ctx.Err()))
			case <-t.C:
			}
		}
		status, err := c.send(ctx, method, &u, payload, out, accepted)
		if !errors.Is(err, errRetry) {
			return status, err
		}
		lastErr = err
	}
	return 0, fmt.Errorf("%w: %w", ErrUnavailable, lastErr)
}

// errRetry marks an attempt worth retrying.
var errRetry = errors.New("retryable")

// send makes one attempt.
func (c *Client) send(ctx context.Context, method string, u *url.URL, payload []byte, out any, accepted []int) (int, error) {
	// Built by hand: http.NewRequest's only errors are for a bad method or
	// URL, and both are fixed here.
	req := (&http.Request{Method: method, URL: u, Header: http.Header{}}).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(payload))
		req.Body = io.NopCloser(bytes.NewReader(payload))
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errRetry, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errRetry, err)
	}
	if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
		return 0, fmt.Errorf("%w: status %d", errRetry, res.StatusCode)
	}
	if res.StatusCode/100 != 2 && !contains(accepted, res.StatusCode) {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error == "" {
			e.Error = http.StatusText(res.StatusCode)
		}
		return res.StatusCode, &APIError{StatusCode: res.StatusCode, Message: e.Error}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return res.StatusCode, fmt.Errorf("topup.sh: decode %d answer: %w", res.StatusCode, err)
	}
	return res.StatusCode, nil
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
