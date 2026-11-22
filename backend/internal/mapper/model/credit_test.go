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
	m := model.CreditBucket{ID: 4, ServiceID: 1, OrgID: "o", UserID: "u", Kind: "grant", Amount: 10, Remaining: 7, CreatedAt: now, ExpiresAt: later}
	e := entity.CreditBucket{ID: 4, ServiceID: 1, OrgID: "o", UserID: "u", Kind: entity.BucketKindGrant, Amount: 10, Remaining: 7, CreatedAt: now, ExpiresAt: later}
	assert.Equal(t, e, FromCreditBucketModelToCreditBucketEntity(m))
	assert.Equal(t, m, FromCreditBucketEntityToCreditBucketModel(e))
}

func TestChargeMappers(t *testing.T) {
	now := time.Now()
	m := model.Charge{ID: 5, ServiceID: 1, RequestID: "r", OrgID: "o", UserID: "u", Credits: 1, Operation: "tool", CreatedAt: now, RefundedAt: &now}
	e := entity.Charge{ID: 5, ServiceID: 1, RequestID: "r", OrgID: "o", UserID: "u", Credits: 1, Operation: "tool", CreatedAt: now, RefundedAt: &now}
	assert.Equal(t, e, FromChargeModelToChargeEntity(m))
	assert.Equal(t, m, FromChargeEntityToChargeModel(e))
}

func TestLedgerEntryMappers(t *testing.T) {
	now := time.Now()
	m := model.LedgerEntry{ID: 6, ServiceID: 1, OrgID: "o", UserID: "u", Kind: "spend", Amount: -1, BalanceAfter: 9, Actor: "mesher", Operation: "tool", CreatedAt: now}
	e := entity.LedgerEntry{ID: 6, ServiceID: 1, OrgID: "o", UserID: "u", Kind: entity.LedgerKindSpend, Amount: -1, BalanceAfter: 9, Actor: "mesher", Operation: "tool", CreatedAt: now}
	assert.Equal(t, e, FromLedgerEntryModelToLedgerEntryEntity(m))
	assert.Equal(t, m, FromLedgerEntryEntityToLedgerEntryModel(e))
	assert.Equal(t, []entity.LedgerEntry{e}, FromLedgerEntryModelsToLedgerEntryEntities([]model.LedgerEntry{m}))
	assert.Empty(t, FromLedgerEntryModelsToLedgerEntryEntities(nil))
}
