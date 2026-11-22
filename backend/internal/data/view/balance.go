// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// Balance is one user's credits in one service.
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
		Grants    []BalanceGrant `json:"grants"`
		// RenewsAt is when the monthly limits renew.
		RenewsAt string `json:"renewsAt"`
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
		// UserLimit is null while users are uncapped.
		UserLimit *int64 `json:"userLimit"`
		Used      int64  `json:"used"`
		Remaining int64  `json:"remaining"`
	}

	// BalanceGrant is an unexpired grant of a balance.
	BalanceGrant struct {
		ID        string `json:"id"`
		Credits   int64  `json:"credits"`
		Remaining int64  `json:"remaining"`
		CreatedAt string `json:"createdAt"`
		ExpiresAt string `json:"expiresAt"`
	}

	// LedgerEntry is one change to a user's balance.
	LedgerEntry struct {
		ID           string `json:"id"`
		Service      string `json:"service"`
		Org          string `json:"org"`
		User         string `json:"user"`
		Kind         string `json:"kind"`
		Amount       int64  `json:"amount"`
		BalanceAfter int64  `json:"balanceAfter"`
		Actor        string `json:"actor"`
		Operation    string `json:"operation,omitempty"`
		CreatedAt    string `json:"createdAt"`
	}

	// LedgerPage is a page of the ledger, newest first.
	LedgerPage struct {
		Entries []LedgerEntry `json:"entries"`
		// NextCursor reads the next page; it is left out on the last one.
		NextCursor string `json:"nextCursor,omitempty"`
	}

	// CreateViewTokenRequest is the body of POST /api/v1/view-tokens.
	CreateViewTokenRequest struct {
		Org  string `json:"org"`
		User string `json:"user"`
	}

	// ViewToken lets one user read their own credits until it expires.
	ViewToken struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
		// URL opens the user's balance page with the token.
		URL string `json:"url,omitempty"`
	}
)
