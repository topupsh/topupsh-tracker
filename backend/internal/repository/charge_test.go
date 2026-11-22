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
func ids() func() int64 { return idsFrom(1000) }

// idsFrom returns IDs after start, for an act whose IDs must not meet
// those of its setup.
func idsFrom(start int64) func() int64 {
	var n atomic.Int64
	n.Store(start)
	return func() int64 { return n.Add(1) }
}

// _mappingIDs gives the IDs of the mappings tests store, far from the
// other rows' IDs.
var _mappingIDs = idsFrom(1 << 40)

// mapped stores the mappings of a service's org and user, as a write
// would, and returns their mapping IDs.
func mapped(t *testing.T, db *gorm.DB, serviceID int64, org, user string) (int64, int64) {
	t.Helper()
	o, u, err := mapUser(db, _mappingIDs, serviceID, org, user)
	require.NoError(t, err)
	return o, u
}

// storeBuckets stores buckets under the mappings of their OrgID and
// UserID.
func storeBuckets(t *testing.T, db *gorm.DB, bs ...model.CreditBucket) {
	t.Helper()
	for _, b := range bs {
		b.OrgMappingID, b.UserMappingID = mapped(t, db, b.ServiceID, b.OrgID, b.UserID)
		require.NoError(t, db.Create(&b).Error)
	}
}

// storeEntries stores ledger entries under the mappings of their OrgID,
// UserID and Operation.
func storeEntries(t *testing.T, db *gorm.DB, es ...model.LedgerEntry) {
	t.Helper()
	for _, e := range es {
		e.OrgMappingID, e.UserMappingID = mapped(t, db, e.ServiceID, e.OrgID, e.UserID)
		var err error
		e.OperationMappingID, err = mapOperation(db, _mappingIDs, e.ServiceID, e.Operation)
		require.NoError(t, err)
		require.NoError(t, db.Create(&e).Error)
	}
}

func setLimits(t *testing.T, db *gorm.DB, org string, monthly int64, user *int64) {
	t.Helper()
	_, err := NewCreditRepository(db).UpsertOrgLimit(context.Background(), model.OrgLimit{
		ID: time.Now().UnixNano(), ServiceID: 1, OrgID: org, MonthlyLimit: monthly, UserLimit: user,
	}, _mappingIDs)
	require.NoError(t, err)
}

func addBucket(t *testing.T, db *gorm.DB, id int64, kind string, credits int64, created, expires time.Time) {
	t.Helper()
	org, user := mapped(t, db, 1, "o1", "u1")
	require.NoError(t, db.Create(&model.CreditBucket{
		ID: id, CreatedAt: created, ServiceID: 1, OrgMappingID: org, UserMappingID: user,
		BucketType: _bucketTypes[kind], Amount: credits, Remaining: credits, ExpiresAt: expires,
	}).Error)
}

// newCharge is a charge of credits as the controller sends it.
func newCharge(request, user string, credits int64, at time.Time) model.LedgerEntry {
	return model.LedgerEntry{
		CreatedAt: at, ServiceID: 1, RequestID: &request,
		OrgID: "o1", UserID: user, Amount: -credits, Operation: "tool",
	}
}

func spendsOf(t *testing.T, db *gorm.DB, entryID int64) []model.ChargeSpend {
	t.Helper()
	var spends []model.ChargeSpend
	require.NoError(t, db.Where("entry_id = ?", entryID).Find(&spends).Error)
	return spends
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
	require.NoError(t, db.Where("org_mapping_id = ? AND user_mapping_id = ? AND bucket_type = ? AND created_at = ?", OrgMappingID(1, "o1"), UserMappingID(1, "o1", user), model.BucketTypeMonthly, _novStart).First(&b).Error)
	return b
}

// ledgerOf returns the ledger entries of a user of service 1's org o1,
// refusals left out.
func ledgerOf(t *testing.T, db *gorm.DB, user string) []model.LedgerEntry {
	t.Helper()
	return ledgerOfSubject(t, db, 1, "o1", user)
}

