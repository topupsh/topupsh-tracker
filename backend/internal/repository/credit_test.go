// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

var _grantAt = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func newCreditDB(t *testing.T) (*gorm.DB, CreditRepository) {
	t.Helper()
	db := newServiceDB(t)
	_, err := NewServiceRepository(db).FindOrCreateService(context.Background(), model.Service{ID: 1, Name: "mesher"})
	require.NoError(t, err)
	return db, NewCreditRepository(db)
}

// failOn makes queries against table fail with err from now on.
func failOn(t *testing.T, db *gorm.DB, table string, err error) {
	t.Helper()
	fail := func(d *gorm.DB) {
		if d.Statement.Table == table {
			_ = d.AddError(err)
		}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_query", fail))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test:fail_row", fail))
}

func grantBucket(id int64, credits int64, at time.Time) model.CreditBucket {
	return model.CreditBucket{
		ID: id, CreatedAt: at, ServiceID: 1, OrgID: "o1", UserID: "u1",
		BucketType: model.BucketTypeGrant, Amount: credits, Remaining: credits, ExpiresAt: at.AddDate(0, 1, 0),
	}
}

func grantEntry(id int64, credits int64, at time.Time) model.LedgerEntry {
	return model.LedgerEntry{
		ID: id, CreatedAt: at, ServiceID: 1, OrgID: "o1", UserID: "u1",
		EntryType: model.EntryTypeGrant, Amount: credits, ActorType: model.ActorTypeAdmin,
	}
}

func TestCreditRepository_UpsertOrgLimit(t *testing.T) {
	ctx := context.Background()
	_, repo := newCreditDB(t)
	five := int64(5)

	l, err := repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 10, ServiceID: 1, OrgID: "o1", MonthlyLimit: 1000, UserLimit: &five}, _mappingIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(10), l.ID)
	assert.Equal(t, int64(1000), l.MonthlyLimit)
	require.NotNil(t, l.UserLimit)
	assert.Equal(t, int64(5), *l.UserLimit)

	l, err = repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 11, ServiceID: 1, OrgID: "o1", MonthlyLimit: 0}, _mappingIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(10), l.ID, "the existing row is updated")
	assert.Equal(t, int64(0), l.MonthlyLimit, "an admin-set 0 replaces the old limit")
	assert.Nil(t, l.UserLimit, "users become uncapped")

	other, err := repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 12, ServiceID: 1, OrgID: "o2", MonthlyLimit: 7}, _mappingIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(12), other.ID, "another org has its own row")
}

func TestCreditRepository_CreateGrant(t *testing.T) {
	ctx := context.Background()
	db, repo := newCreditDB(t)

	// An expired bucket and another user's bucket don't count.
	others := grantBucket(2, 70, _grantAt)
	others.UserID = "u2"
	storeBuckets(t, db, grantBucket(1, 50, _grantAt.AddDate(0, -2, 0)), others)

	b, e, err := repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), grantEntry(4, 100, _grantAt), _mappingIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(3), b.ID)
	assert.Equal(t, int64(100), e.BalanceAfter)

	_, e, err = repo.CreateGrant(ctx, grantBucket(5, 20, _grantAt.Add(time.Hour)), grantEntry(6, 20, _grantAt.Add(time.Hour)), _mappingIDs)
	require.NoError(t, err)
	assert.Equal(t, int64(120), e.BalanceAfter, "the balance adds up the user's unexpired buckets")

	var stored model.LedgerEntry
	require.NoError(t, db.First(&stored, 6).Error)
	assert.Equal(t, int64(120), stored.BalanceAfter)
	assert.Equal(t, model.ActorTypeAdmin, stored.ActorType)
}

