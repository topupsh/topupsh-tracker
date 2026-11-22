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
	return entity.Service{
		ID: m.ID, Name: m.Name, Enabled: m.Enabled, CreatedAt: m.CreatedAt,
		DefaultOrgLimit: m.DefaultOrgLimit, DefaultUserLimit: m.DefaultUserLimit,
	}
}

// FromServiceEntityToServiceModel maps a domain service to its stored form.
func FromServiceEntityToServiceModel(e entity.Service) model.Service {
	return model.Service{
		ID: e.ID, Name: e.Name, Enabled: e.Enabled, CreatedAt: e.CreatedAt,
		DefaultOrgLimit: e.DefaultOrgLimit, DefaultUserLimit: e.DefaultUserLimit,
	}
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
		Kind: FromBucketTypeModelToBucketKindEntity(m.BucketType), Amount: m.Amount, Remaining: m.Remaining,
		CreatedAt: m.CreatedAt, ExpiresAt: m.ExpiresAt,
	}
}

// FromCreditBucketEntityToCreditBucketModel maps a domain bucket to its stored form.
func FromCreditBucketEntityToCreditBucketModel(e entity.CreditBucket) model.CreditBucket {
	return model.CreditBucket{
		ID: e.ID, ServiceID: e.ServiceID, OrgID: e.OrgID, UserID: e.UserID,
		BucketType: FromBucketKindEntityToBucketTypeModel(e.Kind), Amount: e.Amount, Remaining: e.Remaining,
		CreatedAt: e.CreatedAt, ExpiresAt: e.ExpiresAt,
	}
}

// FromLedgerEntryModelToChargeEntity maps a stored spend or refusal entry
// to the domain charge.
func FromLedgerEntryModelToChargeEntity(m model.LedgerEntry) entity.Charge {
	return entity.Charge{
		ID: m.ID, ServiceID: m.ServiceID, RequestID: fromRequestID(m.RequestID), OrgID: m.OrgID,
		UserID: m.UserID, Credits: -m.Amount, Operation: m.Operation,
		Refusal: _refusals[m.RefusalReason], Balance: m.BalanceAfter, OrgRemaining: m.OrgRemaining,
		CreatedAt: m.CreatedAt,
	}
}

// FromLedgerEntryModelToRefundEntity maps a stored refund entry of the
// charge of chargeRequestID to the domain refund.
func FromLedgerEntryModelToRefundEntity(m model.LedgerEntry, chargeRequestID string) entity.Refund {
	return entity.Refund{
		ID: m.ID, ServiceID: m.ServiceID, RequestID: fromRequestID(m.RequestID), ChargeRequestID: chargeRequestID,
		OrgID: m.OrgID, UserID: m.UserID, Credits: m.Amount, Operation: m.Operation,
		Balance: m.BalanceAfter, OrgRemaining: m.OrgRemaining, CreatedAt: m.CreatedAt,
	}
}

// FromLedgerEntryModelToLedgerEntryEntity maps a stored ledger entry to the domain entry.
func FromLedgerEntryModelToLedgerEntryEntity(m model.LedgerEntry) entity.LedgerEntry {
	return entity.LedgerEntry{
		ID: m.ID, ServiceID: m.ServiceID, RequestID: fromRequestID(m.RequestID), OrgID: m.OrgID, UserID: m.UserID,
		Kind: FromEntryTypeModelToLedgerKindEntity(m.EntryType), Amount: m.Amount, BalanceAfter: m.BalanceAfter,
		Actor: FromActorTypeModelToActorKindEntity(m.ActorType), Operation: m.Operation, CreatedAt: m.CreatedAt,
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
		ID: e.ID, ServiceID: e.ServiceID, RequestID: toRequestID(e.RequestID), OrgID: e.OrgID, UserID: e.UserID,
		EntryType: FromLedgerKindEntityToEntryTypeModel(e.Kind), Amount: e.Amount, BalanceAfter: e.BalanceAfter,
		ActorType: FromActorKindEntityToActorTypeModel(e.Actor), Operation: e.Operation, CreatedAt: e.CreatedAt,
	}
}

var (
	_bucketKinds = map[model.BucketType]entity.BucketKind{
		model.BucketTypeMonthly: entity.BucketKindMonthly,
		model.BucketTypeGrant:   entity.BucketKindGrant,
	}
	_ledgerKinds = map[model.EntryType]entity.LedgerKind{
		model.EntryTypeSpend:   entity.LedgerKindSpend,
		model.EntryTypeRefund:  entity.LedgerKindRefund,
		model.EntryTypeGrant:   entity.LedgerKindGrant,
		model.EntryTypeRenewal: entity.LedgerKindRenewal,
		model.EntryTypeLimit:   entity.LedgerKindLimit,
		model.EntryTypeExpiry:  entity.LedgerKindExpiry,
	}
	_actorKinds = map[model.ActorType]entity.ActorKind{
		model.ActorTypeService:   entity.ActorKindService,
		model.ActorTypeAdmin:     entity.ActorKindAdmin,
		model.ActorTypeScheduler: entity.ActorKindScheduler,
	}
	_refusals = map[model.RefusalReason]entity.Refusal{
		model.RefusalReasonOrgLimit:  entity.RefusalOrgLimit,
		model.RefusalReasonUserLimit: entity.RefusalUserLimit,
	}
)

// fromRequestID is a stored request ID; "" for none.
func fromRequestID(id *string) string {
	if id == nil {
		return ""
	}
	return *id
}

// toRequestID is the stored form of a request ID; nil for none.
func toRequestID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}

// FromBucketTypeModelToBucketKindEntity maps a stored bucket type to its
// kind; "" for an unknown type.
func FromBucketTypeModelToBucketKindEntity(t model.BucketType) entity.BucketKind {
	return _bucketKinds[t]
}

// FromBucketKindEntityToBucketTypeModel maps a bucket kind to its stored
// type; 0 for an unknown kind.
func FromBucketKindEntityToBucketTypeModel(k entity.BucketKind) model.BucketType {
	for t, kind := range _bucketKinds {
		if kind == k {
			return t
		}
	}
	return 0
}

// FromEntryTypeModelToLedgerKindEntity maps a stored ledger entry type to
// its kind; "" for an unknown type.
func FromEntryTypeModelToLedgerKindEntity(t model.EntryType) entity.LedgerKind {
	return _ledgerKinds[t]
}

// FromLedgerKindEntityToEntryTypeModel maps a ledger kind to its stored
// type; 0 for an unknown kind.
func FromLedgerKindEntityToEntryTypeModel(k entity.LedgerKind) model.EntryType {
	for t, kind := range _ledgerKinds {
		if kind == k {
			return t
		}
	}
	return 0
}

// FromActorTypeModelToActorKindEntity maps a stored actor type to its kind;
// "" for an unknown type.
func FromActorTypeModelToActorKindEntity(t model.ActorType) entity.ActorKind {
	return _actorKinds[t]
}

// FromActorKindEntityToActorTypeModel maps an actor kind to its stored
// type; 0 for an unknown kind.
func FromActorKindEntityToActorTypeModel(k entity.ActorKind) model.ActorType {
	for t, kind := range _actorKinds {
		if kind == k {
			return t
		}
	}
	return 0
}
