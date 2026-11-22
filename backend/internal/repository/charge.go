// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=charge.go -destination=mocks/charge.go -package=mocks

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

var (
	// ErrRequestConflict is returned when a request ID is reused for
	// another request: a charge of a different org, user or number of
	// credits, or a refund of another charge.
	ErrRequestConflict = errors.New("request id was used for another request")
	// ErrNotCharged is returned when refunding a refused charge.
	ErrNotCharged = errors.New("charge was refused")
)

type (
	// ChargeRepository charges and refunds credits, each in one
	// transaction with its ledger entries.
	ChargeRepository interface {
		// Charge spends -e.Amount credits of e's user, FIFO across the
		// user's unexpired buckets, if both the org limit and the user's
		// credits allow it. It stores the outcome as a spend or refusal
		// entry with e's RequestID; when that request was charged before,
		// it returns that entry instead. While the service's switch is off
		// it stores nothing and returns Limited false. It returns
		// ErrNotFound for an unknown service and ErrRequestConflict for a
		// reused request ID.
		Charge(ctx context.Context, e model.LedgerEntry, nextID func() int64) (ChargeResult, error)
		// Refund gives the spend of chargeRequestID back to the buckets it
		// came from, once, at at, as a refund entry with requestID; a retry
		// returns that entry, and so does refunding the spend again with
		// another request ID. Buckets expired by then get nothing back. It
		// returns ErrNotFound for an unknown service or charge (except
		// while the switch is off, when an unknown charge is a no-op),
		// ErrNotCharged for a refused charge and ErrRequestConflict for a
		// request ID used by another request.
		Refund(ctx context.Context, serviceID int64, requestID, chargeRequestID string, at time.Time, nextID func() int64) (RefundResult, error)
	}

	// ChargeResult is a charge's spend or refusal entry.
	ChargeResult struct {
		// Limited is false while the service's switch is off: nothing was
		// spent or stored.
		Limited bool
		Entry   model.LedgerEntry
	}

	// RefundResult is a charge's refund entry.
	RefundResult struct {
		// Limited is false for an unknown charge while the service's
		// switch is off.
		Limited bool
		Refund  model.LedgerEntry
	}

	chargeRepository struct {
		db *gorm.DB
	}
)

// NewChargeRepository returns a GORM-backed charge repository.
func NewChargeRepository(db *gorm.DB) ChargeRepository {
	return &chargeRepository{db: db}
}

// MonthStart is the start (00:00 UTC on the 1st) of t's month.
func MonthStart(t time.Time) time.Time {
	y, m, _ := t.UTC().Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func (r *chargeRepository) Charge(ctx context.Context, e model.LedgerEntry, nextID func() int64) (ChargeResult, error) {
	var rs ChargeResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		s, err := lockService(tx, e.ServiceID)
		if err != nil || !s.Enabled {
			return err
		}
		rs.Limited = true
		prev, err := findRequest(tx, e.ServiceID, *e.RequestID)
		if err == nil {
			// A refusal entry keeps no credits to compare.
			if prev.EntryType != model.EntryTypeSpend && prev.EntryType != model.EntryTypeRefusal ||
				prev.OrgMappingID != OrgMappingID(e.ServiceID, e.OrgID) || prev.UserMappingID != UserMappingID(e.ServiceID, e.OrgID, e.UserID) ||
				prev.EntryType == model.EntryTypeSpend && prev.Amount != e.Amount {
				return ErrRequestConflict
			}
			prev.OrgID, prev.UserID, prev.Operation = e.OrgID, e.UserID, e.Operation
			rs.Entry = prev
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if e.OrgMappingID, e.UserMappingID, err = mapUser(tx, nextID, e.ServiceID, e.OrgID, e.UserID); err != nil {
			return err
		}
		if e.OperationMappingID, err = mapOperation(tx, nextID, e.ServiceID, e.Operation); err != nil {
			return err
		}
		l := ledger{tx: tx, service: s, subject: e, nextID: nextID}
		rs.Entry, err = l.spend(-e.Amount)
		return err
	})
	if err != nil {
		return ChargeResult{}, err
	}
	return rs, nil
}

