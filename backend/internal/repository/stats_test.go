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

var _statsNow = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

// seedStats stores three users.
func seedStats(t *testing.T, db *gorm.DB) {
	t.Helper()
	day := 24 * time.Hour
	east := time.FixedZone("east", 3*3600)
	require.NoError(t, db.Create(&[]model.User{
		{ID: 1, Email: "ann@example.com", Name: "Ann", CreatedAt: _statsNow.Add(-40 * day)},
		{ID: 2, Email: "bob@example.com", Name: "Bob_Smith", CreatedAt: _statsNow.Add(-2 * day)},
		// 2026-10-21 01:00 east is 2026-10-20 22:00 UTC.
		{ID: 3, Email: "cy@test.org", Name: "Cy", CreatedAt: time.Date(2026, 10, 21, 1, 0, 0, 0, east)},
	}).Error)
}

func TestStatsRepository(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedStats(t, db)
	repo := NewStatsRepository(db)
	since := _statsNow.Add(-30 * 24 * time.Hour)

	users, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), users)

	signups, err := repo.DailySignups(ctx, since)
	require.NoError(t, err)
	assert.Equal(t, []DayCount{{Day: "2026-10-19", Count: 1}, {Day: "2026-10-20", Count: 1}}, signups)
}

func TestStatsRepository_SearchUsers(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	seedStats(t, db)
	repo := NewStatsRepository(db)
	ids := func(us []model.User) []int64 {
		out := make([]int64, len(us))
		for i, u := range us {
			out[i] = u.ID
		}
		return out
	}

	us, err := repo.SearchUsers(ctx, "", 0, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 2, 1}, ids(us), "newest first")

	us, err = repo.SearchUsers(ctx, "", 0, 3, 1)
	require.NoError(t, err)
	assert.Equal(t, []int64{2}, ids(us), "below the cursor, limited")

	us, err = repo.SearchUsers(ctx, "EXAMPLE", 0, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{2, 1}, ids(us), "email, any case")

	us, err = repo.SearchUsers(ctx, "cy", 0, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{3}, ids(us), "name")

	us, err = repo.SearchUsers(ctx, "b_s", 0, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{2}, ids(us), "underscore is literal")

	us, err = repo.SearchUsers(ctx, "a_n", 0, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, us, "underscore is no wildcard")

	us, err = repo.SearchUsers(ctx, "zz", 1, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, ids(us), "by ID")
}

func TestStatsRepository_Errors(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	repo := NewStatsRepository(db)
	require.NoError(t, db.Migrator().DropTable(&model.User{}))

	_, err := repo.CountUsers(ctx)
	assert.Error(t, err)
	_, err = repo.DailySignups(ctx, _statsNow)
	assert.Error(t, err)
	_, err = repo.SearchUsers(ctx, "", 0, 0, 10)
	assert.Error(t, err)
}

func TestDayExpr(t *testing.T) {
	assert.Equal(t, "date(created_at)", dayExpr("sqlite", "created_at"))
	assert.Equal(t, "to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')", dayExpr("postgres", "created_at"))
}
