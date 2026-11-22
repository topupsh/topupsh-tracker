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

	// LedgerRow is a ledger entry with its service's name, and its OrgID
	// and UserID set.
	LedgerRow struct {
		model.LedgerEntry `gorm:"embedded"`
		ServiceName       string
		// ExternalOrgID, ExternalUserID and ExternalOperation are read
		// from the entry's mappings and copied to LedgerEntry.
		ExternalOrgID     string
		ExternalUserID    string
		ExternalOperation string
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
		if b.OrgLimit, err = findOrgLimit(tx, b.Service, orgID); err != nil {
			return err
		}
		if err := tx.Model(&model.CreditBucket{}).
			Where("service_id = ? AND org_mapping_id = ? AND bucket_type = ? AND created_at = ?", serviceID, OrgMappingID(serviceID, orgID), model.BucketTypeMonthly, MonthStart(at)).
			Select("COALESCE(SUM(amount - remaining), 0)").Scan(&b.OrgUsed).Error; err != nil {
			return err
		}
		return tx.Where("service_id = ? AND org_mapping_id = ? AND user_mapping_id = ? AND expires_at > ?",
			serviceID, OrgMappingID(serviceID, orgID), UserMappingID(serviceID, orgID, userID), at).
			Order("created_at, id").Find(&b.Buckets).Error
	})
	if err != nil {
		return Balance{}, err
	}
	for i := range b.Buckets {
		b.Buckets[i].OrgID, b.Buckets[i].UserID = orgID, userID
	}
	return b, nil
}

func (r *balanceRepository) ListLedgerEntries(ctx context.Context, f LedgerFilter) ([]LedgerRow, error) {
	q := r.db.WithContext(ctx).Model(&model.LedgerEntry{}).
		Select(`ledger_entries.*, services.name AS service_name, m.org_id AS external_org_id, m.user_id AS external_user_id,
			COALESCE(o.operation, '') AS external_operation`).
		Joins("JOIN services ON services.id = ledger_entries.service_id").
		Joins("JOIN external_id_mappings m ON m.mapping_id = ledger_entries.user_mapping_id AND m.service_id = ledger_entries.service_id AND m.user_id <> ''").
		Joins("LEFT JOIN operation_mappings o ON o.service_id = ledger_entries.service_id AND o.mapping_id = ledger_entries.operation_mapping_id").
		// A refused charge changed no balance.
		Where("ledger_entries.entry_type <> ?", model.EntryTypeRefusal)
	switch {
	case f.ServiceID != 0:
		// The mapping IDs are known: use the entries' index.
		q = q.Where("ledger_entries.service_id = ?", f.ServiceID)
		if f.UserID != "" {
			q = q.Where("ledger_entries.org_mapping_id = ? AND ledger_entries.user_mapping_id = ?",
				OrgMappingID(f.ServiceID, f.OrgID), UserMappingID(f.ServiceID, f.OrgID, f.UserID))
		} else if f.OrgID != "" {
			q = q.Where("ledger_entries.org_mapping_id = ?", OrgMappingID(f.ServiceID, f.OrgID))
		}
	case f.OrgID != "":
		// Across services an org has a mapping ID per service.
		q = q.Where("m.org_id = ?", f.OrgID)
		if f.UserID != "" {
			q = q.Where("m.user_id = ?", f.UserID)
		}
	}
	if f.BeforeID != 0 {
		q = q.Where("ledger_entries.id < ?", f.BeforeID)
	}
	var rows []LedgerRow
	if err := q.Order("ledger_entries.id DESC").Limit(f.Limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].OrgID, rows[i].UserID, rows[i].Operation = rows[i].ExternalOrgID, rows[i].ExternalUserID, rows[i].ExternalOperation
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

// findOrgLimit returns the org's limits, or its service's default limits
// when the admin set none.
func findOrgLimit(q *gorm.DB, s model.Service, orgID string) (model.OrgLimit, error) {
	var ol model.OrgLimit
	err := q.Where("service_id = ? AND org_mapping_id = ?", s.ID, OrgMappingID(s.ID, orgID)).First(&ol).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultOrgLimit(s, orgID), nil
	}
	ol.OrgID = orgID
	return ol, err
}

// defaultOrgLimit is the limits of an org of s the admin set none for.
func defaultOrgLimit(s model.Service, orgID string) model.OrgLimit {
	return model.OrgLimit{
		ServiceID: s.ID, OrgMappingID: OrgMappingID(s.ID, orgID), OrgID: orgID,
		MonthlyLimit: s.DefaultOrgLimit, UserLimit: s.DefaultUserLimit,
	}
}
