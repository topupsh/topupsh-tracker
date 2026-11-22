// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// _decStart is when November's monthly buckets expire.
var _decStart = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

func addUserBucket(t *testing.T, db *gorm.DB, b model.CreditBucket) {
	t.Helper()
	if b.ServiceID == 0 {
		b.ServiceID = 1
	}
	if b.OrgID == "" {
		b.OrgID = "o1"
	}
	b.Amount = max(b.Amount, b.Remaining)
	storeBuckets(t, db, b)
}

func TestExpiryRepository_ExpireBuckets(t *testing.T) {
	ctx := context.Background()
	db, _ := newCreditDB(t)
	repo := NewExpiryRepository(db)
	require.NoError(t, db.Create(&model.Service{ID: 2, Name: "tmphost"}).Error)
	next := ids()

	// u1: November's monthly bucket and a grant both expire on Dec 1; a
	// later grant and December's monthly bucket stay.
	addUserBucket(t, db, model.CreditBucket{ID: 1, UserID: "u1", BucketType: model.BucketTypeMonthly, Amount: 10, Remaining: 4, CreatedAt: _novStart, ExpiresAt: _decStart})
	addUserBucket(t, db, model.CreditBucket{ID: 2, UserID: "u1", BucketType: model.BucketTypeGrant, Remaining: 5, CreatedAt: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), ExpiresAt: _decStart})
	addUserBucket(t, db, model.CreditBucket{ID: 3, UserID: "u1", BucketType: model.BucketTypeGrant, Remaining: 7, CreatedAt: _chargeAt, ExpiresAt: time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC)})
	addUserBucket(t, db, model.CreditBucket{ID: 4, UserID: "u1", BucketType: model.BucketTypeMonthly, Remaining: 10, CreatedAt: _decStart, ExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)})
	// u2's used-up bucket has nothing to expire; u3 is in another org and
	// u4 in another service.
	addUserBucket(t, db, model.CreditBucket{ID: 5, UserID: "u2", BucketType: model.BucketTypeMonthly, Amount: 10, CreatedAt: _novStart, ExpiresAt: _decStart})
	addUserBucket(t, db, model.CreditBucket{ID: 6, OrgID: "o2", UserID: "u3", BucketType: model.BucketTypeGrant, Remaining: 3, CreatedAt: _novStart, ExpiresAt: _decStart})
	addUserBucket(t, db, model.CreditBucket{ID: 7, ServiceID: 2, UserID: "u4", BucketType: model.BucketTypeGrant, Remaining: 2, CreatedAt: _novStart, ExpiresAt: _decStart})

	// Before Dec 1, nothing has expired.
	rs, err := repo.ExpireBuckets(ctx, _decStart.Add(-time.Second), next)
	require.NoError(t, err)
	assert.Equal(t, ExpiryResult{}, rs)

	at := _decStart.Add(time.Hour)
	rs, err = repo.ExpireBuckets(ctx, at, next)
	require.NoError(t, err)
	assert.Equal(t, ExpiryResult{Buckets: 4, Credits: 14}, rs)
	for id, want := range map[int64]int64{1: 0, 2: 0, 3: 7, 4: 10, 5: 0, 6: 0, 7: 0} {
		assert.Equal(t, want, remaining(t, db, id), "bucket %d", id)
	}

	// The oldest bucket expires first, and the balances run down to the
	// user's unexpired credits.
	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 2)
	for i, want := range []struct{ amount, balance int64 }{{-5, 21}, {-4, 17}} {
		assert.Equal(t, model.EntryTypeExpiry, es[i].EntryType)
		assert.Equal(t, want.amount, es[i].Amount)
		assert.Equal(t, want.balance, es[i].BalanceAfter)
		assert.Equal(t, model.ActorTypeScheduler, es[i].ActorType)
		assert.Equal(t, at, es[i].CreatedAt.UTC())
		assert.Equal(t, int64(1), es[i].ServiceID)
	}
	assert.Empty(t, ledgerOf(t, db, "u2"))
	if es := ledgerOfSubject(t, db, 1, "o2", "u3"); assert.Len(t, es, 1) {
		assert.Equal(t, OrgMappingID(1, "o2"), es[0].OrgMappingID)
		assert.Equal(t, int64(0), es[0].BalanceAfter)
	}
	if es := ledgerOfSubject(t, db, 2, "o1", "u4"); assert.Len(t, es, 1) {
		assert.Equal(t, int64(2), es[0].ServiceID)
	}

	// Running again expires nothing more.
	rs, err = repo.ExpireBuckets(ctx, at.Add(time.Hour), next)
	require.NoError(t, err)
	assert.Equal(t, ExpiryResult{}, rs)
	var n int64
	require.NoError(t, db.Model(&model.LedgerEntry{}).Count(&n).Error)
	assert.Equal(t, int64(4), n)
}

