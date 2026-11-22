// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package topupsh

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Refusal reasons of a charge.
const (
	ReasonOrgLimit  = "org_limit"
	ReasonUserLimit = "user_limit"
)

type (
	// ChargeRequest is one charge for one operation.
	ChargeRequest struct {
		// RequestID makes the charge safe to send again: topup.sh charges a
		// request id once. Charge generates one when it is empty.
		RequestID string `json:"requestId"`
		Org       string `json:"org"`
		User      string `json:"user"`
		Credits   int64  `json:"credits"`
		// Operation is what the credits pay for, e.g. "tool_call".
		Operation string `json:"operation"`
	}

	// Charge is a charge's outcome.
	Charge struct {
		// RequestID is what Refund takes to give the credits back.
		RequestID string `json:"requestId"`
		// Charged is true when the operation may go ahead.
		Charged bool `json:"charged"`
		// Limited is false while the service's credit limiting is off:
		// the charge is allowed and nothing is recorded.
		Limited bool `json:"limited"`
		// Reason is why the charge was refused: ReasonOrgLimit or
		// ReasonUserLimit.
		Reason string `json:"reason,omitempty"`
		// RenewsAt is when a refused charge's limits renew.
		RenewsAt time.Time `json:"renewsAt"`
		// Balance is the user's unexpired credits after the charge.
		Balance *int64 `json:"balance,omitempty"`
		// OrgRemaining is what is left of the org limit this month.
		OrgRemaining *int64 `json:"orgRemaining,omitempty"`
	}

	// Refund is a refunded charge.
	Refund struct {
		// RequestID is the refund's own request id.
		RequestID string `json:"requestId"`
		// ChargeRequestID is the request id of the refunded charge.
		ChargeRequestID string `json:"chargeRequestId"`
		Limited         bool   `json:"limited"`
		// Credits is what was given back; credits of expired buckets are
		// not.
		Credits    int64     `json:"credits"`
		RefundedAt time.Time `json:"refundedAt"`
		Balance    *int64    `json:"balance,omitempty"`
	}

	// Balance is one user's credits in the service.
	Balance struct {
		Service string `json:"service"`
		Org     string `json:"org"`
		User    string `json:"user"`
		// Limited is false while the service's credit limiting is off.
		Limited bool `json:"limited"`
		// Balance is the user's unexpired credits, as the ledger counts
		// them.
		Balance int64 `json:"balance"`
		// Available is what a charge can spend now.
		Available int64          `json:"available"`
		Monthly   MonthlyCredits `json:"monthly"`
		OrgUsage  OrgUsage       `json:"orgUsage"`
		Grants    []Grant        `json:"grants"`
		// RenewsAt is when the monthly limits renew.
		RenewsAt time.Time `json:"renewsAt"`
	}

	// MonthlyCredits is a user's monthly allowance this month.
	MonthlyCredits struct {
		Limit     int64 `json:"limit"`
		Used      int64 `json:"used"`
		Remaining int64 `json:"remaining"`
	}

	// OrgUsage is an org's monthly limits and usage this month.
	OrgUsage struct {
		Limit int64 `json:"limit"`
		// UserLimit is nil while users are uncapped.
		UserLimit *int64 `json:"userLimit"`
		Used      int64  `json:"used"`
		Remaining int64  `json:"remaining"`
	}

	// Grant is an unexpired grant of extra credits.
	Grant struct {
		ID        string    `json:"id"`
		Credits   int64     `json:"credits"`
		Remaining int64     `json:"remaining"`
		CreatedAt time.Time `json:"createdAt"`
		ExpiresAt time.Time `json:"expiresAt"`
	}

	// LedgerRequest selects a page of the service's ledger.
	LedgerRequest struct {
		// Org and User narrow the ledger; User needs Org.
		Org  string
		User string
		// Cursor is the previous page's NextCursor; empty for the first.
		Cursor string
		// Limit is the page size, 1..200; topup.sh's default when 0.
		Limit int
	}

	// LedgerPage is a page of the ledger, newest first.
	LedgerPage struct {
		Entries []LedgerEntry `json:"entries"`
		// NextCursor reads the next page; it is empty on the last one.
		NextCursor string `json:"nextCursor,omitempty"`
	}

	// LedgerEntry is one change to a user's balance.
	LedgerEntry struct {
		ID           string    `json:"id"`
		Service      string    `json:"service"`
		Org          string    `json:"org"`
		User         string    `json:"user"`
		Kind         string    `json:"kind"`
		Amount       int64     `json:"amount"`
		BalanceAfter int64     `json:"balanceAfter"`
		Actor        string    `json:"actor"`
		Operation    string    `json:"operation,omitempty"`
		CreatedAt    time.Time `json:"createdAt"`
	}

	// ViewToken lets one user read their own credits on topup.sh until it
	// expires.
	ViewToken struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expiresAt"`
		// URL opens the user's balance page with the token; it is empty
		// when topup.sh has no base URL.
		URL string `json:"url,omitempty"`
	}
)

// NewRequestID returns a random request id of 26 characters; topup.sh
// takes up to 32.
func NewRequestID() string {
	return rand.Text()
}

// Charge charges a user's credits. A refused charge is not an error: it
// comes back with Charged false and the Reason. Retries reuse the request
// id, so the charge happens at most once.
func (c *Client) Charge(ctx context.Context, rq ChargeRequest) (*Charge, error) {
	if rq.RequestID == "" {
		rq.RequestID = NewRequestID()
	}
	var rs Charge
	if _, err := c.do(ctx, http.MethodPost, "/api/v1/charges", nil, rq, &rs, http.StatusPaymentRequired); err != nil {
		return nil, err
	}
	return &rs, nil
}

// Refund gives back the credits of the charge with chargeRequestID, e.g.
// after the operation it paid for failed. The refund gets a request id of
// its own, which its retries reuse. Refunding twice refunds once.
func (c *Client) Refund(ctx context.Context, chargeRequestID string) (*Refund, error) {
	var rs Refund
	body := struct {
		RequestID       string `json:"requestId"`
		ChargeRequestID string `json:"chargeRequestId"`
	}{NewRequestID(), chargeRequestID}
	if _, err := c.do(ctx, http.MethodPost, "/api/v1/refunds", nil, body, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// Balance reads a user's credits.
func (c *Client) Balance(ctx context.Context, org, user string) (*Balance, error) {
	var rs Balance
	q := url.Values{"org": {org}, "user": {user}}
	if _, err := c.do(ctx, http.MethodGet, "/api/v1/balances", q, nil, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// Ledger reads a page of the service's ledger, newest first.
func (c *Client) Ledger(ctx context.Context, rq LedgerRequest) (*LedgerPage, error) {
	q := url.Values{}
	for k, v := range map[string]string{"org": rq.Org, "user": rq.User, "cursor": rq.Cursor} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if rq.Limit > 0 {
		q.Set("limit", strconv.Itoa(rq.Limit))
	}
	var rs LedgerPage
	if _, err := c.do(ctx, http.MethodGet, "/api/v1/ledger", q, nil, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// ViewToken mints a token the user can read their own credits on topup.sh
// with.
func (c *Client) ViewToken(ctx context.Context, org, user string) (*ViewToken, error) {
	var rs ViewToken
	body := struct {
		Org  string `json:"org"`
		User string `json:"user"`
	}{org, user}
	if _, err := c.do(ctx, http.MethodPost, "/api/v1/view-tokens", nil, body, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}
