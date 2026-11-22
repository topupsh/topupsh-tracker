// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// FromServiceModelToServiceEntity maps a stored service to the domain service.
func FromServiceModelToServiceEntity(m model.Service) entity.Service {
	return entity.Service{ID: m.ID, Name: m.Name, Enabled: m.Enabled, CreatedAt: m.CreatedAt}
}

// FromServiceEntityToServiceModel maps a domain service to its stored form.
func FromServiceEntityToServiceModel(e entity.Service) model.Service {
	return model.Service{ID: e.ID, Name: e.Name, Enabled: e.Enabled, CreatedAt: e.CreatedAt}
}

// FromServiceAPIKeyModelToServiceAPIKeyEntity maps a stored key to the domain key.
func FromServiceAPIKeyModelToServiceAPIKeyEntity(m model.ServiceAPIKey) entity.ServiceAPIKey {
	return entity.ServiceAPIKey{
		ID: m.ID, ServiceID: m.ServiceID, Prefix: m.Prefix, KeyHash: m.KeyHash,
		CreatedAt: m.CreatedAt, RevokedAt: m.RevokedAt,
	}
}

// FromServiceAPIKeyEntityToServiceAPIKeyModel maps a domain key to its stored form.
func FromServiceAPIKeyEntityToServiceAPIKeyModel(e entity.ServiceAPIKey) model.ServiceAPIKey {
	return model.ServiceAPIKey{
		ID: e.ID, ServiceID: e.ServiceID, Prefix: e.Prefix, KeyHash: e.KeyHash,
		CreatedAt: e.CreatedAt, RevokedAt: e.RevokedAt,
	}
}

// FromOrgLimitModelToOrgLimitEntity maps stored org limits to the domain limits.
func FromOrgLimitModelToOrgLimitEntity(m model.OrgLimit) entity.OrgLimit {
	return entity.OrgLimit{
		ID: m.ID, ServiceID: m.ServiceID, OrgID: m.OrgID,
		MonthlyLimit: m.MonthlyLimit, UserLimit: m.UserLimit,
	}
}

// FromOrgLimitEntityToOrgLimitModel maps domain org limits to their stored form.
func FromOrgLimitEntityToOrgLimitModel(e entity.OrgLimit) model.OrgLimit {
	return model.OrgLimit{
		ID: e.ID, ServiceID: e.ServiceID, OrgID: e.OrgID,
		MonthlyLimit: e.MonthlyLimit, UserLimit: e.UserLimit,
	}
}

// FromCreditBucketModelToCreditBucketEntity maps a stored bucket to the domain bucket.
func FromCreditBucketModelToCreditBucketEntity(m model.CreditBucket) entity.CreditBucket {
	return entity.CreditBucket{
		ID: m.ID, ServiceID: m.ServiceID, OrgID: m.OrgID, UserID: m.UserID,
		Kind: entity.BucketKind(m.Kind), Amount: m.Amount, Remaining: m.Remaining,
		CreatedAt: m.CreatedAt, ExpiresAt: m.ExpiresAt,
	}
}

// FromCreditBucketEntityToCreditBucketModel maps a domain bucket to its stored form.
func FromCreditBucketEntityToCreditBucketModel(e entity.CreditBucket) model.CreditBucket {
	return model.CreditBucket{
		ID: e.ID, ServiceID: e.ServiceID, OrgID: e.OrgID, UserID: e.UserID,
		Kind: string(e.Kind), Amount: e.Amount, Remaining: e.Remaining,
		CreatedAt: e.CreatedAt, ExpiresAt: e.ExpiresAt,
	}
}

// FromChargeModelToChargeEntity maps a stored charge to the domain charge.
func FromChargeModelToChargeEntity(m model.Charge) entity.Charge {
	return entity.Charge{
		ID: m.ID, ServiceID: m.ServiceID, RequestID: m.RequestID, OrgID: m.OrgID,
		UserID: m.UserID, Credits: m.Credits, Operation: m.Operation,
		CreatedAt: m.CreatedAt, RefundedAt: m.RefundedAt,
	}
}

// FromChargeEntityToChargeModel maps a domain charge to its stored form.
func FromChargeEntityToChargeModel(e entity.Charge) model.Charge {
	return model.Charge{
		ID: e.ID, ServiceID: e.ServiceID, RequestID: e.RequestID, OrgID: e.OrgID,
		UserID: e.UserID, Credits: e.Credits, Operation: e.Operation,
		CreatedAt: e.CreatedAt, RefundedAt: e.RefundedAt,
	}
}

// FromLedgerEntryModelToLedgerEntryEntity maps a stored ledger entry to the domain entry.
func FromLedgerEntryModelToLedgerEntryEntity(m model.LedgerEntry) entity.LedgerEntry {
	return entity.LedgerEntry{
		ID: m.ID, ServiceID: m.ServiceID, OrgID: m.OrgID, UserID: m.UserID,
		Kind: entity.LedgerKind(m.Kind), Amount: m.Amount, BalanceAfter: m.BalanceAfter,
		Actor: m.Actor, Operation: m.Operation, CreatedAt: m.CreatedAt,
	}
}

// FromLedgerEntryModelsToLedgerEntryEntities maps stored ledger entries to domain entries.
func FromLedgerEntryModelsToLedgerEntryEntities(ms []model.LedgerEntry) []entity.LedgerEntry {
	es := make([]entity.LedgerEntry, len(ms))
	for i, m := range ms {
		es[i] = FromLedgerEntryModelToLedgerEntryEntity(m)
	}
	return es
}

// FromLedgerEntryEntityToLedgerEntryModel maps a domain ledger entry to its stored form.
func FromLedgerEntryEntityToLedgerEntryModel(e entity.LedgerEntry) model.LedgerEntry {
	return model.LedgerEntry{
		ID: e.ID, ServiceID: e.ServiceID, OrgID: e.OrgID, UserID: e.UserID,
		Kind: string(e.Kind), Amount: e.Amount, BalanceAfter: e.BalanceAfter,
		Actor: e.Actor, Operation: e.Operation, CreatedAt: e.CreatedAt,
	}
}
