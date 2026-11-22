// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// _chargeAt is in November 2026, whose monthly buckets start on Nov 1.
var _chargeAt = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

var _novStart = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

func newChargeDB(t *testing.T) (*gorm.DB, ChargeRepository) {
	t.Helper()
	db, _ := newCreditDB(t)
	enableService(t, db)
	return db, NewChargeRepository(db)
}

func enableService(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Model(&model.Service{}).Where("id = 1").Update("enabled", true).Error)
}

// ids returns a generator of increasing IDs, safe for concurrent use.
func ids() func() int64 {
	var n atomic.Int64
	n.Store(1000)
	return func() int64 { return n.Add(1) }
}

func setLimits(t *testing.T, db *gorm.DB, org string, monthly int64, user *int64) {
	t.Helper()
	_, err := NewCreditRepository(db).UpsertOrgLimit(context.Background(), model.OrgLimit{
		ID: time.Now().UnixNano(), ServiceID: 1, OrgID: org, MonthlyLimit: monthly, UserLimit: user,
	})
	require.NoError(t, err)
}

func addBucket(t *testing.T, db *gorm.DB, id int64, kind string, credits int64, created, expires time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.CreditBucket{
		ID: id, CreatedAt: created, ServiceID: 1, OrgID: "o1", UserID: "u1",
		Kind: kind, Amount: credits, Remaining: credits, ExpiresAt: expires,
	}).Error)
}

func newCharge(id int64, request, user string, credits int64, at time.Time) model.Charge {
	return model.Charge{
		ID: id, CreatedAt: at, ServiceID: 1, RequestID: request,
		OrgID: "o1", UserID: user, Credits: credits, Operation: "tool",
	}
}

func remaining(t *testing.T, db *gorm.DB, id int64) int64 {
	t.Helper()
	var b model.CreditBucket
	require.NoError(t, db.First(&b, id).Error)
	return b.Remaining
}

func monthlyBucket(t *testing.T, db *gorm.DB, user string) model.CreditBucket {
	t.Helper()
	var b model.CreditBucket
	require.NoError(t, db.Where("user_id = ? AND kind = 'monthly' AND created_at = ?", user, _novStart).First(&b).Error)
	return b
}

func ledgerOf(t *testing.T, db *gorm.DB, user string) []model.LedgerEntry {
	t.Helper()
	var es []model.LedgerEntry
	require.NoError(t, db.Where("user_id = ?", user).Order("id").Find(&es).Error)
	return es
}

func ptr(v int64) *int64 { return &v }

func TestMonthStart(t *testing.T) {
	assert.Equal(t, _novStart, MonthStart(_chargeAt))
	assert.Equal(t, _novStart, MonthStart(_novStart))
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), MonthStart(time.Date(2026, 11, 1, 1, 0, 0, 0, time.FixedZone("x", 2*3600))), "months follow UTC")
}

