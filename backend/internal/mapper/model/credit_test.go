// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

func TestServiceMappers(t *testing.T) {
	now := time.Now()
	m := model.Service{ID: 1, Name: "mesher", Enabled: true, CreatedAt: now}
	e := entity.Service{ID: 1, Name: "mesher", Enabled: true, CreatedAt: now}
	assert.Equal(t, e, FromServiceModelToServiceEntity(m))
	assert.Equal(t, m, FromServiceEntityToServiceModel(e))
}

func TestServiceAPIKeyMappers(t *testing.T) {
	now := time.Now()
	m := model.ServiceAPIKey{ID: 2, ServiceID: 1, Prefix: "tpk_ab", KeyHash: "h", CreatedAt: now, RevokedAt: &now}
	e := entity.ServiceAPIKey{ID: 2, ServiceID: 1, Prefix: "tpk_ab", KeyHash: "h", CreatedAt: now, RevokedAt: &now}
	assert.Equal(t, e, FromServiceAPIKeyModelToServiceAPIKeyEntity(m))
	assert.Equal(t, m, FromServiceAPIKeyEntityToServiceAPIKeyModel(e))
}

func TestOrgLimitMappers(t *testing.T) {
	userLimit := int64(50)
	m := model.OrgLimit{ID: 3, ServiceID: 1, OrgID: "o", MonthlyLimit: 500, UserLimit: &userLimit}
	e := entity.OrgLimit{ID: 3, ServiceID: 1, OrgID: "o", MonthlyLimit: 500, UserLimit: &userLimit}
	assert.Equal(t, e, FromOrgLimitModelToOrgLimitEntity(m))
	assert.Equal(t, m, FromOrgLimitEntityToOrgLimitModel(e))
}

func TestCreditBucketMappers(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)
	m := model.CreditBucket{ID: 4, ServiceID: 1, OrgID: "o", UserID: "u", BucketType: model.BucketTypeGrant, Amount: 10, Remaining: 7, CreatedAt: now, ExpiresAt: later}
	e := entity.CreditBucket{ID: 4, ServiceID: 1, OrgID: "o", UserID: "u", Kind: entity.BucketKindGrant, Amount: 10, Remaining: 7, CreatedAt: now, ExpiresAt: later}
	assert.Equal(t, e, FromCreditBucketModelToCreditBucketEntity(m))
	assert.Equal(t, m, FromCreditBucketEntityToCreditBucketModel(e))
}

func TestChargeMappers(t *testing.T) {
	now := time.Now()
	r, x := "r", "x"
	spend := model.LedgerEntry{ID: 5, ServiceID: 1, RequestID: &x, OrgID: "o", UserID: "u", EntryType: model.EntryTypeSpend, Amount: -2, BalanceAfter: 3, OrgRemaining: 4, Operation: "tool", CreatedAt: now}
	assert.Equal(t, entity.Charge{ID: 5, ServiceID: 1, RequestID: "x", OrgID: "o", UserID: "u", Credits: 2, Operation: "tool", Balance: 3, OrgRemaining: 4, CreatedAt: now},
		FromLedgerEntryModelToChargeEntity(spend))
	refusal := model.LedgerEntry{ID: 6, ServiceID: 1, RequestID: &x, EntryType: model.EntryTypeRefusal, RefusalReason: model.RefusalReasonUserLimit, BalanceAfter: 1}
	assert.Equal(t, entity.Charge{ID: 6, ServiceID: 1, RequestID: "x", Refusal: entity.RefusalUserLimit, Balance: 1}, FromLedgerEntryModelToChargeEntity(refusal))
	assert.Equal(t, entity.RefusalOrgLimit, FromLedgerEntryModelToChargeEntity(model.LedgerEntry{RefusalReason: model.RefusalReasonOrgLimit}).Refusal)

	refund := model.LedgerEntry{ID: 7, ServiceID: 1, RequestID: &r, RefundOf: &spend.ID, OrgID: "o", UserID: "u", EntryType: model.EntryTypeRefund, Amount: 2, BalanceAfter: 5, OrgRemaining: 6, Operation: "tool", CreatedAt: now}
	assert.Equal(t, entity.Refund{ID: 7, ServiceID: 1, RequestID: "r", ChargeRequestID: "x", OrgID: "o", UserID: "u", Credits: 2, Operation: "tool", Balance: 5, OrgRemaining: 6, CreatedAt: now},
		FromLedgerEntryModelToRefundEntity(refund, "x"))
}

