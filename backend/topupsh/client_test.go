// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package topupsh

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient serves h and returns a Client for it with fast retries.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL + "/", APIKey: "tpk_x", Timeout: time.Second, Attempts: 3})
	require.NoError(t, err)
	return c
}

func TestNew(t *testing.T) {
	for _, base := range []string{"", "topup.sh", "ftp://topup.sh", "https://", "http://bad host"} {
		_, err := New(Config{BaseURL: base, APIKey: "tpk_x"})
		assert.ErrorIs(t, err, ErrInvalidConfig, base)
	}
	_, err := New(Config{BaseURL: "https://topup.sh"})
	assert.ErrorIs(t, err, ErrInvalidConfig)

	c, err := New(Config{BaseURL: "https://topup.sh/", APIKey: "tpk_x"})
	require.NoError(t, err)
	assert.Equal(t, "https://topup.sh", c.base.String())
	assert.Equal(t, DefaultTimeout, c.timeout)
	assert.Equal(t, DefaultAttempts, c.attempts)
	assert.Same(t, http.DefaultClient, c.http)

	hc := &http.Client{}
	c, err = New(Config{BaseURL: "http://localhost:8080", APIKey: "tpk_x", Timeout: time.Second, Attempts: 5, HTTPClient: hc})
	require.NoError(t, err)
	assert.Equal(t, time.Second, c.timeout)
	assert.Equal(t, 5, c.attempts)
	assert.Same(t, hc, c.http)
}

func TestAPIError(t *testing.T) {
	assert.Equal(t, "topup.sh: 409 conflict", (&APIError{StatusCode: 409, Message: "conflict"}).Error())
}

func TestDoRetries(t *testing.T) {
	t.Run("5xx then 429 then success, same body", func(t *testing.T) {
		var n atomic.Int32
		var bodies []string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			bodies = append(bodies, string(b))
			switch n.Add(1) {
			case 1:
				w.WriteHeader(http.StatusBadGateway)
			case 2:
				w.WriteHeader(http.StatusTooManyRequests)
			default:
				_, _ = w.Write([]byte(`{"requestId":"r1","charged":true,"limited":true}`))
			}
		})
		rs, err := c.Charge(context.Background(), ChargeRequest{RequestID: "r1", Org: "o", User: "u", Credits: 1})
		require.NoError(t, err)
		assert.True(t, rs.Charged)
		assert.Equal(t, int32(3), n.Load())
		require.Len(t, bodies, 3)
		assert.Equal(t, bodies[0], bodies[2])
	})

	t.Run("every attempt fails", func(t *testing.T) {
		var n atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			n.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		_, err := c.Balance(context.Background(), "o", "u")
		assert.ErrorIs(t, err, ErrUnavailable)
		assert.ErrorContains(t, err, "status 503")
		assert.Equal(t, int32(3), n.Load())
	})

	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		c, err := New(Config{BaseURL: srv.URL, APIKey: "tpk_x", Attempts: 2})
		require.NoError(t, err)
		_, err = c.Refund(context.Background(), "r1")
		assert.ErrorIs(t, err, ErrUnavailable)
	})

	t.Run("timeout while waiting to retry", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)
		c, err := New(Config{BaseURL: srv.URL, APIKey: "tpk_x", Timeout: 20 * time.Millisecond, Attempts: 5})
		require.NoError(t, err)
		start := time.Now()
		_, err = c.Balance(context.Background(), "o", "u")
		assert.ErrorIs(t, err, ErrUnavailable)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), time.Second)
	})

	t.Run("truncated body is retried", func(t *testing.T) {
		var n atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if n.Add(1) == 1 {
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte(`{`))
				return
			}
			_, _ = w.Write([]byte(`{"token":"t"}`))
		})
		rs, err := c.ViewToken(context.Background(), "o", "u")
		require.NoError(t, err)
		assert.Equal(t, "t", rs.Token)
		assert.Equal(t, int32(2), n.Load())
	})
}

func TestDoAnswers(t *testing.T) {
	t.Run("API error with message", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"request id was used for another charge"}`))
		})
		_, err := c.Charge(context.Background(), ChargeRequest{RequestID: "r1"})
		var apiErr *APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusConflict, apiErr.StatusCode)
		assert.Equal(t, "request id was used for another charge", apiErr.Message)
		assert.False(t, errors.Is(err, ErrUnavailable))
	})

	t.Run("API error without a body", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
		_, err := c.Ledger(context.Background(), LedgerRequest{})
		var apiErr *APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "Forbidden", apiErr.Message)
	})

	t.Run("malformed answer", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`nope`))
		})
		_, err := c.Balance(context.Background(), "o", "u")
		assert.ErrorContains(t, err, "decode 200 answer")
		assert.False(t, errors.Is(err, ErrUnavailable))
	})
}