func TestChargeRepository_Charge_SpendsFIFO(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 10, nil)
	addBucket(t, db, 1, "grant", 9, _chargeAt.AddDate(0, -2, 0), _novStart) // expired
	addBucket(t, db, 2, "grant", 3, time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC))
	addBucket(t, db, 3, "grant", 4, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 11, 0, 0, 0, 0, time.UTC))

	rs, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 12, _chargeAt), next)
	require.NoError(t, err)
	assert.True(t, rs.Limited)
	assert.Empty(t, rs.Charge.Refusal)
	assert.Equal(t, int64(5), rs.Charge.Balance, "3+10+4 less 12")
	assert.Equal(t, int64(1), rs.Charge.OrgRemaining)

	// The October grant goes first, then the monthly bucket of Nov 1; the
	// newer grant is untouched.
	monthly := monthlyBucket(t, db, "u1")
	assert.Equal(t, int64(10), monthly.Amount)
	assert.Equal(t, int64(1), monthly.Remaining)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), monthly.ExpiresAt.UTC())
	assert.Equal(t, int64(9), remaining(t, db, 1), "an expired bucket is not spent")
	assert.Equal(t, int64(0), remaining(t, db, 2))
	assert.Equal(t, int64(4), remaining(t, db, 3))
	var spends []model.ChargeSpend
	require.NoError(t, db.Where("charge_id = 100").Order("id").Find(&spends).Error)
	require.Len(t, spends, 2)
	assert.Equal(t, [2]int64{2, 3}, [2]int64{spends[0].BucketID, spends[0].Credits})
	assert.Equal(t, [2]int64{monthly.ID, 9}, [2]int64{spends[1].BucketID, spends[1].Credits})

	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 2)
	assert.Equal(t, "renewal", es[0].Kind)
	assert.Equal(t, int64(10), es[0].Amount)
	assert.Equal(t, int64(17), es[0].BalanceAfter)
	assert.Equal(t, "spend", es[1].Kind)
	assert.Equal(t, int64(-12), es[1].Amount)
	assert.Equal(t, int64(5), es[1].BalanceAfter)
	assert.Equal(t, "mesher", es[1].Actor)
	assert.Equal(t, "tool", es[1].Operation)

	// 1 monthly credit and 4 granted are left: 6 is refused by the org
	// limit, without spending anything.
	rs, err = repo.Charge(ctx, newCharge(101, "r2", "u1", 6, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, "org_limit", rs.Charge.Refusal)
	assert.Equal(t, int64(5), rs.Charge.Balance)
	assert.Equal(t, int64(1), rs.Charge.OrgRemaining)
	assert.Len(t, ledgerOf(t, db, "u1"), 2)

	rs, err = repo.Charge(ctx, newCharge(102, "r3", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Empty(t, rs.Charge.Refusal)
	assert.Equal(t, int64(0), rs.Charge.Balance)
	assert.Equal(t, int64(0), rs.Charge.OrgRemaining)
}

func TestChargeRepository_Charge_GrantsIgnoreOrgLimit(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	// No limits set: the org limit is the default 0.
	addBucket(t, db, 1, "grant", 5, _chargeAt.AddDate(0, 0, -1), _chargeAt.AddDate(0, 1, 0))

	rs, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 3, _chargeAt), next)
	require.NoError(t, err)
	assert.Empty(t, rs.Charge.Refusal)
	assert.Equal(t, int64(2), rs.Charge.Balance)
	assert.Equal(t, int64(0), rs.Charge.OrgRemaining)
	assert.Equal(t, int64(0), monthlyBucket(t, db, "u1").Amount)

	rs, err = repo.Charge(ctx, newCharge(101, "r2", "u1", 3, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, "org_limit", rs.Charge.Refusal)
	assert.Equal(t, int64(2), rs.Charge.Balance)

	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 1, "an empty monthly bucket writes no renewal")
	assert.Equal(t, "spend", es[0].Kind)
}

func TestChargeRepository_Charge_UserLimit(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	setLimits(t, db, "o1", 100, ptr(2))

	rs, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 3, _chargeAt), ids())
	require.NoError(t, err)
	assert.Equal(t, "user_limit", rs.Charge.Refusal)
	assert.Equal(t, int64(2), rs.Charge.Balance)
	assert.Equal(t, int64(100), rs.Charge.OrgRemaining)
	assert.Equal(t, int64(2), monthlyBucket(t, db, "u1").Remaining)
}

func TestChargeRepository_Charge_UserLimitAboveOrgLimit(t *testing.T) {
	db, repo := newChargeDB(t)
	setLimits(t, db, "o1", 4, ptr(9))

	_, err := repo.Charge(context.Background(), newCharge(100, "r1", "u1", 1, _chargeAt), ids())
	require.NoError(t, err)
	assert.Equal(t, int64(4), monthlyBucket(t, db, "u1").Amount, "a user never gets more than the org")
}