func (r *chargeRepository) Refund(ctx context.Context, serviceID int64, requestID, chargeRequestID string, at time.Time, nextID func() int64) (RefundResult, error) {
	var rs RefundResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		s, err := lockService(tx, serviceID)
		if err != nil {
			return err
		}
		c, err := findRequest(tx, serviceID, chargeRequestID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !s.Enabled {
				return nil
			}
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		rs.Limited = true
		switch c.EntryType {
		case model.EntryTypeSpend:
		case model.EntryTypeRefusal:
			return ErrNotCharged
		default:
			return ErrNotFound
		}
		ms, err := externalIDs(tx, c.ServiceID, []int64{c.UserMappingID})
		if err != nil {
			return err
		}
		c.OrgID, c.UserID = ms[c.UserMappingID].OrgID, ms[c.UserMappingID].UserID
		if c.Operation, err = operationName(tx, c.ServiceID, c.OperationMappingID); err != nil {
			return err
		}
		rs.Refund, err = findRequest(tx, serviceID, requestID)
		switch {
		case err == nil:
			if rs.Refund.RefundOf == nil || *rs.Refund.RefundOf != c.ID {
				return ErrRequestConflict
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		default:
			err = tx.Where("refund_of = ?", c.ID).First(&rs.Refund).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				l := ledger{tx: tx, service: s, subject: c, nextID: nextID}
				rs.Refund, err = l.refund(c.ID, requestID, at)
				return err
			}
			if err != nil {
				return err
			}
		}
		rs.Refund.OrgID, rs.Refund.UserID, rs.Refund.Operation = c.OrgID, c.UserID, c.Operation
		return nil
	})
	if err != nil {
		return RefundResult{}, err
	}
	return rs, nil
}

// findRequest finds the service's ledger entry of requestID.
func findRequest(tx *gorm.DB, serviceID int64, requestID string) (model.LedgerEntry, error) {
	var e model.LedgerEntry
	err := tx.Where("service_id = ? AND request_id = ?", serviceID, requestID).First(&e).Error
	return e, err
}

// lockService reads the service and locks its row until the transaction
// ends, which serializes the service's balance changes: concurrent charges
// can't overspend, and each ledger entry's balance follows the one before.
func lockService(tx *gorm.DB, id int64) (model.Service, error) {
	var s model.Service
	err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).First(&s, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Service{}, ErrNotFound
	}
	return s, err
}

// ledger changes one user's buckets inside a transaction that holds the
// service lock, writing a ledger entry for every change. subject holds the
// user, the operation and, for a charge, the request.
type ledger struct {
	tx      *gorm.DB
	service model.Service
	subject model.LedgerEntry
	nextID  func() int64
}

// spend charges credits, or refuses them, and returns the spend or
// refusal entry.
func (l *ledger) spend(credits int64) (model.LedgerEntry, error) {
	now := l.subject.CreatedAt
	monthly, orgLeft, err := l.monthlyBucket(now)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	var buckets []model.CreditBucket
	if err := l.where(l.tx.Model(&model.CreditBucket{})).
		Where("expires_at > ? AND remaining > 0", now).
		Order("created_at, id").Find(&buckets).Error; err != nil {
		return model.LedgerEntry{}, err
	}
	// Grant credits don't count against the org limit; monthly credits
	// must fit in what is left of it. The org is checked first.
	var grants int64
	for _, b := range buckets {
		if b.BucketType == model.BucketTypeGrant {
			grants += b.Remaining
		}
	}
	e := l.subject
	e.ID = l.nextID()
	switch {
	case grants+orgLeft < credits:
		e.RefusalReason = model.RefusalReasonOrgLimit
	case grants+min(monthly.Remaining, orgLeft) < credits:
		e.RefusalReason = model.RefusalReasonUserLimit
	}
	if e.RefusalReason != 0 {
		e.EntryType, e.Amount, e.OrgRemaining = model.EntryTypeRefusal, 0, orgLeft
		return l.write(e)
	}

	need := credits
	var spends []model.ChargeSpend
	for _, b := range buckets {
		take := min(b.Remaining, need)
		if b.BucketType == model.BucketTypeMonthly {
			take = min(take, orgLeft)
			orgLeft -= take
		}
		if take == 0 {
			continue
		}
		if err := l.tx.Model(&model.CreditBucket{}).Where("id = ?", b.ID).
			Update("remaining", gorm.Expr("remaining - ?", take)).Error; err != nil {
			return model.LedgerEntry{}, err
		}
		spends = append(spends, model.ChargeSpend{ID: l.nextID(), EntryID: e.ID, BucketID: b.ID, Credits: take})
		if need -= take; need == 0 {
			break
		}
	}
	if err := l.tx.Create(&spends).Error; err != nil {
		return model.LedgerEntry{}, err
	}
	e.EntryType, e.Amount, e.OrgRemaining = model.EntryTypeSpend, -credits, orgLeft
	return l.write(e)
}

