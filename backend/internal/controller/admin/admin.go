// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package admin reports on every account for the root-only admin page:
// totals and daily sign-ups, and the users.
package admin

//go:generate mockgen -source=admin.go -destination=mocks/admin.go -package=mocks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mustafaturan/monoflake"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	mappermodel "github.com/topupsh/topupsh-tracker/backend/internal/mapper/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
)

const (
	// StatsDays is how many UTC days, ending today, the stats cover.
	StatsDays = 30
	// DefaultUsersLimit is the user page size unless asked otherwise.
	DefaultUsersLimit = 50
	// MaxUsersLimit is the largest user page.
	MaxUsersLimit = 200

	_day = 24 * time.Hour
)

// ErrUserNotFound is returned for an unknown user.
var ErrUserNotFound = errors.New("user not found")

type (
	// Controller reports on every account.
	Controller interface {
		// GetStats returns the totals over every account and one entry per
		// day of the last StatsDays days.
		GetStats(ctx context.Context, rq *entity.GetAdminStatsRequest) (*entity.GetAdminStatsResponse, error)
		// ListUsers returns a page of users, newest first.
		ListUsers(ctx context.Context, rq *entity.ListAdminUsersRequest) (*entity.ListAdminUsersResponse, error)
		// GetUser returns one user.
		GetUser(ctx context.Context, rq *entity.GetAdminUserRequest) (*entity.GetAdminUserResponse, error)
	}

	// Params are the controller's dependencies.
	Params struct {
		Users repository.UserRepository
		Stats repository.StatsRepository
		Now   func() time.Time
	}

	controller struct {
		users repository.UserRepository
		stats repository.StatsRepository
		now   func() time.Time
	}
)

// New returns the admin controller.
func New(p Params) Controller {
	c := &controller{users: p.Users, stats: p.Stats, now: p.Now}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

func (c *controller) GetStats(ctx context.Context, _ *entity.GetAdminStatsRequest) (*entity.GetAdminStatsResponse, error) {
	since := c.now().UTC().Truncate(_day).Add(-time.Duration(StatsDays-1) * _day)
	users, err := c.stats.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	signups, err := c.stats.DailySignups(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("daily signups: %w", err)
	}
	rs := &entity.GetAdminStatsResponse{Users: users, Days: make([]entity.AdminStatsDay, StatsDays)}
	index := make(map[string]int, StatsDays)
	for i := range rs.Days {
		rs.Days[i].Date = since.Add(time.Duration(i) * _day).Format(time.DateOnly)
		index[rs.Days[i].Date] = i
	}
	for _, d := range signups {
		if i, ok := index[d.Day]; ok {
			rs.Days[i].Signups = d.Count
			rs.Signups += d.Count
		}
	}
	return rs, nil
}

func (c *controller) ListUsers(ctx context.Context, rq *entity.ListAdminUsersRequest) (*entity.ListAdminUsersResponse, error) {
	limit := rq.Limit
	if limit <= 0 {
		limit = DefaultUsersLimit
	}
	limit = min(limit, MaxUsersLimit)
	q := strings.TrimSpace(rq.Query)
	// One extra row tells whether another page follows.
	ms, err := c.stats.SearchUsers(ctx, q, queryID(q), rq.Before, limit+1)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	rs := &entity.ListAdminUsersResponse{}
	if len(ms) > limit {
		ms = ms[:limit]
		rs.Next = ms[limit-1].ID
	}
	rs.Users = make([]entity.AdminUser, len(ms))
	for i, m := range ms {
		rs.Users[i] = adminUser(m)
	}
	return rs, nil
}

// queryID is the user ID a search names, if it is a base62 ID; else 0.
func queryID(q string) int64 {
	if q == "" || strings.ContainsFunc(q, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	}) {
		return 0
	}
	return max(monoflake.IDFromBase62(q).Int64(), 0)
}

func (c *controller) GetUser(ctx context.Context, rq *entity.GetAdminUserRequest) (*entity.GetAdminUserResponse, error) {
	m, err := c.users.FindUserByID(ctx, rq.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &entity.GetAdminUserResponse{User: adminUser(m)}, nil
}

// adminUser maps a stored user to an account as root sees it.
func adminUser(m model.User) entity.AdminUser {
	return entity.AdminUser{User: mappermodel.FromUserModelToUserEntity(m)}
}
