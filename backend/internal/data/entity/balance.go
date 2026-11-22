// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

import "time"

type (
	// Viewer is one user of a service, reading their own credits with a
	// view token the service issued.
	Viewer struct {
		ServiceID int64
		OrgID     string
		UserID    string
	}

	// Reader is who reads credits: a service with its key (ServiceID),
	// the root admin, or a viewer. Exactly one is set.
	Reader struct {
		ServiceID int64
		Root      bool
		Viewer    *Viewer
	}

	// MonthlyCredits is a user's monthly allowance this month.
	MonthlyCredits struct {
		Limit     int64
		Used      int64
		Remaining int64
	}

	// OrgCredits is an org's monthly limits and usage this month.
	OrgCredits struct {
		Limit int64
		// UserLimit is nil when users are uncapped.
		UserLimit *int64
		Used      int64
		Remaining int64
	}

	// Balance is what one user of a service holds.
	Balance struct {
		Service Service
		OrgID   string
		UserID  string
		// Credits is the user's unexpired credits, as the ledger's
		// balances count them.
		Credits int64
		// Available is what a charge can spend now: the grants, and the
		// monthly allowance as far as the org limit allows.
		Available int64
		Monthly   MonthlyCredits
		Org       OrgCredits
		// Grants are the user's unexpired grants, oldest first.
		Grants   []CreditBucket
		RenewsAt time.Time
	}

	// GetBalanceRequest asks for one user's balance. A viewer reads their
	// own and sets no other field; the root admin names the service.
	GetBalanceRequest struct {
		Reader      Reader
		ServiceName string
		OrgID       string
		UserID      string
	}

	// GetBalanceResponse carries the balance.
	GetBalanceResponse struct {
		Balance Balance
	}

	// ListLedgerEntriesRequest asks for a page of the ledger, newest
	// first. Empty filters match everything the reader may see.
	ListLedgerEntriesRequest struct {
		Reader      Reader
		ServiceName string
		OrgID       string
		UserID      string
		// Cursor is the NextCursor of the previous page; 0 starts at the
		// newest entry.
		Cursor int64
		// Limit is the page size; 0 means the default.
		Limit int
	}

	// ListLedgerEntriesResponse is a page of the ledger.
	ListLedgerEntriesResponse struct {
		Entries []LedgerEntry
		// NextCursor reads the next page; 0 means this is the last one.
		NextCursor int64
	}

	// CreateViewTokenRequest asks a service for a token that lets one of
	// its users read their own credits.
	CreateViewTokenRequest struct {
		ServiceID int64
		OrgID     string
		UserID    string
	}

	// CreateViewTokenResponse carries the token.
	CreateViewTokenResponse struct {
		Token     string
		ExpiresAt time.Time
		// URL opens the user's balance page with the token; empty when
		// the app has no base URL.
		URL string
	}

	// AuthenticateViewerRequest carries a view token.
	AuthenticateViewerRequest struct {
		Token string
	}

	// AuthenticateViewerResponse is the user the token was issued for.
	AuthenticateViewerResponse struct {
		Viewer Viewer
	}
)
