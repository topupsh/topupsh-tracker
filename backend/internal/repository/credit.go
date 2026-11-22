// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=credit.go -destination=mocks/credit.go -package=mocks

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

type (
	// CreditRepository persists org limits, credit buckets and the ledger.
	CreditRepository interface {
		// UpsertOrgLimit sets the limits of l's service and org, creating
		// the row with l.ID when there is none.
		UpsertOrgLimit(ctx context.Context, l model.OrgLimit) (model.OrgLimit, error)
		// CreateGrant stores the grant bucket b and the ledger entry e for
		// it in one transaction. e.BalanceAfter is set to the user's
		// unexpired credits at b.CreatedAt, b included.
		CreateGrant(ctx context.Context, b model.CreditBucket, e model.LedgerEntry) (model.CreditBucket, model.LedgerEntry, error)
	}

	creditRepository struct {
		db *gorm.DB
	}
)

// NewCreditRepository returns a GORM-backed credit repository.
func NewCreditRepository(db *gorm.DB) CreditRepository {
	return &creditRepository{db: db}
}

func (r *creditRepository) UpsertOrgLimit(ctx context.Context, l model.OrgLimit) (model.OrgLimit, error) {
	db := r.db.WithContext(ctx)
	// Select writes monthly_limit even when it is 0, which GORM would
	// otherwise skip for a column with a default.
	err := db.Select("*").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "service_id"}, {Name: "org_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"monthly_limit", "user_limit", "updated_at"}),
	}).Create(&l).Error
	if err != nil {
		return model.OrgLimit{}, err
	}
	var saved model.OrgLimit
	if err := db.Where("service_id = ? AND org_id = ?", l.ServiceID, l.OrgID).First(&saved).Error; err != nil {
		return model.OrgLimit{}, err
	}
	return saved, nil
}

func (r *creditRepository) CreateGrant(ctx context.Context, b model.CreditBucket, e model.LedgerEntry) (model.CreditBucket, model.LedgerEntry, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Locking the service row serializes the service's balance changes,
		// so the balance each ledger entry records follows the one before.
		var s model.Service
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
			Select("id").First(&s, b.ServiceID).Error; err != nil {
			return err
		}
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
		var balance int64
		if err := tx.Model(&model.CreditBucket{}).
			Where("service_id = ? AND org_id = ? AND user_id = ? AND expires_at > ?", b.ServiceID, b.OrgID, b.UserID, b.CreatedAt).
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
