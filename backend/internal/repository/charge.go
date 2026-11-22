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

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

var (
	// ErrRequestConflict is returned when a request ID is reused for a
	// different org, user or number of credits.
	ErrRequestConflict = errors.New("request id was used for another charge")
	// ErrNotCharged is returned when refunding a refused charge.
	ErrNotCharged = errors.New("charge was refused")
)

type (
	// ChargeRepository charges and refunds credits, each in one
	// transaction with its ledger entries.
	ChargeRepository interface {
		// Charge spends c.Credits of c's user, FIFO across the user's
		// unexpired buckets, if both the org limit and the user's credits
		// allow it. It stores c with its outcome; when c.RequestID was
		// charged before, it returns that charge instead. While the
		// service's switch is off it stores nothing and returns
		// Limited false. It returns ErrNotFound for an unknown service
		// and ErrRequestConflict for a reused request ID.
		Charge(ctx context.Context, c model.Charge, nextID func() int64) (ChargeResult, error)
		// Refund gives the charge of requestID back to the buckets it
		// came from, once, at at. Buckets expired by then get nothing
		// back. It returns ErrNotFound for an unknown service or request
		// (except while the switch is off, when an unknown request is a
		// no-op) and ErrNotCharged for a refused charge.
		Refund(ctx context.Context, serviceID int64, requestID string, at time.Time, nextID func() int64) (RefundResult, error)
	}

	// ChargeResult is a charge and its outcome.
	ChargeResult struct {
		// Limited is false while the service's switch is off: nothing was
		// spent or stored.
		Limited bool
		Charge  model.Charge
	}

	// RefundResult is a refunded charge and the user's balance after it.
	RefundResult struct {
		// Limited is false for an unknown request while the service's
		// switch is off.
		Limited bool
		Charge  model.Charge
		Balance int64
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

func (r *chargeRepository) Charge(ctx context.Context, c model.Charge, nextID func() int64) (ChargeResult, error) {
	var rs ChargeResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		s, err := lockService(tx, c.ServiceID)
		if err != nil || !s.Enabled {
			return err
		}
		rs.Limited = true
		var prev model.Charge
		err = tx.Where("service_id = ? AND request_id = ?", c.ServiceID, c.RequestID).First(&prev).Error
		if err == nil {
			if prev.OrgID != c.OrgID || prev.UserID != c.UserID || prev.Credits != c.Credits {
				return ErrRequestConflict
			}
			rs.Charge = prev
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		l := ledger{tx: tx, service: s, charge: c, nextID: nextID}
		if err := l.spend(); err != nil {
			return err
		}
		rs.Charge = l.charge
		return nil
	})
	if err != nil {
		return ChargeResult{}, err
	}
	return rs, nil
}

func (r *chargeRepository) Refund(ctx context.Context, serviceID int64, requestID string, at time.Time, nextID func() int64) (RefundResult, error) {
	var rs RefundResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		s, err := lockService(tx, serviceID)
		if err != nil {
			return err
		}
		var c model.Charge
		err = tx.Where("service_id = ? AND request_id = ?", serviceID, requestID).First(&c).Error
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
		if c.Refusal != "" {
			return ErrNotCharged
		}
		l := ledger{tx: tx, service: s, charge: c, nextID: nextID}
		if c.RefundedAt == nil {
			if err := l.refund(at); err != nil {
				return err
			}
		}
		rs.Charge = l.charge
		rs.Balance, err = l.balance(at)
		return err
	})
	if err != nil {
		return RefundResult{}, err
	}
	return rs, nil
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
// service lock, writing a ledger entry for every change.
type ledger struct {
	tx      *gorm.DB
	service model.Service
	charge  model.Charge
	nextID  func() int64
}

func (l *ledger) spend() error {
	c := &l.charge
	now := c.CreatedAt
	monthly, orgLeft, err := l.monthlyBucket(now)
	if err != nil {
		return err
	}
	var buckets []model.CreditBucket
	if err := l.subject(l.tx.Model(&model.CreditBucket{})).
		Where("expires_at > ? AND remaining > 0", now).
		Order("created_at, id").Find(&buckets).Error; err != nil {
		return err
	}
	// Grant credits don't count against the org limit; monthly credits
	// must fit in what is left of it. The org is checked first.
	var grants int64
	for _, b := range buckets {
		if b.Kind == string(entity.BucketKindGrant) {
			grants += b.Remaining
		}
	}
	switch {
	case grants+orgLeft < c.Credits:
		c.Refusal = string(entity.RefusalOrgLimit)
	case grants+min(monthly.Remaining, orgLeft) < c.Credits:
		c.Refusal = string(entity.RefusalUserLimit)
	}
	if c.Refusal != "" {
		c.OrgRemaining = orgLeft
		if c.Balance, err = l.balance(now); err != nil {
			return err
		}
		return l.tx.Create(c).Error
	}

	need := c.Credits
	var spends []model.ChargeSpend
	for _, b := range buckets {
		take := min(b.Remaining, need)
		if b.Kind == string(entity.BucketKindMonthly) {
			take = min(take, orgLeft)
			orgLeft -= take
		}
		if take == 0 {
			continue
		}
		if err := l.tx.Model(&model.CreditBucket{}).Where("id = ?", b.ID).
			Update("remaining", gorm.Expr("remaining - ?", take)).Error; err != nil {
			return err
		}
		spends = append(spends, model.ChargeSpend{ID: l.nextID(), ChargeID: c.ID, BucketID: b.ID, Credits: take})
		if need -= take; need == 0 {
			break
		}
	}
	c.OrgRemaining = orgLeft
	if err := l.tx.Create(&spends).Error; err != nil {
		return err
	}
	if c.Balance, err = l.record(entity.LedgerKindSpend, -c.Credits, now); err != nil {
		return err
	}
	return l.tx.Create(c).Error
}

