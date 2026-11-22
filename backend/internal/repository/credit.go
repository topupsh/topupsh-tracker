// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=credit.go -destination=mocks/credit.go -package=mocks

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

type (
	// CreditRepository persists org limits, credit buckets and the ledger.
	CreditRepository interface {
		// UpsertOrgLimit sets the limits of l's service and org, creating
		// the row with l.ID when there is none. nextID gives the ID of the
		// org's mapping when it is new.
		UpsertOrgLimit(ctx context.Context, l model.OrgLimit, nextID func() int64) (model.OrgLimit, error)
		// CreateGrant stores the grant bucket b and the ledger entry e for
		// it in one transaction. e.BalanceAfter is set to the user's
		// unexpired credits at b.CreatedAt, b included. nextID gives the
		// IDs of new org and user mappings. It returns ErrRequestConflict
		// when e.RequestID was used before, so a retry never grants twice.
		CreateGrant(ctx context.Context, b model.CreditBucket, e model.LedgerEntry, nextID func() int64) (model.CreditBucket, model.LedgerEntry, error)
	}

	creditRepository struct {
		db *gorm.DB
	}
)

// NewCreditRepository returns a GORM-backed credit repository.
func NewCreditRepository(db *gorm.DB) CreditRepository {
	return &creditRepository{db: db}
}

func (r *creditRepository) UpsertOrgLimit(ctx context.Context, l model.OrgLimit, nextID func() int64) (model.OrgLimit, error) {
	var saved model.OrgLimit
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if l.OrgMappingID, err = mapOrg(tx, nextID, l.ServiceID, l.OrgID); err != nil {
			return err
		}
		// Select writes monthly_limit even when it is 0, which GORM would
		// otherwise skip for a column with a default.
		if err := tx.Select("*").Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "service_id"}, {Name: "org_mapping_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"monthly_limit", "user_limit", "updated_at"}),
		}).Create(&l).Error; err != nil {
			return err
		}
		return tx.Where("service_id = ? AND org_mapping_id = ?", l.ServiceID, l.OrgMappingID).First(&saved).Error
	})
	if err != nil {
		return model.OrgLimit{}, err
	}
	saved.OrgID = l.OrgID
	return saved, nil
}

func (r *creditRepository) CreateGrant(ctx context.Context, b model.CreditBucket, e model.LedgerEntry, nextID func() int64) (model.CreditBucket, model.LedgerEntry, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Locking the service row serializes the service's balance changes,
		// so the balance each ledger entry records follows the one before.
		var s model.Service
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
			Select("id").First(&s, b.ServiceID).Error; err != nil {
			return err
		}
		var err error
		if e.RequestID != nil {
			_, err = findRequest(tx, e.ServiceID, *e.RequestID)
			if err == nil {
				return ErrRequestConflict
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if b.OrgMappingID, b.UserMappingID, err = mapUser(tx, nextID, b.ServiceID, b.OrgID, b.UserID); err != nil {
			return err
		}
		e.OrgMappingID, e.UserMappingID = b.OrgMappingID, b.UserMappingID
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
		var balance int64
		if err := tx.Model(&model.CreditBucket{}).
			Where("service_id = ? AND org_mapping_id = ? AND user_mapping_id = ? AND expires_at > ?", b.ServiceID, b.OrgMappingID, b.UserMappingID, b.CreatedAt).
			Select("COALESCE(SUM(remaining), 0)").Scan(&balance).Error; err != nil {
			return err
		}
		e.BalanceAfter = balance
		return tx.Create(&e).Error
	})
	if err != nil {
		return model.CreditBucket{}, model.LedgerEntry{}, err
	}
	return b, e, nil
}