func TestChargeRepository_Charge_OrgLimitIsShared(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 5, nil)

	rs, err := repo.Charge(ctx, newCharge(100, "r1", "u2", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Empty(t, rs.Charge.Refusal)

	rs, err = repo.Charge(ctx, newCharge(101, "r2", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, "org_limit", rs.Charge.Refusal)
	assert.Equal(t, int64(0), rs.Charge.OrgRemaining)

	// u1's monthly credits are blocked by the org, a newer grant isn't.
	addBucket(t, db, 1, "grant", 3, _chargeAt, _chargeAt.AddDate(0, 1, 0))
	rs, err = repo.Charge(ctx, newCharge(102, "r3", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	assert.Empty(t, rs.Charge.Refusal)
	assert.Equal(t, int64(1), remaining(t, db, 1))
	assert.Equal(t, int64(5), monthlyBucket(t, db, "u1").Remaining)
}

func TestChargeRepository_Charge_Replay(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 10, ptr(3))

	first, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	again, err := repo.Charge(ctx, newCharge(101, "r1", "u1", 2, _chargeAt.Add(time.Hour)), next)
	require.NoError(t, err)
	assert.Equal(t, first.Charge.ID, again.Charge.ID)
	assert.Equal(t, int64(1), again.Charge.Balance, "the original balance")
	assert.Equal(t, int64(1), monthlyBucket(t, db, "u1").Remaining, "charged once")
	assert.Len(t, ledgerOf(t, db, "u1"), 2)

	for _, c := range []model.Charge{
		newCharge(102, "r1", "u1", 3, _chargeAt),
		newCharge(102, "r1", "u2", 2, _chargeAt),
		{ID: 102, ServiceID: 1, RequestID: "r1", OrgID: "o2", UserID: "u1", Credits: 2},
	} {
		_, err = repo.Charge(ctx, c, next)
		assert.ErrorIs(t, err, ErrRequestConflict)
	}

	refused, err := repo.Charge(ctx, newCharge(103, "r2", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, "user_limit", refused.Charge.Refusal)
	setLimits(t, db, "o1", 10, nil)
	again, err = repo.Charge(ctx, newCharge(104, "r2", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, "user_limit", again.Charge.Refusal, "a retry gets the original refusal")
}

func TestChargeRepository_Charge_ResizesMonthlyBucket(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 10, nil)
	_, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 6, _chargeAt), next)
	require.NoError(t, err)

	// Raised: the user gets the difference.
	setLimits(t, db, "o1", 20, nil)
	rs, err := repo.Charge(ctx, newCharge(101, "r2", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, int64(13), rs.Charge.Balance)
	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 4)
	assert.Equal(t, "limit", es[2].Kind)
	assert.Equal(t, int64(10), es[2].Amount)
	assert.Equal(t, int64(14), es[2].BalanceAfter)

	// Lowered below what was used (7): nothing is left, and the bucket
	// keeps the used credits.
	setLimits(t, db, "o1", 3, nil)
	rs, err = repo.Charge(ctx, newCharge(102, "r3", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, "org_limit", rs.Charge.Refusal)
	assert.Equal(t, int64(0), rs.Charge.Balance)
	b := monthlyBucket(t, db, "u1")
	assert.Equal(t, [2]int64{7, 0}, [2]int64{b.Amount, b.Remaining})
	assert.Equal(t, int64(-13), ledgerOf(t, db, "u1")[4].Amount)

	// A refund lowers what was used, so the next charge fits the limit.
	_, err = repo.Refund(ctx, 1, "r1", _chargeAt, next)
	require.NoError(t, err)
	rs, err = repo.Charge(ctx, newCharge(103, "r4", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	assert.Empty(t, rs.Charge.Refusal)
	b = monthlyBucket(t, db, "u1")
	assert.Equal(t, [2]int64{3, 0}, [2]int64{b.Amount, b.Remaining})

	// Unchanged limits write nothing.
	n := len(ledgerOf(t, db, "u1"))
	_, err = repo.Charge(ctx, newCharge(104, "r5", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Len(t, ledgerOf(t, db, "u1"), n)
}

func TestChargeRepository_Charge_SwitchedOffOrUnknown(t *testing.T) {
	ctx := context.Background()
	db, _ := newCreditDB(t)
	repo := NewChargeRepository(db)

	rs, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 5, _chargeAt), ids())
	require.NoError(t, err)
	assert.False(t, rs.Limited)
	var n int64
	require.NoError(t, db.Model(&model.Charge{}).Count(&n).Error)
	assert.Zero(t, n, "nothing is recorded")
	require.NoError(t, db.Model(&model.CreditBucket{}).Count(&n).Error)
	assert.Zero(t, n)

	c := newCharge(100, "r1", "u1", 5, _chargeAt)
	c.ServiceID = 2
	_, err = repo.Charge(ctx, c, ids())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestChargeRepository_Refund(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 10, nil)
	grantExpiry := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	addBucket(t, db, 1, "grant", 2, time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC), grantExpiry)

	_, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	rs, err := repo.Refund(ctx, 1, "r1", _chargeAt.Add(time.Minute), next)
	require.NoError(t, err)
	assert.True(t, rs.Limited)
	assert.Equal(t, int64(5), rs.Charge.RefundedCredits)
	require.NotNil(t, rs.Charge.RefundedAt)
	assert.Equal(t, int64(12), rs.Balance)
	assert.Equal(t, int64(2), remaining(t, db, 1))
	assert.Equal(t, int64(10), monthlyBucket(t, db, "u1").Remaining)
	es := ledgerOf(t, db, "u1")
	last := es[len(es)-1]
	assert.Equal(t, [3]any{"refund", int64(5), int64(12)}, [3]any{last.Kind, last.Amount, last.BalanceAfter})

	// Refunded once: a retry returns the same refund.
	again, err := repo.Refund(ctx, 1, "r1", _chargeAt.Add(time.Hour), next)
	require.NoError(t, err)
	assert.Equal(t, int64(5), again.Charge.RefundedCredits)
	assert.Equal(t, rs.Charge.RefundedAt.UTC(), again.Charge.RefundedAt.UTC())
	assert.Equal(t, int64(12), again.Balance)
	assert.Len(t, ledgerOf(t, db, "u1"), len(es))

	// The expired grant's share is not returned.
	_, err = repo.Charge(ctx, newCharge(101, "r2", "u1", 3, _chargeAt), next)
	require.NoError(t, err)
	rs, err = repo.Refund(ctx, 1, "r2", grantExpiry, next)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rs.Charge.RefundedCredits)
	assert.Equal(t, int64(0), remaining(t, db, 1))

	// Nothing returned, nothing recorded.
	_, err = repo.Charge(ctx, newCharge(102, "r3", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	n := len(ledgerOf(t, db, "u1"))
	rs, err = repo.Refund(ctx, 1, "r3", time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), next)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rs.Charge.RefundedCredits)
	assert.NotNil(t, rs.Charge.RefundedAt)
	assert.Len(t, ledgerOf(t, db, "u1"), n)
}

func TestChargeRepository_Refund_Errors(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()

	_, err := repo.Refund(ctx, 1, "nope", _chargeAt, next)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = repo.Refund(ctx, 2, "nope", _chargeAt, next)
	assert.ErrorIs(t, err, ErrNotFound)

	rs, err := repo.Charge(ctx, newCharge(100, "r1", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	require.Equal(t, "org_limit", rs.Charge.Refusal)
	_, err = repo.Refund(ctx, 1, "r1", _chargeAt, next)
	assert.ErrorIs(t, err, ErrNotCharged)

	require.NoError(t, db.Model(&model.Service{}).Where("id = 1").Update("enabled", false).Error)
	off, err := repo.Refund(ctx, 1, "nope", _chargeAt, next)
	require.NoError(t, err)
	assert.False(t, off.Limited, "a charge made while off was never recorded")
}

// failNth makes the nth database call of kind ("query", "row", "create" or
// "update") fail from now on.
func failNth(t *testing.T, db *gorm.DB, kind string, nth int) {
	t.Helper()
	var calls int
	fail := func(d *gorm.DB) {
		if calls++; calls == nth {
			_ = d.AddError(errDB)
		}
	}
	var err error
	switch kind {
	case "query":
		err = db.Callback().Query().Before("gorm:query").Register("test:fail_nth", fail)
	case "row":
		err = db.Callback().Row().Before("gorm:row").Register("test:fail_nth", fail)
	case "create":
		err = db.Callback().Create().Before("gorm:create").Register("test:fail_nth", fail)
	case "update":
		err = db.Callback().Update().Before("gorm:update").Register("test:fail_nth", fail)
	}
	require.NoError(t, err)
}

var errDB = errors.New("db down")

// TestChargeRepository_DBErrors fails each database call of each scenario
// in turn: every failure must surface and leave the stored state as it was.
func TestChargeRepository_DBErrors(t *testing.T) {
	ctx := context.Background()
	type scenario struct {
		setup func(t *testing.T, db *gorm.DB, repo ChargeRepository)
		act   func(repo ChargeRepository) error
	}
	charge := func(c model.Charge) func(repo ChargeRepository) error {
		return func(repo ChargeRepository) error {
			_, err := repo.Charge(ctx, c, ids())
			return err
		}
	}
	scenarios := map[string]scenario{
		"first charge": {
			setup: func(t *testing.T, db *gorm.DB, _ ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				addBucket(t, db, 1, "grant", 2, time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC), _chargeAt.AddDate(0, 1, 0))
			},
			act: charge(newCharge(100, "r1", "u1", 5, _chargeAt)),
		},
		"resized and refused": {
			setup: func(t *testing.T, db *gorm.DB, repo ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				require.NoError(t, charge(newCharge(100, "r1", "u1", 1, _chargeAt))(repo))
				setLimits(t, db, "o1", 2, nil)
			},
			act: charge(newCharge(101, "r2", "u1", 5, _chargeAt)),
		},
		"replay": {
			setup: func(t *testing.T, _ *gorm.DB, repo ChargeRepository) {
				require.NoError(t, charge(newCharge(100, "r1", "u1", 5, _chargeAt))(repo))
			},
			act: charge(newCharge(100, "r1", "u1", 5, _chargeAt)),
		},
		"refund": {
			setup: func(t *testing.T, db *gorm.DB, repo ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				require.NoError(t, charge(newCharge(100, "r1", "u1", 5, _chargeAt))(repo))
			},
			act: func(repo ChargeRepository) error {
				_, err := repo.Refund(ctx, 1, "r1", _chargeAt, ids())
				return err
			},
		},
	}
	for name, sc := range scenarios {
		for _, kind := range []string{"query", "row", "create", "update"} {
			for nth := 1; ; nth++ {
				db, repo := newChargeDB(t)
				sc.setup(t, db, repo)
				var before int64
				require.NoError(t, db.Model(&model.LedgerEntry{}).Count(&before).Error)
				failNth(t, db, kind, nth)
				err := sc.act(repo)
				if err == nil {
					break // past the scenario's last call of this kind
				}
				assert.ErrorIs(t, err, errDB, "%s: %s #%d", name, kind, nth)
				require.NoError(t, db.Callback().Query().Remove("test:fail_nth"))
				require.NoError(t, db.Callback().Row().Remove("test:fail_nth"))
				var after int64
				require.NoError(t, db.Model(&model.LedgerEntry{}).Count(&after).Error)
				assert.Equal(t, before, after, "%s: %s #%d rolls back", name, kind, nth)
			}
		}
	}
}

// newConcurrentDB opens a database that several connections share: a
// SQLite file whose transactions take the write lock up front, or the
// Postgres database at TOPUPSH_TEST_POSTGRES_DSN.
func newConcurrentDB(t *testing.T, driver string) *gorm.DB {
	t.Helper()
	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		dsn := os.Getenv("TOPUPSH_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TOPUPSH_TEST_POSTGRES_DSN is not set")
		}
		dialector = postgres.Open(dsn)
	default:
		dialector = sqlite.Open("file:" + filepath.Join(t.TempDir(), "c.db") + "?_pragma=busy_timeout(10000)&_txlock=immediate")
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	if driver == "postgres" {
		require.NoError(t, db.Migrator().DropTable(model.Models()...))
	}
	require.NoError(t, db.AutoMigrate(model.Models()...))
	require.NoError(t, db.Create(&model.Service{ID: 1, Name: "mesher", Enabled: true}).Error)
	return db
}

func TestChargeRepository_ConcurrentChargesNeverOverspend(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			db := newConcurrentDB(t, driver)
			repo := NewChargeRepository(db)
			next := ids()
			setLimits(t, db, "o1", 30, ptr(10))
			addBucket(t, db, 1, "grant", 5, _chargeAt.AddDate(0, 0, -1), _chargeAt.AddDate(0, 1, 0))

			// 4 users try 60 credits; the org allows 30 (10 each for 3
			// users) and u1 has a 5-credit grant on top.
			var wg sync.WaitGroup
			var charged atomic.Int64
			for i := range 60 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					c := newCharge(next(), fmt.Sprintf("r%d", i), fmt.Sprintf("u%d", i%4+1), 1, _chargeAt)
					rs, err := repo.Charge(context.Background(), c, next)
					if assert.NoError(t, err) && rs.Charge.Refusal == "" {
						charged.Add(1)
					}
				}()
			}
			wg.Wait()
			assert.Equal(t, int64(35), charged.Load())

			var orgUsed int64
			require.NoError(t, db.Model(&model.CreditBucket{}).Where("kind = 'monthly'").
				Select("SUM(amount - remaining)").Scan(&orgUsed).Error)
			assert.Equal(t, int64(30), orgUsed)
			// Each user's ledger is a chain: every balance follows the one
			// before, and the last one is the user's balance now.
			for u := 1; u <= 4; u++ {
				user := fmt.Sprintf("u%d", u)
				var balance int64
				es := ledgerOf(t, db, user)
				for _, e := range es {
					balance += e.Amount
					assert.Equal(t, balance, e.BalanceAfter+grantOf(user), user)
				}
			}
		})
	}
}

// grantOf is the grant made before the concurrency test, outside the ledger.
func grantOf(user string) int64 {
	if user == "u1" {
		return -5
	}
	return 0
}
