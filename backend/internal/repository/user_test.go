// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	return db
}

func TestUserRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository(newDB(t))

	_, err := repo.FindUserByEmail(ctx, "a@b.c")
	assert.ErrorIs(t, err, ErrNotFound)

	created, err := repo.CreateUser(ctx, model.User{ID: 1, Email: "a@b.c", Name: "A"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), created.ID)

	_, err = repo.CreateUser(ctx, model.User{ID: 2, Email: "a@b.c"})
	assert.Error(t, err, "email is unique")

	byEmail, err := repo.FindUserByEmail(ctx, "a@b.c")
	require.NoError(t, err)
	assert.Equal(t, "A", byEmail.Name)

	byEmail.Name = "B"
	updated, err := repo.UpdateUser(ctx, byEmail)
	require.NoError(t, err)
	assert.Equal(t, "B", updated.Name)

	byID, err := repo.FindUserByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "B", byID.Name)

	_, err = repo.FindUserByID(ctx, 99)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUserRepository_DBErrors(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	repo := NewUserRepository(db)

	_, err = repo.FindUserByID(ctx, 1)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)

	_, err = repo.UpdateUser(ctx, model.User{ID: 1, Email: "x"})
	assert.Error(t, err)
}
