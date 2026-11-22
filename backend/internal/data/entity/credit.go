// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

import "time"

// DefaultOrgMonthlyLimit is a new service's default org limit, the monthly
// org limit of orgs the admin set none for: nothing can be spent until an
// admin sets a limit.
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
	// LedgerKindLimit resizes a user's monthly bucket after an admin
	// changed the org's limits.
	LedgerKindLimit  LedgerKind = "limit"
	LedgerKindExpiry LedgerKind = "expiry"
)

// Ledger entry actors.
const (
	ActorKindService   ActorKind = "service"
	ActorKindAdmin     ActorKind = "admin"
	ActorKindScheduler ActorKind = "scheduler"
)

// Charge refusals.
const (
	RefusalOrgLimit  Refusal = "org_limit"
	RefusalUserLimit Refusal = "user_limit"
)

type (
	// BucketKind says where a bucket's credits came from.
	BucketKind string

	// LedgerKind says what changed a balance.
	LedgerKind string

	// ActorKind says who or what changed a balance.
	ActorKind string

	// Refusal says which limit refused a charge; empty means charged.
	Refusal string

	// Service is a product that charges credits, e.g. mesher or tmphost.
	Service struct {
		ID        int64
		Name      string
		Enabled   bool
		CreatedAt time.Time
		// DefaultOrgLimit is the monthly org limit of orgs the admin set
		// no limits for.
		DefaultOrgLimit int64
		// DefaultUserLimit is the user limit of those orgs; nil means
		// uncapped.
		DefaultUserLimit *int64
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

	// Charge is one spend a service asked for and its outcome.
	Charge struct {
		ID        int64
		ServiceID int64
		RequestID string
		OrgID     string
		UserID    string
		// Credits is what was spent; 0 when refused.
		Credits   int64
		Operation string
		Refusal   Refusal
		// Balance is the user's unexpired credits after the charge.
		Balance int64
		// OrgRemaining is what was left of the org limit after the charge.
		OrgRemaining int64
		CreatedAt    time.Time
	}

	// Refund gives a charge's credits back.
	Refund struct {
		ID        int64
		ServiceID int64
		RequestID string
		// ChargeRequestID is the request ID of the refunded charge.
		ChargeRequestID string
		OrgID           string
		UserID          string
		// Credits is what was given back; credits of expired buckets are
		// not.
		Credits   int64
		Operation string
		// Balance is the user's unexpired credits after the refund.
		Balance int64
		// OrgRemaining is what was left of the org limit after the refund.
		OrgRemaining int64
		CreatedAt    time.Time
	}

	// LedgerEntry is one change to a user's balance.
	LedgerEntry struct {
		ID        int64
		ServiceID int64
		// ServiceName is set where entries of several services are read.
		ServiceName string
		// RequestID is the request that made the entry; empty on entries
		// the system makes.
		RequestID    string
		OrgID        string
		UserID       string
		Kind         LedgerKind
		Amount       int64
		BalanceAfter int64
		Actor        ActorKind
		Operation    string
		CreatedAt    time.Time
	}
)
