// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrLedgerAppendOnly is returned when a ledger entry is updated or deleted.
var ErrLedgerAppendOnly = errors.New("ledger is append-only")

type (
	// OrgLimit holds a service's monthly limits for one org. Orgs and users
	// are the calling service's own identifiers.
	OrgLimit struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time
		UpdatedAt time.Time
		ServiceID int64  `gorm:"uniqueIndex:idx_org_limits_service_org;not null"`
		OrgID     string `gorm:"type:varchar(64);uniqueIndex:idx_org_limits_service_org;not null"`
		// MonthlyLimit is the credits the whole org may use in a month. It
		// defaults to 0, as does an org without a row
		// (entity.DefaultOrgMonthlyLimit).
		MonthlyLimit int64 `gorm:"not null;default:0"`
		// UserLimit is the credits each user of the org may use in a month;
		// nil means uncapped.
		UserLimit *int64
	}

	// CreditBucket is a lot of credits a user can spend, either the monthly
	// allowance or an admin grant. Buckets are spent FIFO by CreatedAt.
	CreditBucket struct {
		ID        int64     `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time `gorm:"index:idx_credit_buckets_fifo,priority:4;not null"`
		ServiceID int64     `gorm:"index:idx_credit_buckets_fifo,priority:1;not null"`
		OrgID     string    `gorm:"type:varchar(64);index:idx_credit_buckets_fifo,priority:2;not null"`
		UserID    string    `gorm:"type:varchar(64);index:idx_credit_buckets_fifo,priority:3;not null"`
		// Kind is "monthly" or "grant".
		Kind      string    `gorm:"type:varchar(16);not null"`
		Amount    int64     `gorm:"not null"`
		Remaining int64     `gorm:"not null;check:chk_credit_buckets_remaining,remaining >= 0"`
		ExpiresAt time.Time `gorm:"index;not null"`
	}

	// Charge is one spend a service asked for. RequestID is unique per
	// service, so a retried request never charges twice.
	Charge struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time
		ServiceID int64  `gorm:"uniqueIndex:idx_charges_service_request;not null"`
		RequestID string `gorm:"type:varchar(128);uniqueIndex:idx_charges_service_request;not null"`
		OrgID     string `gorm:"type:varchar(64);not null"`
		UserID    string `gorm:"type:varchar(64);not null"`
		Credits   int64  `gorm:"not null"`
		Operation string `gorm:"type:varchar(255)"`
		// RefundedAt is set once the charge is refunded.
		RefundedAt *time.Time
	}

	// LedgerEntry is one change to a user's balance. Entries are written in
	// the same transaction as the change and are never updated or deleted.
	LedgerEntry struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false;index:idx_ledger_entries_subject,priority:4"`
		CreatedAt time.Time
		ServiceID int64  `gorm:"index:idx_ledger_entries_subject,priority:1;not null"`
		OrgID     string `gorm:"type:varchar(64);index:idx_ledger_entries_subject,priority:2;not null"`
		UserID    string `gorm:"type:varchar(64);index:idx_ledger_entries_subject,priority:3;not null"`
		// Kind is "spend", "refund", "grant", "renewal" or "expiry".
		Kind string `gorm:"type:varchar(16);not null"`
		// Amount is signed: negative takes credits away.
		Amount       int64 `gorm:"not null"`
		BalanceAfter int64 `gorm:"not null"`
		// Actor is who or what made the change, e.g. a service, an admin or
		// the renewal job.
		Actor     string `gorm:"type:varchar(255);not null"`
		Operation string `gorm:"type:varchar(255)"`
	}
)

// BeforeUpdate keeps the ledger append-only.
func (LedgerEntry) BeforeUpdate(*gorm.DB) error { return ErrLedgerAppendOnly }

// BeforeDelete keeps the ledger append-only.
func (LedgerEntry) BeforeDelete(*gorm.DB) error { return ErrLedgerAppendOnly }
