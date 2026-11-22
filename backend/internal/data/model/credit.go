// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrLedgerAppendOnly is returned when a ledger entry is updated or
	// deleted.
	ErrLedgerAppendOnly = errors.New("ledger is append-only")
	// ErrMappingImmutable is returned when an external ID or operation
	// mapping is updated or deleted.
	ErrMappingImmutable = errors.New("mappings are immutable")
)

// Bucket types.
const (
	BucketTypeMonthly BucketType = iota + 1
	BucketTypeGrant
)

// Ledger entry types.
const (
	EntryTypeSpend EntryType = iota + 1
	EntryTypeRefund
	EntryTypeGrant
	EntryTypeRenewal
	EntryTypeLimit
	EntryTypeExpiry
	// EntryTypeRefusal records a refused charge; it changes no balance.
	EntryTypeRefusal
)

// Ledger entry actors.
const (
	// ActorTypeService is the entry's service, through its API key.
	ActorTypeService ActorType = iota + 1
	// ActorTypeAdmin is the root admin.
	ActorTypeAdmin
	// ActorTypeScheduler is the scheduled credit jobs.
	ActorTypeScheduler
)

// Refusal reasons of a refused charge.
const (
	RefusalReasonOrgLimit RefusalReason = iota + 1
	RefusalReasonUserLimit
)

