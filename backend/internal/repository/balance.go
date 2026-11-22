// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=balance.go -destination=mocks/balance.go -package=mocks

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

type (
	// BalanceRepository reads balances and the ledger without changing
	// anything.
	BalanceRepository interface {
		// GetBalance returns what a user of a service's org holds at at,
		// read in one transaction. It returns ErrNotFound for an unknown
		// service.
		GetBalance(ctx context.Context, serviceID int64, orgID, userID string, at time.Time) (Balance, error)
		// ListLedgerEntries returns the entries f matches, newest first.
		ListLedgerEntries(ctx context.Context, f LedgerFilter) ([]LedgerRow, error)
	}

	// Balance is a user's credits and their org's limits at one time.
	Balance struct {
		Service model.Service
		// OrgLimit is the org's limits row, or the defaults when the
		// admin set none.
		OrgLimit model.OrgLimit
		// OrgUsed is the monthly credits the org's users spent this month.
		OrgUsed int64
		// Buckets are the user's unexpired buckets, oldest first.
		Buckets []model.CreditBucket
	}

	// LedgerFilter picks ledger entries; zero fields match everything.
	LedgerFilter struct {
		ServiceID int64
		OrgID     string
		UserID    string
		// BeforeID keeps entries older than the entry with this ID.
		BeforeID int64
		Limit    int
	}

	// LedgerRow is a ledger entry with its service's name.
	LedgerRow struct {
		model.LedgerEntry `gorm:"embedded"`
		ServiceName       string
	}

	balanceRepository struct {
		db *gorm.DB
	}
)

// NewBalanceRepository returns a GORM-backed balance repository.
func NewBalanceRepository(db *gorm.DB) BalanceRepository {
	return &balanceRepository{db: db}
}

func (r *balanceRepository) GetBalance(ctx context.Context, serviceID int64, orgID, userID string, at time.Time) (Balance, error) {
	var b Balance
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if b.Service, err = findService(tx, serviceID); err != nil {
			return err
		}
		if b.OrgLimit, err = findOrgLimit(tx, serviceID, orgID); err != nil {
			return err
		}
		if err := tx.Model(&model.CreditBucket{}).
			Where("service_id = ? AND org_id = ? AND kind = ? AND created_at = ?", serviceID, orgID, entity.BucketKindMonthly, MonthStart(at)).
			Select("COALESCE(SUM(amount - remaining), 0)").Scan(&b.OrgUsed).Error; err != nil {
			return err
		}
		return tx.Where("service_id = ? AND org_id = ? AND user_id = ? AND expires_at > ?", serviceID, orgID, userID, at).
			Order("created_at, id").Find(&b.Buckets).Error
	})
	if err != nil {
		return Balance{}, err
	}
	return b, nil
}

func (r *balanceRepository) ListLedgerEntries(ctx context.Context, f LedgerFilter) ([]LedgerRow, error) {
	q := r.db.WithContext(ctx).Model(&model.LedgerEntry{}).
		Select("ledger_entries.*, services.name AS service_name").
		Joins("JOIN services ON services.id = ledger_entries.service_id")
	if f.ServiceID != 0 {
		q = q.Where("ledger_entries.service_id = ?", f.ServiceID)
	}
	if f.OrgID != "" {
		q = q.Where("ledger_entries.org_id = ?", f.OrgID)
	}
	if f.UserID != "" {
		q = q.Where("ledger_entries.user_id = ?", f.UserID)
	}
	if f.BeforeID != 0 {
		q = q.Where("ledger_entries.id < ?", f.BeforeID)
	}
	var rows []LedgerRow
	if err := q.Order("ledger_entries.id DESC").Limit(f.Limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func findService(q *gorm.DB, id int64) (model.Service, error) {
	var s model.Service
	err := q.First(&s, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Service{}, ErrNotFound
	}
	return s, err
}

// findOrgLimit returns the org's limits, or the defaults when the admin set
// none: an org limit of entity.DefaultOrgMonthlyLimit and uncapped users.
func findOrgLimit(q *gorm.DB, serviceID int64, orgID string) (model.OrgLimit, error) {
	var ol model.OrgLimit
	err := q.Where("service_id = ? AND org_id = ?", serviceID, orgID).First(&ol).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.OrgLimit{ServiceID: serviceID, OrgID: orgID, MonthlyLimit: entity.DefaultOrgMonthlyLimit}, nil
	}
	return ol, err
}