func TestExpiryRepository_ExpireBuckets_Batches(t *testing.T) {
	db, _ := newCreditDB(t)
	// One more bucket than a batch holds, all for one user: the user's
	// balances still run down across the batches.
	total := _expiryBatch + 1
	for i := range total {
		addUserBucket(t, db, model.CreditBucket{ID: int64(i + 1), UserID: "u1", BucketType: model.BucketTypeGrant, Remaining: 1, CreatedAt: _novStart.Add(time.Duration(i) * time.Second), ExpiresAt: _decStart})
	}
	rs, err := NewExpiryRepository(db).ExpireBuckets(context.Background(), _decStart, ids())
	require.NoError(t, err)
	assert.Equal(t, ExpiryResult{Buckets: total, Credits: int64(total)}, rs)
	es := ledgerOf(t, db, "u1")
	require.Len(t, es, total)
	for i, e := range es {
		assert.Equal(t, int64(total-i-1), e.BalanceAfter)
	}
}

func TestExpiryRepository_DBErrors(t *testing.T) {
	for _, kind := range []string{"query", "row", "create", "update"} {
		for nth := 1; ; nth++ {
			db, _ := newCreditDB(t)
			addUserBucket(t, db, model.CreditBucket{ID: 1, UserID: "u1", BucketType: model.BucketTypeMonthly, Remaining: 4, CreatedAt: _novStart, ExpiresAt: _decStart})
			addUserBucket(t, db, model.CreditBucket{ID: 2, UserID: "u2", BucketType: model.BucketTypeGrant, Remaining: 5, CreatedAt: _novStart, ExpiresAt: _decStart})
			failNth(t, db, kind, nth)
			_, err := NewExpiryRepository(db).ExpireBuckets(context.Background(), _decStart, ids())
			if err == nil {
				break // past the run's last call of this kind
			}
			assert.ErrorIs(t, err, errDB, "%s #%d", kind, nth)
			require.NoError(t, db.Callback().Query().Remove("test:fail_nth"))
			require.NoError(t, db.Callback().Row().Remove("test:fail_nth"))
			var entries, emptied int64
			require.NoError(t, db.Model(&model.LedgerEntry{}).Count(&entries).Error)
			require.NoError(t, db.Model(&model.CreditBucket{}).Where("remaining = 0").Count(&emptied).Error)
			assert.Equal(t, emptied, entries, "%s #%d: a bucket is expired with its entry or not at all", kind, nth)
		}
	}
}

func TestExpiryRepository_UnknownService(t *testing.T) {
	db, _ := newCreditDB(t)
	addUserBucket(t, db, model.CreditBucket{ID: 1, ServiceID: 9, UserID: "u1", BucketType: model.BucketTypeGrant, Remaining: 1, CreatedAt: _novStart, ExpiresAt: _decStart})
	_, err := NewExpiryRepository(db).ExpireBuckets(context.Background(), _decStart, ids())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestExpiryRepository_ConcurrentRunsExpireOnce(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			db := newConcurrentDB(t, driver)
			for i := range 20 {
				addUserBucket(t, db, model.CreditBucket{ID: int64(i + 1), UserID: "u" + string(rune('a'+i%5)), BucketType: model.BucketTypeGrant, Remaining: 3, CreatedAt: _novStart, ExpiresAt: _decStart})
			}
			// Several replicas run the job at the same time.
			repo := NewExpiryRepository(db)
			next := ids()
			var wg sync.WaitGroup
			var mu sync.Mutex
			var total ExpiryResult
			for range 4 {
				wg.Go(func() {
					rs, err := repo.ExpireBuckets(context.Background(), _decStart, next)
					if assert.NoError(t, err) {
						mu.Lock()
						total.Buckets += rs.Buckets
						total.Credits += rs.Credits
						mu.Unlock()
					}
				})
			}
			wg.Wait()
			assert.Equal(t, ExpiryResult{Buckets: 20, Credits: 60}, total)
			var entries int64
			require.NoError(t, db.Model(&model.LedgerEntry{}).Where("entry_type = ?", model.EntryTypeExpiry).Count(&entries).Error)
			assert.Equal(t, int64(20), entries)
		})
	}
}