func TestCreditRepository_CreateGrant_RequestID(t *testing.T) {
	ctx := context.Background()
	db, repo := newCreditDB(t)
	g1 := "g1"
	e := grantEntry(4, 100, _grantAt)
	e.RequestID = &g1
	_, saved, err := repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), e, _mappingIDs)
	require.NoError(t, err)
	assert.Equal(t, "g1", *saved.RequestID)

	e.ID = 6
	_, _, err = repo.CreateGrant(ctx, grantBucket(5, 100, _grantAt), e, _mappingIDs)
	assert.ErrorIs(t, err, ErrRequestConflict, "a retry never grants twice")
	var n int64
	require.NoError(t, db.Model(&model.CreditBucket{}).Count(&n).Error)
	assert.Equal(t, int64(1), n)

	db, repo = newCreditDB(t)
	failNth(t, db, "query", 2) // after the service lock
	_, _, err = repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), e, _mappingIDs)
	assert.ErrorIs(t, err, errDB)
}

func TestCreditRepository_CreateGrantRollsBack(t *testing.T) {
	ctx := context.Background()
	errBoom := errors.New("boom")

	t.Run("unknown service", func(t *testing.T) {
		_, repo := newCreditDB(t)
		b := grantBucket(3, 100, _grantAt)
		b.ServiceID = 99
		_, _, err := repo.CreateGrant(ctx, b, grantEntry(4, 100, _grantAt), _mappingIDs)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
	t.Run("bucket insert fails", func(t *testing.T) {
		db, repo := newCreditDB(t)
		existing := grantBucket(3, 1, _grantAt)
		require.NoError(t, db.Create(&existing).Error)
		_, _, err := repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), grantEntry(4, 100, _grantAt), _mappingIDs)
		assert.Error(t, err)
	})
	t.Run("balance query fails", func(t *testing.T) {
		db, repo := newCreditDB(t)
		failOn(t, db, "credit_buckets", errBoom)
		_, _, err := repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), grantEntry(4, 100, _grantAt), _mappingIDs)
		assert.ErrorIs(t, err, errBoom)
	})
	t.Run("ledger insert fails", func(t *testing.T) {
		db, repo := newCreditDB(t)
		existing := grantEntry(4, 1, _grantAt)
		require.NoError(t, db.Create(&existing).Error)
		_, _, err := repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), grantEntry(4, 100, _grantAt), _mappingIDs)
		assert.Error(t, err)
		var n int64
		require.NoError(t, db.Model(&model.CreditBucket{}).Where("id = ?", 3).Count(&n).Error)
		assert.Zero(t, n, "the bucket is rolled back with the entry")
	})
}

func TestCreditRepository_DBErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("closed", func(t *testing.T) {
		db, repo := newCreditDB(t)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		_, err = repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 1, ServiceID: 1, OrgID: "o1"}, _mappingIDs)
		assert.Error(t, err)
	})
	t.Run("mapping fails", func(t *testing.T) {
		db, repo := newCreditDB(t)
		failNth(t, db, "create", 1)
		_, err := repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 1, ServiceID: 1, OrgID: "o1"}, _mappingIDs)
		assert.ErrorIs(t, err, errDB)

		db, repo = newCreditDB(t)
		failNth(t, db, "create", 1)
		_, _, err = repo.CreateGrant(ctx, grantBucket(3, 100, _grantAt), grantEntry(4, 100, _grantAt), _mappingIDs)
		assert.ErrorIs(t, err, errDB)
	})
	t.Run("upsert fails", func(t *testing.T) {
		db, repo := newCreditDB(t)
		failNth(t, db, "create", 2) // after the org's mapping
		_, err := repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 1, ServiceID: 1, OrgID: "o1"}, _mappingIDs)
		assert.ErrorIs(t, err, errDB)
	})
	t.Run("re-read fails", func(t *testing.T) {
		db, repo := newCreditDB(t)
		errBoom := errors.New("boom")
		failOn(t, db, "org_limits", errBoom)
		_, err := repo.UpsertOrgLimit(ctx, model.OrgLimit{ID: 1, ServiceID: 1, OrgID: "o1"}, _mappingIDs)
		assert.ErrorIs(t, err, errBoom)
	})
}