// refund gives the spend entry spendID back to its buckets and writes the
// refund entry of requestID.
func (l *ledger) refund(spendID int64, requestID string, at time.Time) (model.LedgerEntry, error) {
	var spends []model.ChargeSpend
	if err := l.tx.Where("entry_id = ?", spendID).Find(&spends).Error; err != nil {
		return model.LedgerEntry{}, err
	}
	var credits int64
	for _, sp := range spends {
		res := l.tx.Model(&model.CreditBucket{}).Where("id = ? AND expires_at > ?", sp.BucketID, at).
			Update("remaining", gorm.Expr("remaining + ?", sp.Credits))
		if res.Error != nil {
			return model.LedgerEntry{}, res.Error
		}
		credits += sp.Credits * res.RowsAffected
	}
	orgLeft, err := l.orgRemaining(at)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	s := l.subject
	return l.write(model.LedgerEntry{
		ID: l.nextID(), CreatedAt: at, ServiceID: s.ServiceID, OrgMappingID: s.OrgMappingID, UserMappingID: s.UserMappingID,
		OrgID: s.OrgID, UserID: s.UserID, EntryType: model.EntryTypeRefund, RequestID: &requestID, RefundOf: &spendID,
		Amount: credits, OrgRemaining: orgLeft, OperationMappingID: s.OperationMappingID, Operation: s.Operation,
	})
}

// monthlyBucket returns the user's monthly bucket for now's month, sized
// to the org's current limits, and what is left of the org limit. It
// creates the bucket on the user's first charge of the month and resizes it
// after the admin changed the limits, keeping what the user already used.
func (l *ledger) monthlyBucket(now time.Time) (model.CreditBucket, int64, error) {
	start := MonthStart(now)
	orgLimit, userCap, err := l.limits()
	if err != nil {
		return model.CreditBucket{}, 0, err
	}
	var b model.CreditBucket
	err = l.where(l.tx).Where("bucket_type = ? AND created_at = ?", model.BucketTypeMonthly, start).First(&b).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		b = model.CreditBucket{
			ID: l.nextID(), CreatedAt: start, ServiceID: l.subject.ServiceID,
			OrgMappingID: l.subject.OrgMappingID, UserMappingID: l.subject.UserMappingID, OrgID: l.subject.OrgID, UserID: l.subject.UserID,
			BucketType: model.BucketTypeMonthly, Amount: userCap, Remaining: userCap, ExpiresAt: start.AddDate(0, 1, 0),
		}
		if err := l.tx.Create(&b).Error; err != nil {
			return model.CreditBucket{}, 0, err
		}
		if userCap > 0 {
			if _, err := l.record(model.EntryTypeRenewal, userCap, now); err != nil {
				return model.CreditBucket{}, 0, err
			}
		}
	case err != nil:
		return model.CreditBucket{}, 0, err
	default:
		// Amount stays at least what was used, so used = Amount - Remaining
		// survives a lowered limit.
		used := b.Amount - b.Remaining
		if amount := max(userCap, used); amount != b.Amount {
			delta := amount - b.Amount
			b.Amount, b.Remaining = amount, amount-used
			if err := l.tx.Model(&b).Select("amount", "remaining").Updates(&b).Error; err != nil {
				return model.CreditBucket{}, 0, err
			}
			if _, err := l.record(model.EntryTypeLimit, delta, now); err != nil {
				return model.CreditBucket{}, 0, err
			}
		}
	}
	orgLeft, err := l.orgLeft(orgLimit, start)
	return b, orgLeft, err
}

