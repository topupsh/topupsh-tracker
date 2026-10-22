// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mustafaturan/monoflake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	repomocks "github.com/topupsh/topupsh-tracker/backend/internal/repository/mocks"
)

var (
	// _now is 2026-10-21 15:30 UTC, so the stats start 2026-09-22.
	_now    = time.Date(2026, 10, 21, 15, 30, 0, 0, time.UTC)
	_since  = time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

type deps struct {
	users *repomocks.MockUserRepository
	stats *repomocks.MockStatsRepository
}

func newController(t *testing.T) (Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		users: repomocks.NewMockUserRepository(ctrl),
		stats: repomocks.NewMockStatsRepository(ctrl),
	}
	c := New(Params{Users: d.users, Stats: d.stats, Now: func() time.Time { return _now }})
	return c, d
}

func TestNew_Defaults(t *testing.T) {
	c := New(Params{}).(*controller)
	assert.WithinDuration(t, time.Now(), c.now(), time.Second)
}

func TestGetStats(t *testing.T) {
	ctx := context.Background()
	c, d := newController(t)
	d.stats.EXPECT().CountUsers(ctx).Return(int64(3), nil)
	d.stats.EXPECT().DailySignups(ctx, _since).Return([]repository.DayCount{
		{Day: "2026-09-21", Count: 99}, // before the range: ignored
		{Day: "2026-09-22", Count: 5},
		{Day: "2026-10-20", Count: 2},
	}, nil)

	rs, err := c.GetStats(ctx, &entity.GetAdminStatsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), rs.Users)
	assert.Equal(t, int64(7), rs.Signups)
	require.Len(t, rs.Days, StatsDays)
	assert.Equal(t, entity.AdminStatsDay{Date: "2026-09-22", Signups: 5}, rs.Days[0])
	assert.Equal(t, entity.AdminStatsDay{Date: "2026-10-20", Signups: 2}, rs.Days[28])
	assert.Equal(t, entity.AdminStatsDay{Date: "2026-10-21"}, rs.Days[29])
}

func TestGetStats_Errors(t *testing.T) {
	ctx := context.Background()
	t.Run("count", func(t *testing.T) {
		c, d := newController(t)
		d.stats.EXPECT().CountUsers(ctx).Return(int64(0), errBoom)
		_, err := c.GetStats(ctx, &entity.GetAdminStatsRequest{})
		assert.ErrorIs(t, err, errBoom)
	})
	t.Run("signups", func(t *testing.T) {
		c, d := newController(t)
		d.stats.EXPECT().CountUsers(ctx).Return(int64(0), nil)
		d.stats.EXPECT().DailySignups(ctx, _since).Return(nil, errBoom)
		_, err := c.GetStats(ctx, &entity.GetAdminStatsRequest{})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestListUsers(t *testing.T) {
	ctx := context.Background()
	c, d := newController(t)
	users := []model.User{{ID: 3, Email: "c@x"}, {ID: 2, Email: "b@x"}, {ID: 1}}
	d.stats.EXPECT().SearchUsers(ctx, "", int64(0), int64(9), 3).Return(users, nil)

	rs, err := c.ListUsers(ctx, &entity.ListAdminUsersRequest{Before: 9, Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(2), rs.Next)
	require.Len(t, rs.Users, 2)
	assert.Equal(t, int64(3), rs.Users[0].ID)
	assert.Equal(t, "b@x", rs.Users[1].Email)
}

func TestListUsers_Query(t *testing.T) {
	ctx := context.Background()
	id := monoflake.ID(4242)
	t.Run("ID", func(t *testing.T) {
		c, d := newController(t)
		d.stats.EXPECT().SearchUsers(ctx, id.String(), int64(4242), int64(0), DefaultUsersLimit+1).Return(nil, nil)
		rs, err := c.ListUsers(ctx, &entity.ListAdminUsersRequest{Query: "  " + id.String() + " "})
		require.NoError(t, err)
		assert.Empty(t, rs.Users)
		assert.Zero(t, rs.Next)
	})
	t.Run("email", func(t *testing.T) {
		c, d := newController(t)
		d.stats.EXPECT().SearchUsers(ctx, "a@b.c", int64(0), int64(0), MaxUsersLimit+1).Return(nil, nil)
		_, err := c.ListUsers(ctx, &entity.ListAdminUsersRequest{Query: "a@b.c", Limit: 10_000})
		require.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		c, d := newController(t)
		d.stats.EXPECT().SearchUsers(ctx, "", int64(0), int64(0), DefaultUsersLimit+1).Return(nil, errBoom)
		_, err := c.ListUsers(ctx, &entity.ListAdminUsersRequest{})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestQueryID(t *testing.T) {
	assert.Zero(t, queryID(""))
	assert.Zero(t, queryID("a@b"))
	assert.Equal(t, int64(4242), queryID(monoflake.ID(4242).String()))
	assert.Zero(t, queryID("zzzzzzzzzzzzzzzzzzzzzzzz"), "too large for an ID")
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		c, d := newController(t)
		d.users.EXPECT().FindUserByID(ctx, int64(7)).Return(model.User{ID: 7, Email: "a@b.c"}, nil)
		rs, err := c.GetUser(ctx, &entity.GetAdminUserRequest{UserID: 7})
		require.NoError(t, err)
		assert.Equal(t, "a@b.c", rs.User.Email)
	})
	t.Run("not found", func(t *testing.T) {
		c, d := newController(t)
		d.users.EXPECT().FindUserByID(ctx, int64(7)).Return(model.User{}, repository.ErrNotFound)
		_, err := c.GetUser(ctx, &entity.GetAdminUserRequest{UserID: 7})
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
	t.Run("find error", func(t *testing.T) {
		c, d := newController(t)
		d.users.EXPECT().FindUserByID(ctx, int64(7)).Return(model.User{}, errBoom)
		_, err := c.GetUser(ctx, &entity.GetAdminUserRequest{UserID: 7})
		assert.ErrorIs(t, err, errBoom)
	})
}
