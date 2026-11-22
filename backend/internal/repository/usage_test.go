// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// newUsageDB returns a DB with the services mesher (1, with default limits
// 50 and 4) and tmphost (2), and mesher's usage this month:
//   - o1 has limits of its own (100, users uncapped); u1 spent 3 of its
//     monthly 100 and holds a 5 credit grant, u2 spent 10.
//   - o2 has no limits; u3 spent 1 of its monthly 4, and holds an expired
//     grant and last month's bucket.
//   - o3 has limits but no usage; tmphost's spending is not mesher's.
func newUsageDB(t *testing.T) (*gorm.DB, UsageRepository, model.Service) {
	t.Helper()
	ctx := context.Background()
	db, credits := newCreditDB(t)
	services := NewServiceRepository(db)
	_, err := services.FindOrCreateService(ctx, model.Service{ID: 2, Name: "tmphost"})
	require.NoError(t, err)
	four := int64(4)
	mesher, err := services.SetServiceDefaults(ctx, "mesher", 50, &four)
	require.NoError(t, err)
	for i, org := range []string{"o1", "o3"} {
		_, err = credits.UpsertOrgLimit(ctx, model.OrgLimit{ID: int64(90 + i), ServiceID: 1, OrgID: org, MonthlyLimit: 100})
		require.NoError(t, err)
	}
	month := MonthStart(_readAt)
	next := month.AddDate(0, 1, 0)
	tmphost := bucket(19, "o1", "u1", "monthly", 100, 0, month, next)
	tmphost.ServiceID = 2
	for _, b := range []model.CreditBucket{
		bucket(10, "o1", "u1", "monthly", 100, 97, month, next),
		bucket(11, "o1", "u1", "grant", 5, 5, _readAt.Add(-time.Hour), _readAt.AddDate(0, 1, 0)),
		bucket(12, "o1", "u2", "monthly", 100, 90, month, next),
		bucket(13, "o2", "u3", "monthly", 4, 3, month, next),
		bucket(14, "o2", "u3", "grant", 9, 0, month.AddDate(0, -1, 0), _readAt.Add(-time.Second)),
		bucket(15, "o2", "u3", "monthly", 4, 0, month.AddDate(0, -1, 0), month),
		tmphost,
	} {
		require.NoError(t, db.Create(&b).Error)
	}
	return db, NewUsageRepository(db), mesher
}

func TestUsageRepository_ListOrgUsage(t *testing.T) {
	ctx := context.Background()
	_, repo, mesher := newUsageDB(t)
	four := int64(4)

	orgs, err := repo.ListOrgUsage(ctx, mesher, _readAt)
	require.NoError(t, err)
	require.Len(t, orgs, 3)
	assert.Equal(t, OrgUsage{OrgLimit: orgs[0].OrgLimit, Custom: true, Used: 13}, orgs[0])
	assert.Equal(t, "o1", orgs[0].OrgLimit.OrgID)
	assert.Equal(t, int64(100), orgs[0].OrgLimit.MonthlyLimit)
	assert.Equal(t, OrgUsage{OrgLimit: model.OrgLimit{ServiceID: 1, OrgID: "o2", MonthlyLimit: 50, UserLimit: &four}, Used: 1}, orgs[1],
		"an org without limits gets the service's defaults")
	assert.Equal(t, "o3", orgs[2].OrgLimit.OrgID)
	assert.True(t, orgs[2].Custom)
	assert.Zero(t, orgs[2].Used)

	orgs, err = repo.ListOrgUsage(ctx, mesher, _readAt.AddDate(0, 1, 0))
	require.NoError(t, err)
	require.Len(t, orgs, 2, "next month only the orgs with limits are left")
	assert.Zero(t, orgs[0].Used)
}

func TestUsageRepository_ListUserUsage(t *testing.T) {
	ctx := context.Background()
	_, repo, mesher := newUsageDB(t)
	four := int64(4)

	users, err := repo.ListUserUsage(ctx, mesher, "", _readAt, 10)
	require.NoError(t, err)
	require.Len(t, users, 3)
	assert.Equal(t, "o1", users[0].OrgID)
	assert.Equal(t, "u1", users[0].UserID)
	assert.Equal(t, int64(3), users[0].MonthlyUsed)
	assert.Equal(t, int64(5), users[0].Grants)
	assert.Equal(t, int64(102), users[0].Balance)
	assert.Equal(t, int64(100), users[0].OrgLimit.MonthlyLimit)
	assert.Equal(t, "u2", users[1].UserID)
	assert.Equal(t, int64(10), users[1].MonthlyUsed)
	assert.Equal(t, UserUsage{
		OrgID: "o2", UserID: "u3", MonthlyUsed: 1, Grants: 0, Balance: 3,
		OrgLimit: model.OrgLimit{ServiceID: 1, OrgID: "o2", MonthlyLimit: 50, UserLimit: &four},
	}, users[2], "expired buckets and last month's spending don't count")

	users, err = repo.ListUserUsage(ctx, mesher, "o2", _readAt, 10)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "u3", users[0].UserID)

	users, err = repo.ListUserUsage(ctx, mesher, "", _readAt, 2)
	require.NoError(t, err)
	assert.Len(t, users, 2, "limit caps the list")
}

func TestUsageRepository_DBErrors(t *testing.T) {
	ctx := context.Background()
	// Each query fails in turn, so every error return is reached.
	for _, tc := range []struct {
		kind string
		call func(UsageRepository, model.Service) error
	}{
		{"query", func(r UsageRepository, s model.Service) error { _, err := r.ListOrgUsage(ctx, s, _readAt); return err }},
		{"row", func(r UsageRepository, s model.Service) error { _, err := r.ListOrgUsage(ctx, s, _readAt); return err }},
		{"row", func(r UsageRepository, s model.Service) error {
			_, err := r.ListUserUsage(ctx, s, "", _readAt, 10)
			return err
		}},
		{"query", func(r UsageRepository, s model.Service) error {
			_, err := r.ListUserUsage(ctx, s, "", _readAt, 10)
			return err
		}},
	} {
		db, repo, mesher := newUsageDB(t)
		failNth(t, db, tc.kind, 1)
		assert.Error(t, tc.call(repo, mesher), tc.kind)
	}
}