// orgRemaining is what is left of the org limit in at's month.
func (l *ledger) orgRemaining(at time.Time) (int64, error) {
	orgLimit, _, err := l.limits()
	if err != nil {
		return 0, err
	}
	return l.orgLeft(orgLimit, MonthStart(at))
}

// orgLeft is what is left of orgLimit after the org's monthly buckets
// of the month starting at start.
func (l *ledger) orgLeft(orgLimit int64, start time.Time) (int64, error) {
	var orgUsed int64
	if err := l.tx.Model(&model.CreditBucket{}).
		Where("service_id = ? AND org_mapping_id = ? AND bucket_type = ? AND created_at = ?", l.subject.ServiceID, l.subject.OrgMappingID, model.BucketTypeMonthly, start).
		Select("COALESCE(SUM(amount - remaining), 0)").Scan(&orgUsed).Error; err != nil {
		return 0, err
	}
	return max(orgLimit-orgUsed, 0), nil
}

// limits returns the org's monthly limit and the monthly credits each of
// its users gets: the user limit, or the org limit when users are uncapped,
// and never more than the org limit.
func (l *ledger) limits() (int64, int64, error) {
	ol, err := findOrgLimit(l.tx, l.service, l.subject.OrgID)
	if err != nil {
		return 0, 0, err
	}
	return ol.MonthlyLimit, MonthlyAllowance(ol), nil
}

// MonthlyAllowance is the monthly credits each user of the org gets: the
// user limit, or the org limit when users are uncapped, and never more
// than the org limit.
func MonthlyAllowance(ol model.OrgLimit) int64 {
	if ol.UserLimit == nil {
		return ol.MonthlyLimit
	}
	return min(*ol.UserLimit, ol.MonthlyLimit)
}

// record writes a system entry of amount, without the request, and
// returns the balance after it.
func (l *ledger) record(t model.EntryType, amount int64, at time.Time) (int64, error) {
	s := l.subject
	e, err := l.write(model.LedgerEntry{
		ID: l.nextID(), CreatedAt: at, ServiceID: s.ServiceID, OrgMappingID: s.OrgMappingID, UserMappingID: s.UserMappingID,
		OrgID: s.OrgID, UserID: s.UserID, EntryType: t, Amount: amount,
		OperationMappingID: s.OperationMappingID, Operation: s.Operation,
	})
	return e.BalanceAfter, err
}

// write stores e, by the service, with the user's balance after it.
func (l *ledger) write(e model.LedgerEntry) (model.LedgerEntry, error) {
	balance, err := l.balance(e.CreatedAt)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	e.BalanceAfter, e.ActorType = balance, model.ActorTypeService
	if err := l.tx.Create(&e).Error; err != nil {
		return model.LedgerEntry{}, err
	}
	return e, nil
}

// balance is the user's unexpired credits at at.
func (l *ledger) balance(at time.Time) (int64, error) {
	var balance int64
	err := l.where(l.tx.Model(&model.CreditBucket{})).Where("expires_at > ?", at).
		Select("COALESCE(SUM(remaining), 0)").Scan(&balance).Error
	return balance, err
}

// where narrows q to the user's rows.
func (l *ledger) where(q *gorm.DB) *gorm.DB {
	return q.Where("service_id = ? AND org_mapping_id = ? AND user_mapping_id = ?", l.subject.ServiceID, l.subject.OrgMappingID, l.subject.UserMappingID)
}