func ledgerOfSubject(t *testing.T, db *gorm.DB, serviceID int64, org, user string) []model.LedgerEntry {
	t.Helper()
	var es []model.LedgerEntry
	require.NoError(t, db.Where("org_mapping_id = ? AND user_mapping_id = ? AND entry_type <> ?", OrgMappingID(serviceID, org), UserMappingID(serviceID, org, user), model.EntryTypeRefusal).
		Order("id").Find(&es).Error)
	return es
}

func ptr(v int64) *int64 { return &v }

// _bucketTypes are the stored types of the bucket kinds tests name.
var _bucketTypes = map[string]model.BucketType{"monthly": model.BucketTypeMonthly, "grant": model.BucketTypeGrant}

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

	rs, err := repo.Charge(ctx, newCharge("r1", "u1", 12, _chargeAt), next)
	require.NoError(t, err)
	assert.True(t, rs.Limited)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType)
	assert.Equal(t, int64(5), rs.Entry.BalanceAfter, "3+10+4 less 12")
	assert.Equal(t, int64(1), rs.Entry.OrgRemaining)

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
	require.NoError(t, db.Where("entry_id = ?", rs.Entry.ID).Order("id").Find(&spends).Error)
	require.Len(t, spends, 2)
	assert.Equal(t, [2]int64{2, 3}, [2]int64{spends[0].BucketID, spends[0].Credits})
	assert.Equal(t, [2]int64{monthly.ID, 9}, [2]int64{spends[1].BucketID, spends[1].Credits})

	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 2)
	assert.Equal(t, es[1], model.LedgerEntry{
		ID: rs.Entry.ID, CreatedAt: es[1].CreatedAt, ServiceID: 1, OrgMappingID: es[1].OrgMappingID, UserMappingID: es[1].UserMappingID,
		EntryType: model.EntryTypeSpend, RequestID: es[1].RequestID, Amount: -12, BalanceAfter: 5, OrgRemaining: 1, ActorType: model.ActorTypeService,
		OperationMappingID: OperationMappingID(1, "tool"),
	}, "the spend entry is the charge")
	assert.Equal(t, "r1", *es[1].RequestID)
	assert.Greater(t, es[1].ID, es[0].ID, "the spend follows the renewal")
	assert.Equal(t, model.EntryTypeRenewal, es[0].EntryType)
	assert.Nil(t, es[0].RequestID, "a system entry has no request")
	assert.Equal(t, int64(10), es[0].Amount)
	assert.Equal(t, int64(17), es[0].BalanceAfter)
	assert.Equal(t, model.EntryTypeSpend, es[1].EntryType)
	assert.Equal(t, int64(-12), es[1].Amount)
	assert.Equal(t, int64(5), es[1].BalanceAfter)
	assert.Equal(t, model.ActorTypeService, es[1].ActorType)
	assert.Equal(t, OperationMappingID(1, "tool"), es[1].OperationMappingID)
	op, err := operationName(db, 1, es[1].OperationMappingID)
	require.NoError(t, err)
	assert.Equal(t, "tool", op, "the operation is mapped")

	// 1 monthly credit and 4 granted are left: 6 is refused by the org
	// limit, without spending anything.
	rs, err = repo.Charge(ctx, newCharge("r2", "u1", 6, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonOrgLimit, rs.Entry.RefusalReason)
	assert.Equal(t, int64(5), rs.Entry.BalanceAfter)
	assert.Equal(t, int64(1), rs.Entry.OrgRemaining)
	assert.Equal(t, [2]int64{0, 0}, [2]int64{rs.Entry.Amount, int64(len(spendsOf(t, db, rs.Entry.ID)))}, "a refusal spends nothing")
	var refusal model.LedgerEntry
	require.NoError(t, db.Where("request_id = ?", "r2").First(&refusal).Error)
	assert.Equal(t, model.EntryTypeRefusal, refusal.EntryType, "the refusal is stored")
	assert.Len(t, ledgerOf(t, db, "u1"), 2)

	rs, err = repo.Charge(ctx, newCharge("r3", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType)
	assert.Equal(t, int64(0), rs.Entry.BalanceAfter)
	assert.Equal(t, int64(0), rs.Entry.OrgRemaining)
}

func TestChargeRepository_Charge_GrantsIgnoreOrgLimit(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	// No limits set: the org limit is the default 0.
	addBucket(t, db, 1, "grant", 5, _chargeAt.AddDate(0, 0, -1), _chargeAt.AddDate(0, 1, 0))

	rs, err := repo.Charge(ctx, newCharge("r1", "u1", 3, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType)
	assert.Equal(t, int64(2), rs.Entry.BalanceAfter)
	assert.Equal(t, int64(0), rs.Entry.OrgRemaining)
	assert.Equal(t, int64(0), monthlyBucket(t, db, "u1").Amount)

	rs, err = repo.Charge(ctx, newCharge("r2", "u1", 3, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonOrgLimit, rs.Entry.RefusalReason)
	assert.Equal(t, int64(2), rs.Entry.BalanceAfter)

	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 1, "an empty monthly bucket writes no renewal")
	assert.Equal(t, model.EntryTypeSpend, es[0].EntryType)
}

func TestChargeRepository_Charge_UserLimit(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	setLimits(t, db, "o1", 100, ptr(2))

	rs, err := repo.Charge(ctx, newCharge("r1", "u1", 3, _chargeAt), ids())
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonUserLimit, rs.Entry.RefusalReason)
	assert.Equal(t, int64(2), rs.Entry.BalanceAfter)
	assert.Equal(t, int64(100), rs.Entry.OrgRemaining)
	assert.Equal(t, int64(2), monthlyBucket(t, db, "u1").Remaining)
}

func TestChargeRepository_Charge_ServiceDefaults(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	_, err := NewServiceRepository(db).SetServiceDefaults(ctx, "mesher", 100, ptr(2))
	require.NoError(t, err)

	rs, err := repo.Charge(ctx, newCharge("r1", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType, "an org without limits spends under the service's defaults")
	assert.Equal(t, int64(98), rs.Entry.OrgRemaining)
	rs, err = repo.Charge(ctx, newCharge("r2", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonUserLimit, rs.Entry.RefusalReason, "the default user limit applies")

	setLimits(t, db, "o1", 100, nil)
	rs, err = repo.Charge(ctx, newCharge("r3", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType, "the org's own limits win over the defaults")
}

func TestChargeRepository_Charge_UserLimitAboveOrgLimit(t *testing.T) {
	db, repo := newChargeDB(t)
	setLimits(t, db, "o1", 4, ptr(9))

	_, err := repo.Charge(context.Background(), newCharge("r1", "u1", 1, _chargeAt), ids())
	require.NoError(t, err)
	assert.Equal(t, int64(4), monthlyBucket(t, db, "u1").Amount, "a user never gets more than the org")
}

func TestChargeRepository_Charge_OrgLimitIsShared(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 5, nil)

	rs, err := repo.Charge(ctx, newCharge("r1", "u2", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType)

	rs, err = repo.Charge(ctx, newCharge("r2", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonOrgLimit, rs.Entry.RefusalReason)
	assert.Equal(t, int64(0), rs.Entry.OrgRemaining)

	// u1's monthly credits are blocked by the org, a newer grant isn't.
	addBucket(t, db, 1, "grant", 3, _chargeAt, _chargeAt.AddDate(0, 1, 0))
	rs, err = repo.Charge(ctx, newCharge("r3", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType)
	assert.Equal(t, int64(1), remaining(t, db, 1))
	assert.Equal(t, int64(5), monthlyBucket(t, db, "u1").Remaining)
}

func TestChargeRepository_Charge_Replay(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 10, ptr(3))

	first, err := repo.Charge(ctx, newCharge("r1", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	again, err := repo.Charge(ctx, newCharge("r1", "u1", 2, _chargeAt.Add(time.Hour)), next)
	require.NoError(t, err)
	assert.Equal(t, first.Entry.ID, again.Entry.ID)
	assert.Equal(t, int64(1), again.Entry.BalanceAfter, "the original balance")
	assert.Equal(t, int64(1), monthlyBucket(t, db, "u1").Remaining, "charged once")
	assert.Len(t, ledgerOf(t, db, "u1"), 2)

	otherOrg := newCharge("r1", "u1", 2, _chargeAt)
	otherOrg.OrgID = "o2"
	for _, c := range []model.LedgerEntry{newCharge("r1", "u1", 3, _chargeAt), newCharge("r1", "u2", 2, _chargeAt), otherOrg} {
		_, err = repo.Charge(ctx, c, next)
		assert.ErrorIs(t, err, ErrRequestConflict)
	}
	_, err = repo.Refund(ctx, 1, "f1", "r1", _chargeAt, next)
	require.NoError(t, err)
	_, err = repo.Charge(ctx, newCharge("f1", "u1", 2, _chargeAt), next)
	assert.ErrorIs(t, err, ErrRequestConflict, "the request ID of a refund")

	refused, err := repo.Charge(ctx, newCharge("r2", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonUserLimit, refused.Entry.RefusalReason)
	setLimits(t, db, "o1", 10, nil)
	again, err = repo.Charge(ctx, newCharge("r2", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonUserLimit, again.Entry.RefusalReason, "a retry gets the original refusal")
	assert.Equal(t, refused.Entry.ID, again.Entry.ID)
}

func TestChargeRepository_Charge_ResizesMonthlyBucket(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()
	setLimits(t, db, "o1", 10, nil)
	_, err := repo.Charge(ctx, newCharge("r1", "u1", 6, _chargeAt), next)
	require.NoError(t, err)

	// Raised: the user gets the difference.
	setLimits(t, db, "o1", 20, nil)
	rs, err := repo.Charge(ctx, newCharge("r2", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, int64(13), rs.Entry.BalanceAfter)
	es := ledgerOf(t, db, "u1")
	require.Len(t, es, 4)
	assert.Equal(t, model.EntryTypeLimit, es[2].EntryType)
	assert.Equal(t, int64(10), es[2].Amount)
	assert.Equal(t, int64(14), es[2].BalanceAfter)

	// Lowered below what was used (7): nothing is left, and the bucket
	// keeps the used credits.
	setLimits(t, db, "o1", 3, nil)
	rs, err = repo.Charge(ctx, newCharge("r3", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.RefusalReasonOrgLimit, rs.Entry.RefusalReason)
	assert.Equal(t, int64(0), rs.Entry.BalanceAfter)
	b := monthlyBucket(t, db, "u1")
	assert.Equal(t, [2]int64{7, 0}, [2]int64{b.Amount, b.Remaining})
	assert.Equal(t, int64(-13), ledgerOf(t, db, "u1")[4].Amount)

	// A refund lowers what was used, so the next charge fits the limit.
	_, err = repo.Refund(ctx, 1, "fr1", "r1", _chargeAt, next)
	require.NoError(t, err)
	rs, err = repo.Charge(ctx, newCharge("r4", "u1", 2, _chargeAt), next)
	require.NoError(t, err)
	assert.Equal(t, model.EntryTypeSpend, rs.Entry.EntryType)
	b = monthlyBucket(t, db, "u1")
	assert.Equal(t, [2]int64{3, 0}, [2]int64{b.Amount, b.Remaining})

	// Unchanged limits write nothing.
	n := len(ledgerOf(t, db, "u1"))
	_, err = repo.Charge(ctx, newCharge("r5", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	assert.Len(t, ledgerOf(t, db, "u1"), n)
}

func TestChargeRepository_Charge_SwitchedOffOrUnknown(t *testing.T) {
	ctx := context.Background()
	db, _ := newCreditDB(t)
	repo := NewChargeRepository(db)

	rs, err := repo.Charge(ctx, newCharge("r1", "u1", 5, _chargeAt), ids())
	require.NoError(t, err)
	assert.False(t, rs.Limited)
	var n int64
	require.NoError(t, db.Model(&model.LedgerEntry{}).Count(&n).Error)
	assert.Zero(t, n, "nothing is recorded")
	require.NoError(t, db.Model(&model.CreditBucket{}).Count(&n).Error)
	assert.Zero(t, n)

	c := newCharge("r1", "u1", 5, _chargeAt)
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

	charged, err := repo.Charge(ctx, newCharge("r1", "u1", 5, _chargeAt), next)
	require.NoError(t, err)
	rs, err := repo.Refund(ctx, 1, "f1", "r1", _chargeAt.Add(time.Minute), next)
	require.NoError(t, err)
	assert.True(t, rs.Limited)
	assert.Equal(t, model.EntryTypeRefund, rs.Refund.EntryType)
	assert.Equal(t, int64(5), rs.Refund.Amount)
	assert.Equal(t, charged.Entry.ID, *rs.Refund.RefundOf)
	assert.Equal(t, "f1", *rs.Refund.RequestID, "a refund has its own request")
	assert.Equal(t, [3]string{"o1", "u1", "tool"}, [3]string{rs.Refund.OrgID, rs.Refund.UserID, rs.Refund.Operation})
	assert.Equal(t, OperationMappingID(1, "tool"), rs.Refund.OperationMappingID)
	assert.Equal(t, _chargeAt.Add(time.Minute), rs.Refund.CreatedAt.UTC())
	assert.Equal(t, int64(12), rs.Refund.BalanceAfter)
	assert.Equal(t, int64(10), rs.Refund.OrgRemaining, "the refund gave the org limit back")
	assert.Equal(t, model.ActorTypeService, rs.Refund.ActorType)
	assert.Equal(t, int64(2), remaining(t, db, 1))
	assert.Equal(t, int64(10), monthlyBucket(t, db, "u1").Remaining)
	es := ledgerOf(t, db, "u1")
	assert.Equal(t, rs.Refund.ID, es[len(es)-1].ID)
	assert.Equal(t, int64(-5), es[len(es)-2].Amount, "the spend stays as it was")

	// Refunded once: a retry, or a refund under another request ID,
	// returns the same refund.
	for _, request := range []string{"f1", "f1b"} {
		again, err := repo.Refund(ctx, 1, request, "r1", _chargeAt.Add(time.Hour), next)
		require.NoError(t, err)
		assert.Equal(t, rs.Refund.ID, again.Refund.ID)
		assert.Equal(t, int64(5), again.Refund.Amount)
		assert.Equal(t, [3]string{"o1", "u1", "tool"}, [3]string{again.Refund.OrgID, again.Refund.UserID, again.Refund.Operation})
		assert.Equal(t, rs.Refund.CreatedAt.UTC(), again.Refund.CreatedAt.UTC())
		assert.Equal(t, int64(12), again.Refund.BalanceAfter)
		assert.Len(t, ledgerOf(t, db, "u1"), len(es))
	}

	// The expired grant's share is not returned.
	_, err = repo.Charge(ctx, newCharge("r2", "u1", 3, _chargeAt), next)
	require.NoError(t, err)
	rs, err = repo.Refund(ctx, 1, "f2", "r2", grantExpiry, next)
	require.NoError(t, err)
	assert.Equal(t, int64(1), rs.Refund.Amount)
	assert.Equal(t, int64(0), remaining(t, db, 1))

	// Nothing returned is still a refund entry, so a retry finds it.
	charged, err = repo.Charge(ctx, newCharge("r3", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	rs, err = repo.Refund(ctx, 1, "f3", "r3", time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), next)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rs.Refund.Amount)
	assert.Equal(t, charged.Entry.ID, *rs.Refund.RefundOf)
	assert.Equal(t, int64(0), rs.Refund.BalanceAfter)
	es = ledgerOf(t, db, "u1")
	assert.Equal(t, rs.Refund.ID, es[len(es)-1].ID)
}

func TestChargeRepository_Refund_Errors(t *testing.T) {
	ctx := context.Background()
	db, repo := newChargeDB(t)
	next := ids()

	_, err := repo.Refund(ctx, 1, "f1", "nope", _chargeAt, next)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = repo.Refund(ctx, 2, "f1", "nope", _chargeAt, next)
	assert.ErrorIs(t, err, ErrNotFound)

	rs, err := repo.Charge(ctx, newCharge("r1", "u1", 1, _chargeAt), next)
	require.NoError(t, err)
	require.Equal(t, model.RefusalReasonOrgLimit, rs.Entry.RefusalReason)
	_, err = repo.Refund(ctx, 1, "f1", "r1", _chargeAt, next)
	assert.ErrorIs(t, err, ErrNotCharged)

	// Two charges, the first refunded: its refund's request ID can't
	// refund the second, nor be refunded itself.
	setLimits(t, db, "o1", 10, nil)
	for _, r := range []string{"r2", "r3"} {
		_, err = repo.Charge(ctx, newCharge(r, "u1", 1, _chargeAt), next)
		require.NoError(t, err)
	}
	_, err = repo.Refund(ctx, 1, "f2", "r2", _chargeAt, next)
	require.NoError(t, err)
	for _, tc := range []struct {
		request, charge string
		want            error
	}{
		{"f2", "r3", ErrRequestConflict},
		{"r2", "r3", ErrRequestConflict},
		{"f3", "f2", ErrNotFound},
	} {
		_, err = repo.Refund(ctx, 1, tc.request, tc.charge, _chargeAt, next)
		assert.ErrorIs(t, err, tc.want, "%s of %s", tc.request, tc.charge)
	}

	require.NoError(t, db.Model(&model.Service{}).Where("id = 1").Update("enabled", false).Error)
	off, err := repo.Refund(ctx, 1, "f1", "nope", _chargeAt, next)
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
	// The IDs keep increasing across setups and acts, so an act never
	// meets its setup's IDs.
	next := idsFrom(5000)
	charge := func(c model.LedgerEntry) func(repo ChargeRepository) error {
		return func(repo ChargeRepository) error {
			_, err := repo.Charge(ctx, c, next)
			return err
		}
	}
	scenarios := map[string]scenario{
		"first charge": {
			setup: func(t *testing.T, db *gorm.DB, _ ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				addBucket(t, db, 1, "grant", 2, time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC), _chargeAt.AddDate(0, 1, 0))
			},
			act: charge(newCharge("r1", "u1", 5, _chargeAt)),
		},
		"resized and refused": {
			setup: func(t *testing.T, db *gorm.DB, repo ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				require.NoError(t, charge(newCharge("r1", "u1", 1, _chargeAt))(repo))
				setLimits(t, db, "o1", 2, nil)
			},
			act: charge(newCharge("r2", "u1", 5, _chargeAt)),
		},
		"replay": {
			setup: func(t *testing.T, _ *gorm.DB, repo ChargeRepository) {
				require.NoError(t, charge(newCharge("r1", "u1", 5, _chargeAt))(repo))
			},
			act: charge(newCharge("r1", "u1", 5, _chargeAt)),
		},
		"refund": {
			setup: func(t *testing.T, db *gorm.DB, repo ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				require.NoError(t, charge(newCharge("r1", "u1", 5, _chargeAt))(repo))
			},
			act: func(repo ChargeRepository) error {
				_, err := repo.Refund(ctx, 1, "fr1", "r1", _chargeAt, idsFrom(2000))
				return err
			},
		},
		"refund retry": {
			setup: func(t *testing.T, db *gorm.DB, repo ChargeRepository) {
				setLimits(t, db, "o1", 10, nil)
				require.NoError(t, charge(newCharge("r1", "u1", 5, _chargeAt))(repo))
				_, err := repo.Refund(ctx, 1, "fr1", "r1", _chargeAt, idsFrom(2000))
				require.NoError(t, err)
			},
			act: func(repo ChargeRepository) error {
				_, err := repo.Refund(ctx, 1, "fr2", "r1", _chargeAt, idsFrom(3000))
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
					c := newCharge(fmt.Sprintf("r%d", i), fmt.Sprintf("u%d", i%4+1), 1, _chargeAt)
					rs, err := repo.Charge(context.Background(), c, next)
					if assert.NoError(t, err) && rs.Entry.EntryType == model.EntryTypeSpend {
						charged.Add(1)
					}
				}()
			}
			wg.Wait()
			assert.Equal(t, int64(35), charged.Load())

			var orgUsed int64
			require.NoError(t, db.Model(&model.CreditBucket{}).Where("bucket_type = ?", model.BucketTypeMonthly).
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
