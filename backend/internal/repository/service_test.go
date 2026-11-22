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

func newServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newDB(t)
	require.NoError(t, db.AutoMigrate(model.Models()...))
	return db
}

func TestServiceRepository_FindOrCreateService(t *testing.T) {
	ctx := context.Background()
	repo := NewServiceRepository(newServiceDB(t))

	_, err := repo.FindServiceByName(ctx, "mesher")
	assert.ErrorIs(t, err, ErrNotFound)

	created, err := repo.FindOrCreateService(ctx, model.Service{ID: 1, Name: "mesher"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), created.ID)
	assert.False(t, created.Enabled)

	again, err := repo.FindOrCreateService(ctx, model.Service{ID: 2, Name: "mesher"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), again.ID, "an existing service is kept")

	found, err := repo.FindServiceByName(ctx, "mesher")
	require.NoError(t, err)
	assert.Equal(t, int64(1), found.ID)
}

func TestServiceRepository_SetServiceEnabled(t *testing.T) {
	ctx := context.Background()
	repo := NewServiceRepository(newServiceDB(t))
	_, err := repo.SetServiceEnabled(ctx, "mesher", true)
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = repo.FindOrCreateService(ctx, model.Service{ID: 1, Name: "mesher"})
	require.NoError(t, err)
	s, err := repo.SetServiceEnabled(ctx, "mesher", true)
	require.NoError(t, err)
	assert.True(t, s.Enabled)
	s, err = repo.SetServiceEnabled(ctx, "mesher", false)
	require.NoError(t, err)
	assert.False(t, s.Enabled, "false is written, not skipped as a zero value")
}

func TestServiceRepository_Keys(t *testing.T) {
	ctx := context.Background()
	repo := NewServiceRepository(newServiceDB(t))
	_, err := repo.FindOrCreateService(ctx, model.Service{ID: 1, Name: "mesher"})
	require.NoError(t, err)
	_, err = repo.FindOrCreateService(ctx, model.Service{ID: 2, Name: "tmphost"})
	require.NoError(t, err)

	k, err := repo.CreateServiceAPIKey(ctx, model.ServiceAPIKey{ID: 10, ServiceID: 1, Prefix: "tpk_a", KeyHash: "ha"})
	require.NoError(t, err)
	assert.Equal(t, int64(10), k.ID)
	_, err = repo.CreateServiceAPIKey(ctx, model.ServiceAPIKey{ID: 11, ServiceID: 1, Prefix: "tpk_a", KeyHash: "ha"})
	assert.Error(t, err, "hash is unique")
	_, err = repo.CreateServiceAPIKey(ctx, model.ServiceAPIKey{ID: 12, ServiceID: 2, Prefix: "tpk_b", KeyHash: "hb"})
	require.NoError(t, err)

	s, err := repo.FindServiceByActiveKeyHash(ctx, "ha")
	require.NoError(t, err)
	assert.Equal(t, "mesher", s.Name)
	s, err = repo.FindServiceByActiveKeyHash(ctx, "hb")
	require.NoError(t, err)
	assert.Equal(t, "tmphost", s.Name)
	_, err = repo.FindServiceByActiveKeyHash(ctx, "nope")
	assert.ErrorIs(t, err, ErrNotFound)

	assert.ErrorIs(t, repo.RevokeServiceAPIKey(ctx, 2, 10, time.Now()), ErrNotFound, "another service's key")
	assert.ErrorIs(t, repo.RevokeServiceAPIKey(ctx, 1, 99, time.Now()), ErrNotFound)

	first := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	require.NoError(t, repo.RevokeServiceAPIKey(ctx, 1, 10, first))
	require.NoError(t, repo.RevokeServiceAPIKey(ctx, 1, 10, first.Add(time.Hour)), "revoking twice is a no-op")
	_, err = repo.FindServiceByActiveKeyHash(ctx, "ha")
	assert.ErrorIs(t, err, ErrNotFound, "a revoked key no longer authenticates")
	_, err = repo.FindServiceByActiveKeyHash(ctx, "hb")
	assert.NoError(t, err, "other services' keys still work")

	var stored model.ServiceAPIKey
	require.NoError(t, repo.(*serviceRepository).db.First(&stored, 10).Error)
	require.NotNil(t, stored.RevokedAt)
	assert.True(t, first.Equal(*stored.RevokedAt), "the first revocation time is kept")
}

func TestServiceRepository_DBErrors(t *testing.T) {
	ctx := context.Background()
	db := newServiceDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	repo := NewServiceRepository(db)

	_, err = repo.FindServiceByName(ctx, "mesher")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)

	_, err = repo.FindOrCreateService(ctx, model.Service{ID: 1, Name: "mesher"})
	assert.Error(t, err)

	_, err = repo.SetServiceEnabled(ctx, "mesher", true)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)

	_, err = repo.CreateServiceAPIKey(ctx, model.ServiceAPIKey{ID: 1})
	assert.Error(t, err)

	err = repo.RevokeServiceAPIKey(ctx, 1, 1, time.Now())
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}
