// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(Models()...))
	return db
}

func TestModels_Migrate(t *testing.T) {
	db := newDB(t)
	for _, table := range []string{"users", "services", "service_api_keys", "external_id_mappings", "operation_mappings", "org_limits", "credit_buckets", "charge_spends", "ledger_entries"} {
		assert.True(t, db.Migrator().HasTable(table), table)
	}
	for index, table := range map[string]string{
		"idx_ledger_entries_service_request":       "ledger_entries",
		"idx_ledger_entries_refund_of":             "ledger_entries",
		"idx_org_limits_service_org":               "org_limits",
		"idx_external_id_mappings_subject":         "external_id_mappings",
		"idx_external_id_mappings_service_mapping": "external_id_mappings",
		"idx_operation_mappings_service_mapping":   "operation_mappings",
		"idx_credit_buckets_fifo":                  "credit_buckets",
		"idx_ledger_entries_subject":               "ledger_entries",
	} {
		assert.True(t, db.Migrator().HasIndex(table, index), index)
	}
	assert.NoError(t, db.AutoMigrate(Models()...), "migrating twice is a no-op")
}

func TestService_DisabledByDefault(t *testing.T) {
	db := newDB(t)
	require.NoError(t, db.Create(&Service{ID: 1, Name: "mesher"}).Error)
	var got Service
	require.NoError(t, db.First(&got, 1).Error)
	assert.False(t, got.Enabled)
	assert.Error(t, db.Create(&Service{ID: 2, Name: "mesher"}).Error, "name is unique")
}

func TestServiceAPIKey_HashIsUnique(t *testing.T) {
	db := newDB(t)
	require.NoError(t, db.Create(&ServiceAPIKey{ID: 1, ServiceID: 1, Prefix: "tpk_a", KeyHash: "h"}).Error)
	assert.Error(t, db.Create(&ServiceAPIKey{ID: 2, ServiceID: 1, Prefix: "tpk_a", KeyHash: "h"}).Error)
}

func TestOrgLimit_Defaults(t *testing.T) {
	db := newDB(t)
	require.NoError(t, db.Create(&OrgLimit{ID: 1, ServiceID: 1, OrgID: "o"}).Error)
	var got OrgLimit
	require.NoError(t, db.First(&got, 1).Error)
	assert.Equal(t, int64(0), got.MonthlyLimit)
	assert.Nil(t, got.UserLimit, "null user limit is uncapped")
	assert.Error(t, db.Create(&OrgLimit{ID: 2, ServiceID: 1, OrgID: "o", MonthlyLimit: 5}).Error, "one row per service and org")
	assert.NoError(t, db.Create(&OrgLimit{ID: 3, ServiceID: 2, OrgID: "o", MonthlyLimit: 5}).Error)
}

func TestCreditBucket_RemainingNeverNegative(t *testing.T) {
	db := newDB(t)
	b := CreditBucket{ID: 1, ServiceID: 1, OrgID: "o", UserID: "u", BucketType: BucketTypeMonthly, Amount: 10, Remaining: 10, ExpiresAt: time.Now()}
	require.NoError(t, db.Create(&b).Error)
	assert.Error(t, db.Model(&b).Update("remaining", -1).Error)
}

func TestLedgerEntry_RequestIDUniquePerService(t *testing.T) {
	db := newDB(t)
	r, r2 := "r", "r2"
	spend := int64(1)
	require.NoError(t, db.Create(&LedgerEntry{ID: 1, ServiceID: 1, RequestID: &r, EntryType: EntryTypeSpend, Amount: -1}).Error)
	assert.Error(t, db.Create(&LedgerEntry{ID: 2, ServiceID: 1, RequestID: &r, EntryType: EntryTypeSpend, Amount: -1}).Error, "a retry never charges twice")
	assert.NoError(t, db.Create(&LedgerEntry{ID: 3, ServiceID: 2, RequestID: &r, EntryType: EntryTypeSpend, Amount: -1}).Error, "another service's")
	assert.NoError(t, db.Create(&LedgerEntry{ID: 4, ServiceID: 1, EntryType: EntryTypeRenewal, Amount: 5}).Error)
	assert.NoError(t, db.Create(&LedgerEntry{ID: 5, ServiceID: 1, EntryType: EntryTypeRenewal, Amount: 5}).Error, "entries without a request")

	assert.NoError(t, db.Create(&LedgerEntry{ID: 6, ServiceID: 1, RequestID: &r2, RefundOf: &spend, EntryType: EntryTypeRefund, Amount: 1}).Error, "a refund has its own request")
	r3 := "r3"
	assert.Error(t, db.Create(&LedgerEntry{ID: 7, ServiceID: 1, RequestID: &r3, RefundOf: &spend, EntryType: EntryTypeRefund, Amount: 1}).Error, "a spend is refunded once")
}

func TestExternalIDMapping(t *testing.T) {
	db := newDB(t)
	m := ExternalIDMapping{ID: 1, ServiceID: 1, OrgID: "o", MappingID: 10}
	require.NoError(t, db.Create(&m).Error)
	assert.NoError(t, db.Create(&ExternalIDMapping{ID: 2, ServiceID: 1, OrgID: "o", UserID: "u", MappingID: 11}).Error, "an org's user")
	assert.Error(t, db.Create(&ExternalIDMapping{ID: 3, ServiceID: 1, OrgID: "o", MappingID: 12}).Error, "one row per org")
	assert.Error(t, db.Create(&ExternalIDMapping{ID: 4, ServiceID: 1, OrgID: "p", MappingID: 10}).Error, "one row per service and mapping ID")
	assert.NoError(t, db.Create(&ExternalIDMapping{ID: 5, ServiceID: 2, OrgID: "o", MappingID: 10}).Error, "another service's")

	assert.ErrorIs(t, db.Model(&m).Update("org_id", "p").Error, ErrMappingImmutable)
	assert.ErrorIs(t, db.Delete(&m).Error, ErrMappingImmutable)
}

func TestOperationMapping(t *testing.T) {
	db := newDB(t)
	m := OperationMapping{ID: 1, ServiceID: 1, Operation: "tool", MappingID: 10}
	require.NoError(t, db.Create(&m).Error)
	assert.Error(t, db.Create(&OperationMapping{ID: 2, ServiceID: 1, Operation: "other", MappingID: 10}).Error, "one row per service and mapping ID")
	assert.NoError(t, db.Create(&OperationMapping{ID: 3, ServiceID: 2, Operation: "tool", MappingID: 10}).Error)

	assert.ErrorIs(t, db.Model(&m).Update("operation", "x").Error, ErrMappingImmutable)
	assert.ErrorIs(t, db.Delete(&m).Error, ErrMappingImmutable)
}

func TestLedgerEntry_AppendOnly(t *testing.T) {
	db := newDB(t)
	e := LedgerEntry{ID: 1, ServiceID: 1, OrgID: "o", UserID: "u", EntryType: EntryTypeSpend, Amount: -1, BalanceAfter: 9, ActorType: ActorTypeService}
	require.NoError(t, db.Create(&e).Error)

	assert.ErrorIs(t, db.Model(&e).Update("amount", -2).Error, ErrLedgerAppendOnly)
	assert.ErrorIs(t, db.Save(&e).Error, ErrLedgerAppendOnly)
	assert.ErrorIs(t, db.Delete(&e).Error, ErrLedgerAppendOnly)

	var got LedgerEntry
	require.NoError(t, db.First(&got, 1).Error)
	assert.Equal(t, int64(-1), got.Amount)
}
