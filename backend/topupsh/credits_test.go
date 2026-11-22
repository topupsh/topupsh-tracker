// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package topupsh

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// request is what a test server saw.
type request struct {
	method, path, query, auth, contentType string
	body                                   map[string]any
}

// serve answers every request with status and answer, recording it in got.
func serve(t *testing.T, got *request, status int, answer string) *Client {
	t.Helper()
	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		*got = request{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type")}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			require.NoError(t, json.Unmarshal(b, &got.body))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	})
}

// unavailable is a Client whose server always fails.
func unavailable(t *testing.T) *Client {
	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
}

func ptr(v int64) *int64 { return &v }

func TestNewRequestID(t *testing.T) {
	a, b := NewRequestID(), NewRequestID()
	assert.Len(t, a, 26)
	assert.NotEqual(t, a, b)
}

func TestCharge(t *testing.T) {
	t.Run("charged, with a generated request id", func(t *testing.T) {
		var got request
		c := serve(t, &got, http.StatusOK, `{"requestId":"gen","charged":true,"limited":true,"balance":9,"orgRemaining":90}`)
		rs, err := c.Charge(context.Background(), ChargeRequest{Org: "o", User: "u", Credits: 1, Operation: "tool_call"})
		require.NoError(t, err)
		assert.Equal(t, &Charge{RequestID: "gen", Charged: true, Limited: true, Balance: ptr(9), OrgRemaining: ptr(90)}, rs)
		assert.Equal(t, http.MethodPost, got.method)
		assert.Equal(t, "/api/v1/charges", got.path)
		assert.Equal(t, "Bearer tpk_x", got.auth)
		assert.Equal(t, "application/json", got.contentType)
		assert.Len(t, got.body["requestId"], 26)
		assert.Equal(t, map[string]any{"requestId": got.body["requestId"], "org": "o", "user": "u", "credits": 1.0, "operation": "tool_call"}, got.body)
	})

	t.Run("refused", func(t *testing.T) {
		var got request
		c := serve(t, &got, http.StatusPaymentRequired, `{"requestId":"r1","charged":false,"limited":true,"reason":"user_limit","renewsAt":"2026-12-01T00:00:00Z","balance":0,"orgRemaining":5}`)
		rs, err := c.Charge(context.Background(), ChargeRequest{RequestID: "r1", Org: "o", User: "u", Credits: 1})
		require.NoError(t, err)
		assert.False(t, rs.Charged)
		assert.Equal(t, ReasonUserLimit, rs.Reason)
		assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), rs.RenewsAt)
		assert.Equal(t, "r1", got.body["requestId"])
	})

	t.Run("unavailable", func(t *testing.T) {
		_, err := unavailable(t).Charge(context.Background(), ChargeRequest{Org: "o", User: "u", Credits: 1})
		assert.ErrorIs(t, err, ErrUnavailable)
	})
}

func TestRefund(t *testing.T) {
	var got request
	c := serve(t, &got, http.StatusOK, `{"requestId":"r1","limited":true,"credits":1,"refundedAt":"2026-11-22T01:02:03Z","balance":10}`)
	rs, err := c.Refund(context.Background(), "r1")
	require.NoError(t, err)
	assert.Equal(t, &Refund{RequestID: "r1", Limited: true, Credits: 1, RefundedAt: time.Date(2026, 11, 22, 1, 2, 3, 0, time.UTC), Balance: ptr(10)}, rs)
	assert.Equal(t, "/api/v1/refunds", got.path)
	assert.Equal(t, map[string]any{"requestId": "r1"}, got.body)

	_, err = unavailable(t).Refund(context.Background(), "r1")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestBalance(t *testing.T) {
	var got request
	c := serve(t, &got, http.StatusOK, `{"service":"mesher","org":"o","user":"u","limited":true,"balance":12,"available":7,
		"monthly":{"limit":10,"used":5,"remaining":5},"orgUsage":{"limit":100,"userLimit":10,"used":40,"remaining":60},
		"grants":[{"id":"g","credits":5,"remaining":2,"createdAt":"2026-11-01T00:00:00Z","expiresAt":"2026-12-02T00:00:00Z"}],
		"renewsAt":"2026-12-01T00:00:00Z"}`)
	rs, err := c.Balance(context.Background(), "o", "u")
	require.NoError(t, err)
	assert.Equal(t, &Balance{
		Service: "mesher", Org: "o", User: "u", Limited: true, Balance: 12, Available: 7,
		Monthly:  MonthlyCredits{Limit: 10, Used: 5, Remaining: 5},
		OrgUsage: OrgUsage{Limit: 100, UserLimit: ptr(10), Used: 40, Remaining: 60},
		Grants: []Grant{{ID: "g", Credits: 5, Remaining: 2,
			CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)}},
		RenewsAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}, rs)
	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "/api/v1/balances", got.path)
	assert.Equal(t, "org=o&user=u", got.query)
	assert.Empty(t, got.contentType)

	_, err = unavailable(t).Balance(context.Background(), "o", "u")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestLedger(t *testing.T) {
	var got request
	c := serve(t, &got, http.StatusOK, `{"entries":[{"id":"e","service":"mesher","org":"o","user":"u","kind":"charge","amount":-1,
		"balanceAfter":4,"actor":"mesher","operation":"tool_call","createdAt":"2026-11-22T00:00:00Z"}],"nextCursor":"e"}`)
	rs, err := c.Ledger(context.Background(), LedgerRequest{Org: "o", User: "u", Cursor: "c", Limit: 20})
	require.NoError(t, err)
	assert.Equal(t, &LedgerPage{Entries: []LedgerEntry{{ID: "e", Service: "mesher", Org: "o", User: "u", Kind: "charge", Amount: -1,
		BalanceAfter: 4, Actor: "mesher", Operation: "tool_call", CreatedAt: time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)}}, NextCursor: "e"}, rs)
	assert.Equal(t, "/api/v1/ledger", got.path)
	assert.Equal(t, "cursor=c&limit=20&org=o&user=u", got.query)

	_, err = c.Ledger(context.Background(), LedgerRequest{})
	require.NoError(t, err)
	assert.Empty(t, got.query)

	_, err = unavailable(t).Ledger(context.Background(), LedgerRequest{})
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestViewToken(t *testing.T) {
	var got request
	c := serve(t, &got, http.StatusCreated, `{"token":"t","expiresAt":"2026-11-22T01:00:00Z","url":"https://topup.sh/balance#token=t"}`)
	rs, err := c.ViewToken(context.Background(), "o", "u")
	require.NoError(t, err)
	assert.Equal(t, &ViewToken{Token: "t", ExpiresAt: time.Date(2026, 11, 22, 1, 0, 0, 0, time.UTC), URL: "https://topup.sh/balance#token=t"}, rs)
	assert.Equal(t, "/api/v1/view-tokens", got.path)
	assert.Equal(t, map[string]any{"org": "o", "user": "u"}, got.body)

	_, err = unavailable(t).ViewToken(context.Background(), "o", "u")
	assert.ErrorIs(t, err, ErrUnavailable)
}