func TestLedgerEntryMappers(t *testing.T) {
	now := time.Now()
	r := "r"
	m := model.LedgerEntry{ID: 6, ServiceID: 1, RequestID: &r, OrgID: "o", UserID: "u", EntryType: model.EntryTypeSpend, Amount: -1, BalanceAfter: 9, ActorType: model.ActorTypeService, Operation: "tool", CreatedAt: now}
	e := entity.LedgerEntry{ID: 6, ServiceID: 1, RequestID: "r", OrgID: "o", UserID: "u", Kind: entity.LedgerKindSpend, Amount: -1, BalanceAfter: 9, Actor: entity.ActorKindService, Operation: "tool", CreatedAt: now}
	assert.Equal(t, e, FromLedgerEntryModelToLedgerEntryEntity(m))
	assert.Equal(t, m, FromLedgerEntryEntityToLedgerEntryModel(e))
	m.RequestID, e.RequestID = nil, ""
	assert.Equal(t, e, FromLedgerEntryModelToLedgerEntryEntity(m), "a system entry has no request")
	assert.Equal(t, m, FromLedgerEntryEntityToLedgerEntryModel(e))
	assert.Equal(t, []entity.LedgerEntry{e}, FromLedgerEntryModelsToLedgerEntryEntities([]model.LedgerEntry{m}))
	assert.Empty(t, FromLedgerEntryModelsToLedgerEntryEntities(nil))
}

func TestTypeMappers(t *testing.T) {
	for typ, kind := range map[model.BucketType]entity.BucketKind{model.BucketTypeMonthly: "monthly", model.BucketTypeGrant: "grant"} {
		assert.Equal(t, kind, FromBucketTypeModelToBucketKindEntity(typ))
		assert.Equal(t, typ, FromBucketKindEntityToBucketTypeModel(kind))
	}
	assert.Empty(t, FromBucketTypeModelToBucketKindEntity(0))
	assert.Zero(t, FromBucketKindEntityToBucketTypeModel("nope"))

	for typ, kind := range map[model.EntryType]entity.LedgerKind{
		model.EntryTypeSpend: "spend", model.EntryTypeRefund: "refund", model.EntryTypeGrant: "grant",
		model.EntryTypeRenewal: "renewal", model.EntryTypeLimit: "limit", model.EntryTypeExpiry: "expiry",
	} {
		assert.Equal(t, kind, FromEntryTypeModelToLedgerKindEntity(typ))
		assert.Equal(t, typ, FromLedgerKindEntityToEntryTypeModel(kind))
	}
	assert.Empty(t, FromEntryTypeModelToLedgerKindEntity(0))
	assert.Zero(t, FromLedgerKindEntityToEntryTypeModel("nope"))

	for typ, kind := range map[model.ActorType]entity.ActorKind{
		model.ActorTypeService: "service", model.ActorTypeAdmin: "admin", model.ActorTypeScheduler: "scheduler",
	} {
		assert.Equal(t, kind, FromActorTypeModelToActorKindEntity(typ))
		assert.Equal(t, typ, FromActorKindEntityToActorTypeModel(kind))
	}
	assert.Empty(t, FromActorTypeModelToActorKindEntity(0))
	assert.Zero(t, FromActorKindEntityToActorTypeModel("nope"))
}
