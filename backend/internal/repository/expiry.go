// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=expiry.go -destination=mocks/expiry.go -package=mocks

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// _expiryBatch is the most buckets one expiry transaction takes, so the
// service lock is never held for long.
const _expiryBatch = 500

type (
	// ExpiryRepository expires credit buckets.
	ExpiryRepository interface {
		// ExpireBuckets takes what is left of every bucket that expired by
		// at, monthly or grant, and writes an expiry ledger entry for each.
		// A bucket is expired once: running it again, or on several
		// replicas at once, expires nothing more.
		ExpireBuckets(ctx context.Context, at time.Time, nextID func() int64) (ExpiryResult, error)
	}

	// ExpiryResult is what an expiry run took.
	ExpiryResult struct {
		Buckets int
		Credits int64
	}

	expiryRepository struct {
		db *gorm.DB
	}
)

// NewExpiryRepository returns a GORM-backed expiry repository.
func NewExpiryRepository(db *gorm.DB) ExpiryRepository {
	return &expiryRepository{db: db}
}

func (r *expiryRepository) ExpireBuckets(ctx context.Context, at time.Time, nextID func() int64) (ExpiryResult, error) {
	db := r.db.WithContext(ctx)
	var services []int64
	if err := expired(db.Model(&model.CreditBucket{}), at).Distinct().Pluck("service_id", &services).Error; err != nil {
		return ExpiryResult{}, err
	}
	var rs ExpiryResult
	for _, id := range services {
		for {
			n, err := r.expireBatch(db, id, at, nextID, &rs)
			if err != nil {
				return rs, err
			}
			if n < _expiryBatch {
				break
			}
		}
	}
	return rs, nil
}

// expireBatch expires up to _expiryBatch of the service's expired buckets
// in one transaction and returns how many it found.
func (r *expiryRepository) expireBatch(db *gorm.DB, serviceID int64, at time.Time, nextID func() int64, rs *ExpiryResult) (int, error) {
	var buckets []model.CreditBucket
	var credits int64
	err := db.Transaction(func(tx *gorm.DB) error {
		// The service lock is what charges and grants take too: after it,
		// the buckets read here can't change under the transaction, and a
		// concurrent run waits, then finds them expired.
		if _, err := lockService(tx, serviceID); err != nil {
			return err
		}
		if err := expired(tx.Where("service_id = ?", serviceID), at).
			Order("org_mapping_id, user_mapping_id, created_at, id").Limit(_expiryBatch).Find(&buckets).Error; err != nil {
			return err
		}
		var balance int64
		for i, b := range buckets {
			if i == 0 || b.OrgMappingID != buckets[i-1].OrgMappingID || b.UserMappingID != buckets[i-1].UserMappingID {
				// Expired buckets keep their credits until expired here,
				// so the sum over all of the user's buckets is the balance
				// before this run's expiries.
				if err := tx.Model(&model.CreditBucket{}).
					Where("service_id = ? AND org_mapping_id = ? AND user_mapping_id = ?", serviceID, b.OrgMappingID, b.UserMappingID).
					Select("COALESCE(SUM(remaining), 0)").Scan(&balance).Error; err != nil {
					return err
				}
			}
			left := b.Remaining
			if err := tx.Model(&b).Update("remaining", 0).Error; err != nil {
				return err
			}
			balance -= left
			credits += left
			if err := tx.Create(&model.LedgerEntry{
				ID: nextID(), CreatedAt: at, ServiceID: serviceID, OrgMappingID: b.OrgMappingID, UserMappingID: b.UserMappingID,
				EntryType: model.EntryTypeExpiry, Amount: -left, BalanceAfter: balance, ActorType: model.ActorTypeScheduler,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	rs.Buckets += len(buckets)
	rs.Credits += credits
	return len(buckets), nil
}

// expired narrows q to the buckets that expired by at and still hold
// credits.
func expired(q *gorm.DB, at time.Time) *gorm.DB {
	return q.Where("expires_at <= ? AND remaining > 0", at)
}
