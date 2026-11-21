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
	for _, table := range []string{"users", "services", "service_api_keys", "org_limits", "credit_buckets", "charges", "ledger_entries"} {
		assert.True(t, db.Migrator().HasTable(table), table)
	}
	for table, index := range map[string]string{
		"charges":        "idx_charges_service_request",
		"org_limits":     "idx_org_limits_service_org",
		"credit_buckets": "idx_credit_buckets_fifo",
		"ledger_entries": "idx_ledger_entries_subject",
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
	b := CreditBucket{ID: 1, ServiceID: 1, OrgID: "o", UserID: "u", Kind: "monthly", Amount: 10, Remaining: 10, ExpiresAt: time.Now()}
	require.NoError(t, db.Create(&b).Error)
	assert.Error(t, db.Model(&b).Update("remaining", -1).Error)
}

func TestCharge_RequestIDUniquePerService(t *testing.T) {
	db := newDB(t)
	require.NoError(t, db.Create(&Charge{ID: 1, ServiceID: 1, RequestID: "r", OrgID: "o", UserID: "u", Credits: 1}).Error)
	assert.Error(t, db.Create(&Charge{ID: 2, ServiceID: 1, RequestID: "r", OrgID: "o", UserID: "u", Credits: 1}).Error, "a retry never charges twice")
	assert.NoError(t, db.Create(&Charge{ID: 3, ServiceID: 2, RequestID: "r", OrgID: "o", UserID: "u", Credits: 1}).Error)
}

func TestLedgerEntry_AppendOnly(t *testing.T) {
	db := newDB(t)
	e := LedgerEntry{ID: 1, ServiceID: 1, OrgID: "o", UserID: "u", Kind: "spend", Amount: -1, BalanceAfter: 9, Actor: "mesher"}
	require.NoError(t, db.Create(&e).Error)

	assert.ErrorIs(t, db.Model(&e).Update("amount", -2).Error, ErrLedgerAppendOnly)
	assert.ErrorIs(t, db.Save(&e).Error, ErrLedgerAppendOnly)
	assert.ErrorIs(t, db.Delete(&e).Error, ErrLedgerAppendOnly)

	var got LedgerEntry
	require.NoError(t, db.First(&got, 1).Error)
	assert.Equal(t, int64(-1), got.Amount)
}
