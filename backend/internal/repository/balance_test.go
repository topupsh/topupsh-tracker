// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

var _readAt = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func newBalanceDB(t *testing.T) (*gorm.DB, BalanceRepository) {
	t.Helper()
	db, _ := newCreditDB(t)
	_, err := NewServiceRepository(db).FindOrCreateService(context.Background(), model.Service{ID: 2, Name: "tmphost"})
	require.NoError(t, err)
	return db, NewBalanceRepository(db)
}

func bucket(id int64, org, user, kind string, amount, remaining int64, createdAt, expiresAt time.Time) model.CreditBucket {
	return model.CreditBucket{
		ID: id, CreatedAt: createdAt, ServiceID: 1, OrgID: org, UserID: user,
		BucketType: _bucketTypes[kind], Amount: amount, Remaining: remaining, ExpiresAt: expiresAt,
	}
}

func TestBalanceRepository_GetBalance(t *testing.T) {
	ctx := context.Background()
	month := MonthStart(_readAt)
	next := month.AddDate(0, 1, 0)

	t.Run("defaults", func(t *testing.T) {
		_, repo := newBalanceDB(t)
		b, err := repo.GetBalance(ctx, 1, "o1", "u1", _readAt)
		require.NoError(t, err)
		assert.Equal(t, "mesher", b.Service.Name)
		assert.Equal(t, model.OrgLimit{ServiceID: 1, OrgMappingID: OrgMappingID(1, "o1"), OrgID: "o1", MonthlyLimit: 0}, b.OrgLimit)
		assert.Zero(t, b.OrgUsed)
		assert.Empty(t, b.Buckets)
	})

	t.Run("service defaults", func(t *testing.T) {
		db, repo := newBalanceDB(t)
		five := int64(5)
		_, err := NewServiceRepository(db).SetServiceDefaults(ctx, "mesher", 100, &five)
		require.NoError(t, err)
		b, err := repo.GetBalance(ctx, 1, "o1", "u1", _readAt)
		require.NoError(t, err)
		assert.Equal(t, model.OrgLimit{ServiceID: 1, OrgMappingID: OrgMappingID(1, "o1"), OrgID: "o1", MonthlyLimit: 100, UserLimit: &five}, b.OrgLimit)
	})

	t.Run("buckets and usage", func(t *testing.T) {
		db, repo := newBalanceDB(t)
		five := int64(5)
		_, err := NewCreditRepository(db).UpsertOrgLimit(ctx, model.OrgLimit{ID: 9, ServiceID: 1, OrgID: "o1", MonthlyLimit: 100, UserLimit: &five}, _mappingIDs)
		require.NoError(t, err)
		buckets := []model.CreditBucket{
			bucket(10, "o1", "u1", "monthly", 5, 2, month, next),
			bucket(11, "o1", "u1", "grant", 7, 7, _readAt.Add(-time.Hour), _readAt.AddDate(0, 1, 0)),
			bucket(12, "o1", "u1", "grant", 3, 3, _readAt.AddDate(0, -1, -1), _readAt.Add(-time.Second)),                                         // expired
			bucket(13, "o1", "u2", "monthly", 5, 1, month, next),                                                                                 // another user of the org
			bucket(14, "o1", "u1", "monthly", 5, 0, month.AddDate(0, -1, 0), month),                                                              // last month
			bucket(15, "o2", "u1", "monthly", 5, 0, month, next),                                                                                 // another org
			{ID: 16, CreatedAt: month, ServiceID: 2, OrgID: "o1", UserID: "u1", BucketType: model.BucketTypeMonthly, Amount: 9, ExpiresAt: next}, // another service
		}
		storeBuckets(t, db, buckets...)

		b, err := repo.GetBalance(ctx, 1, "o1", "u1", _readAt)
		require.NoError(t, err)
		assert.Equal(t, int64(100), b.OrgLimit.MonthlyLimit)
		require.NotNil(t, b.OrgLimit.UserLimit)
		assert.Equal(t, int64(7), b.OrgUsed, "u1's 3 and u2's 4 this month")
		require.Len(t, b.Buckets, 2)
		assert.Equal(t, int64(10), b.Buckets[0].ID, "oldest first")
		assert.Equal(t, int64(11), b.Buckets[1].ID)
	})

	t.Run("unknown service", func(t *testing.T) {
		_, repo := newBalanceDB(t)
		_, err := repo.GetBalance(ctx, 99, "o1", "u1", _readAt)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("db errors", func(t *testing.T) {
		for _, step := range []struct {
			kind string
			nth  int
		}{{"query", 1}, {"query", 2}, {"query", 3}, {"row", 1}} {
			db, repo := newBalanceDB(t)
			failNth(t, db, step.kind, step.nth)
			_, err := repo.GetBalance(ctx, 1, "o1", "u1", _readAt)
			assert.ErrorIs(t, err, errDB, "%s %d", step.kind, step.nth)
		}
	})
}

func TestBalanceRepository_ListLedgerEntries(t *testing.T) {
	ctx := context.Background()
	db, repo := newBalanceDB(t)
	entries := []model.LedgerEntry{
		{ID: 1, CreatedAt: _readAt, ServiceID: 1, OrgID: "o1", UserID: "u1", EntryType: model.EntryTypeGrant, Amount: 5, BalanceAfter: 5, ActorType: model.ActorTypeAdmin},
		{ID: 2, CreatedAt: _readAt, ServiceID: 1, OrgID: "o1", UserID: "u2", EntryType: model.EntryTypeGrant, Amount: 3, BalanceAfter: 3, ActorType: model.ActorTypeAdmin},
		{ID: 3, CreatedAt: _readAt, ServiceID: 2, OrgID: "o1", UserID: "u1", EntryType: model.EntryTypeGrant, Amount: 4, BalanceAfter: 4, ActorType: model.ActorTypeAdmin},
		{ID: 4, CreatedAt: _readAt, ServiceID: 1, OrgID: "o2", UserID: "u1", EntryType: model.EntryTypeGrant, Amount: 2, BalanceAfter: 2, ActorType: model.ActorTypeAdmin},
		{ID: 5, CreatedAt: _readAt, ServiceID: 1, OrgID: "o1", UserID: "u1", EntryType: model.EntryTypeSpend, Amount: -1, BalanceAfter: 4, ActorType: model.ActorTypeService, Operation: "tool"},
		{ID: 6, CreatedAt: _readAt, ServiceID: 1, OrgID: "o1", UserID: "u1", EntryType: model.EntryTypeRefusal, BalanceAfter: 4, ActorType: model.ActorTypeService, Operation: "tool"},
	}
	storeEntries(t, db, entries...)

	ids := func(f LedgerFilter) []int64 {
		t.Helper()
		rows, err := repo.ListLedgerEntries(ctx, f)
		require.NoError(t, err)
		out := make([]int64, len(rows))
		for i, r := range rows {
			out[i] = r.ID
		}
		return out
	}
	assert.Equal(t, []int64{5, 4, 3, 2, 1}, ids(LedgerFilter{Limit: 10}), "everything but refusals, newest first")
	assert.Equal(t, []int64{5, 4, 2, 1}, ids(LedgerFilter{ServiceID: 1, Limit: 10}))
	assert.Equal(t, []int64{5, 3, 2, 1}, ids(LedgerFilter{OrgID: "o1", Limit: 10}))
	assert.Equal(t, []int64{5, 1}, ids(LedgerFilter{ServiceID: 1, OrgID: "o1", UserID: "u1", Limit: 10}))
	assert.Equal(t, []int64{5, 2, 1}, ids(LedgerFilter{ServiceID: 1, OrgID: "o1", Limit: 10}))
	assert.Equal(t, []int64{5, 3, 1}, ids(LedgerFilter{OrgID: "o1", UserID: "u1", Limit: 10}), "the user in every service")
	assert.Equal(t, []int64{4, 3}, ids(LedgerFilter{BeforeID: 5, Limit: 2}), "a page after the cursor")

	rows, err := repo.ListLedgerEntries(ctx, LedgerFilter{ServiceID: 2, Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "tmphost", rows[0].ServiceName)
	assert.Equal(t, [2]string{"o1", "u1"}, [2]string{rows[0].OrgID, rows[0].UserID}, "read back from the mapping")
	assert.Equal(t, entries[2].Amount, rows[0].Amount)
	assert.Equal(t, entries[2].ActorType, rows[0].ActorType)
	assert.True(t, entries[2].CreatedAt.Equal(rows[0].CreatedAt))

	failNth(t, db, "row", 1)
	_, err = repo.ListLedgerEntries(ctx, LedgerFilter{Limit: 1})
	assert.ErrorIs(t, err, errDB)
}
