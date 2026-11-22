// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

import "time"

// DefaultOrgMonthlyLimit is the monthly org limit of an org the admin has
// set none for: nothing can be spent until an admin sets a limit.
const DefaultOrgMonthlyLimit int64 = 0

// Bucket kinds.
const (
	BucketKindMonthly BucketKind = "monthly"
	BucketKindGrant   BucketKind = "grant"
)

// Ledger entry kinds.
const (
	LedgerKindSpend   LedgerKind = "spend"
	LedgerKindRefund  LedgerKind = "refund"
	LedgerKindGrant   LedgerKind = "grant"
	LedgerKindRenewal LedgerKind = "renewal"
	LedgerKindExpiry  LedgerKind = "expiry"
)

type (
	// BucketKind says where a bucket's credits came from.
	BucketKind string

	// LedgerKind says what changed a balance.
	LedgerKind string

	// Service is a product that charges credits, e.g. mesher or tmphost.
	Service struct {
		ID        int64
		Name      string
		Enabled   bool
		CreatedAt time.Time
	}

	// ServiceAPIKey is a service's API key; only its hash is kept.
	ServiceAPIKey struct {
		ID        int64
		ServiceID int64
		Prefix    string
		KeyHash   string
		CreatedAt time.Time
		RevokedAt *time.Time
	}

	// OrgLimit holds a service's monthly limits for one org.
	OrgLimit struct {
		ID           int64
		ServiceID    int64
		OrgID        string
		MonthlyLimit int64
		// UserLimit is nil when users are uncapped.
		UserLimit *int64
	}

	// CreditBucket is a lot of credits a user can spend, oldest first.
	CreditBucket struct {
		ID        int64
		ServiceID int64
		OrgID     string
		UserID    string
		Kind      BucketKind
		Amount    int64
		Remaining int64
		CreatedAt time.Time
		ExpiresAt time.Time
	}

	// Charge is one spend a service asked for.
	Charge struct {
		ID         int64
		ServiceID  int64
		RequestID  string
		OrgID      string
		UserID     string
		Credits    int64
		Operation  string
		CreatedAt  time.Time
		RefundedAt *time.Time
	}

	// LedgerEntry is one change to a user's balance.
	LedgerEntry struct {
		ID           int64
		ServiceID    int64
		OrgID        string
		UserID       string
		Kind         LedgerKind
		Amount       int64
		BalanceAfter int64
		Actor        string
		Operation    string
		CreatedAt    time.Time
	}
)