type (
	// BucketType is where a bucket's credits came from, stored as a
	// smallint; 0 is no type.
	BucketType uint8

	// EntryType is what changed a balance, stored as a smallint; 0 is no
	// type.
	EntryType uint8

	// ActorType is who or what made a ledger entry, stored as a smallint;
	// 0 is no actor.
	ActorType uint8

	// RefusalReason is which limit refused a charge, stored as a smallint;
	// 0 is not refused.
	RefusalReason uint8

	// ExternalIDMapping maps a calling service's own org, or org and user,
	// identifiers to the int64 MappingID the other tables store. An org's
	// row has an empty UserID. Rows are never changed.
	ExternalIDMapping struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time
		ServiceID int64  `gorm:"uniqueIndex:idx_external_id_mappings_subject;uniqueIndex:idx_external_id_mappings_service_mapping;not null"`
		OrgID     string `gorm:"type:varchar(64);uniqueIndex:idx_external_id_mappings_subject;not null"`
		UserID    string `gorm:"type:varchar(64);uniqueIndex:idx_external_id_mappings_subject;not null"`
		// MappingID is the xxhash64 of the service's base62 ID, the org and,
		// for a user, the user. It is unique per service: an org or user
		// that hashes like a stored one shares its mapping.
		MappingID int64 `gorm:"uniqueIndex:idx_external_id_mappings_service_mapping;not null"`
	}

	// OperationMapping maps an operation a calling service charges for to
	// the int64 MappingID the ledger stores. Rows are never
	// changed.
	OperationMapping struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time
		ServiceID int64  `gorm:"uniqueIndex:idx_operation_mappings_service_mapping;not null"`
		Operation string `gorm:"type:varchar(128);not null"`
		// MappingID is the xxhash64 of the service's base62 ID and the
		// operation.
		MappingID int64 `gorm:"uniqueIndex:idx_operation_mappings_service_mapping;not null"`
	}

	// OrgLimit holds a service's monthly limits for one org.
	OrgLimit struct {
		ID           int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt    time.Time
		UpdatedAt    time.Time
		ServiceID    int64 `gorm:"uniqueIndex:idx_org_limits_service_org;not null"`
		OrgMappingID int64 `gorm:"uniqueIndex:idx_org_limits_service_org;not null"`
		// OrgID is the service's own org ID, which OrgMappingID maps; it
		// is not stored here.
		OrgID string `gorm:"-"`
		// MonthlyLimit is the credits the whole org may use in a month. It
		// defaults to 0; an org without a row gets its service's
		// DefaultOrgLimit.
		MonthlyLimit int64 `gorm:"not null;default:0"`
		// UserLimit is the credits each user of the org may use in a month;
		// nil means uncapped.
		UserLimit *int64
	}

	// CreditBucket is a lot of credits a user can spend, either the monthly
	// allowance or an admin grant. Buckets are spent FIFO by CreatedAt.
	CreditBucket struct {
		ID            int64     `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt     time.Time `gorm:"index:idx_credit_buckets_fifo,priority:4;not null"`
		ServiceID     int64     `gorm:"index:idx_credit_buckets_fifo,priority:1;not null"`
		OrgMappingID  int64     `gorm:"index:idx_credit_buckets_fifo,priority:2;not null"`
		UserMappingID int64     `gorm:"index:idx_credit_buckets_fifo,priority:3;not null"`
		// OrgID and UserID are the service's own IDs, which the mapping IDs
		// map; they are not stored here.
		OrgID      string     `gorm:"-"`
		UserID     string     `gorm:"-"`
		BucketType BucketType `gorm:"type:smallint;not null"`
		Amount     int64      `gorm:"not null"`
		Remaining  int64      `gorm:"not null;check:chk_credit_buckets_remaining,remaining >= 0"`
		ExpiresAt  time.Time  `gorm:"index;not null"`
	}

	// ChargeSpend is the credits a spend entry took from one bucket, so a
	// refund can return them to the same bucket.
	ChargeSpend struct {
		ID       int64 `gorm:"primaryKey;autoIncrement:false"`
		EntryID  int64 `gorm:"index;not null"`
		BucketID int64 `gorm:"not null"`
		Credits  int64 `gorm:"not null"`
	}

	// LedgerEntry is one change to a user's balance, or a refused charge.
	// Entries are written in the same transaction as the change and are
	// never updated or deleted. A charge is a spend or refusal entry and
	// its refund a refund entry of its own; each request a service sends
	// has its own RequestID, unique per service, so a retry never charges,
	// refunds or grants twice.
	LedgerEntry struct {
		ID            int64 `gorm:"primaryKey;autoIncrement:false;index:idx_ledger_entries_subject,priority:4"`
		CreatedAt     time.Time
		ServiceID     int64 `gorm:"index:idx_ledger_entries_subject,priority:1;uniqueIndex:idx_ledger_entries_service_request;not null"`
		OrgMappingID  int64 `gorm:"index:idx_ledger_entries_subject,priority:2;not null"`
		UserMappingID int64 `gorm:"index:idx_ledger_entries_subject,priority:3;not null"`
		// OrgID and UserID are the service's own IDs, which the mapping IDs
		// map; they are not stored here.
		OrgID     string    `gorm:"-"`
		UserID    string    `gorm:"-"`
		EntryType EntryType `gorm:"type:smallint;not null"`
		// RequestID is the request that made the entry: a charge, refund or
		// grant. It is nil on entries the system makes (renewal, limit,
		// expiry).
		RequestID *string `gorm:"type:varchar(32);uniqueIndex:idx_ledger_entries_service_request"`
		// RefundOf is the ID of the spend entry a refund entry gives back;
		// a spend is refunded once.
		RefundOf *int64 `gorm:"uniqueIndex:idx_ledger_entries_refund_of"`
		// Amount is signed: negative takes credits away. A refund entry's
		// is what it gave back: credits whose bucket had expired are not
		// returned, so it can be 0. A refusal entry's is 0.
		Amount       int64 `gorm:"not null"`
		BalanceAfter int64 `gorm:"not null"`
		// RefusalReason is set on refusal entries.
		RefusalReason RefusalReason `gorm:"type:smallint;not null;default:0"`
		// OrgRemaining is what was left of the org limit after a spend,
		// refusal or refund entry; 0 on the others.
		OrgRemaining int64 `gorm:"not null;default:0"`
		// ActorType is who or what made the change: the service, the admin
		// or the scheduler.
		ActorType ActorType `gorm:"type:smallint;not null"`
		// OperationMappingID maps the operation; 0 when there is none.
		OperationMappingID int64 `gorm:"not null;default:0"`
		// Operation is the service's operation, which OperationMappingID
		// maps; it is not stored here.
		Operation string `gorm:"-"`
	}
)

// BeforeUpdate keeps mappings unchanged.
func (ExternalIDMapping) BeforeUpdate(*gorm.DB) error { return ErrMappingImmutable }

// BeforeDelete keeps mappings unchanged.
func (ExternalIDMapping) BeforeDelete(*gorm.DB) error { return ErrMappingImmutable }

// BeforeUpdate keeps mappings unchanged.
func (OperationMapping) BeforeUpdate(*gorm.DB) error { return ErrMappingImmutable }

// BeforeDelete keeps mappings unchanged.
func (OperationMapping) BeforeDelete(*gorm.DB) error { return ErrMappingImmutable }

// BeforeUpdate keeps the ledger append-only.
func (LedgerEntry) BeforeUpdate(*gorm.DB) error { return ErrLedgerAppendOnly }

// BeforeDelete keeps the ledger append-only.
func (LedgerEntry) BeforeDelete(*gorm.DB) error { return ErrLedgerAppendOnly }