func (l *ledger) refund(at time.Time) error {
	c := &l.charge
	var spends []model.ChargeSpend
	if err := l.tx.Where("charge_id = ?", c.ID).Find(&spends).Error; err != nil {
		return err
	}
	for _, sp := range spends {
		res := l.tx.Model(&model.CreditBucket{}).Where("id = ? AND expires_at > ?", sp.BucketID, at).
			Update("remaining", gorm.Expr("remaining + ?", sp.Credits))
		if res.Error != nil {
			return res.Error
		}
		c.RefundedCredits += sp.Credits * res.RowsAffected
	}
	c.RefundedAt = &at
	if err := l.tx.Model(c).Select("refunded_at", "refunded_credits").Updates(c).Error; err != nil {
		return err
	}
	if c.RefundedCredits == 0 {
		return nil
	}
	_, err := l.record(entity.LedgerKindRefund, c.RefundedCredits, at)
	return err
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
	err = l.subject(l.tx).Where("kind = ? AND created_at = ?", entity.BucketKindMonthly, start).First(&b).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		b = model.CreditBucket{
			ID: l.nextID(), CreatedAt: start, ServiceID: l.charge.ServiceID, OrgID: l.charge.OrgID, UserID: l.charge.UserID,
			Kind: string(entity.BucketKindMonthly), Amount: userCap, Remaining: userCap, ExpiresAt: start.AddDate(0, 1, 0),
		}
		if err := l.tx.Create(&b).Error; err != nil {
			return model.CreditBucket{}, 0, err
		}
		if userCap > 0 {
			if _, err := l.record(entity.LedgerKindRenewal, userCap, now); err != nil {
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
			if _, err := l.record(entity.LedgerKindLimit, delta, now); err != nil {
				return model.CreditBucket{}, 0, err
			}
		}
	}
	var orgUsed int64
	if err := l.tx.Model(&model.CreditBucket{}).
		Where("service_id = ? AND org_id = ? AND kind = ? AND created_at = ?", l.charge.ServiceID, l.charge.OrgID, entity.BucketKindMonthly, start).
		Select("COALESCE(SUM(amount - remaining), 0)").Scan(&orgUsed).Error; err != nil {
		return model.CreditBucket{}, 0, err
	}
	return b, max(orgLimit-orgUsed, 0), nil
}

// limits returns the org's monthly limit and the monthly credits each of
// its users gets: the user limit, or the org limit when users are uncapped,
// and never more than the org limit.
func (l *ledger) limits() (int64, int64, error) {
	var ol model.OrgLimit
	err := l.tx.Where("service_id = ? AND org_id = ?", l.charge.ServiceID, l.charge.OrgID).First(&ol).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.DefaultOrgMonthlyLimit, entity.DefaultOrgMonthlyLimit, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if ol.UserLimit == nil {
		return ol.MonthlyLimit, ol.MonthlyLimit, nil
	}
	return ol.MonthlyLimit, min(*ol.UserLimit, ol.MonthlyLimit), nil
}

// record writes a ledger entry of amount and returns the balance after it.
func (l *ledger) record(kind entity.LedgerKind, amount int64, at time.Time) (int64, error) {
	balance, err := l.balance(at)
	if err != nil {
		return 0, err
	}
	c := l.charge
	return balance, l.tx.Create(&model.LedgerEntry{
		ID: l.nextID(), CreatedAt: at, ServiceID: c.ServiceID, OrgID: c.OrgID, UserID: c.UserID,
		Kind: string(kind), Amount: amount, BalanceAfter: balance, Actor: l.service.Name, Operation: c.Operation,
	}).Error
}

// balance is the user's unexpired credits at at.
func (l *ledger) balance(at time.Time) (int64, error) {
	var balance int64
	err := l.subject(l.tx.Model(&model.CreditBucket{})).Where("expires_at > ?", at).
		Select("COALESCE(SUM(remaining), 0)").Scan(&balance).Error
	return balance, err
}

func (l *ledger) subject(q *gorm.DB) *gorm.DB {
	return q.Where("service_id = ? AND org_id = ? AND user_id = ?", l.charge.ServiceID, l.charge.OrgID, l.charge.UserID)
}
