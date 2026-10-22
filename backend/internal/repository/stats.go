// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=stats.go -destination=mocks/stats.go -package=mocks

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

type (
	// StatsRepository answers the admin page's questions about every
	// account. It only reads.
	StatsRepository interface {
		// CountUsers counts every account.
		CountUsers(ctx context.Context) (int64, error)
		// DailySignups returns, per UTC day since since, how many users
		// registered. Days without any are left out.
		DailySignups(ctx context.Context, since time.Time) ([]DayCount, error)
		// SearchUsers returns up to limit users, newest first, with IDs
		// below before (0: from the newest). A non-empty q keeps the users
		// whose email or name contains it, ignoring case, and the user
		// whose ID is id.
		SearchUsers(ctx context.Context, q string, id, before int64, limit int) ([]model.User, error)
	}

	// DayCount is a count for one UTC day, YYYY-MM-DD.
	DayCount struct {
		Day   string
		Count int64
	}

	statsRepository struct {
		db *gorm.DB
	}
)

// NewStatsRepository returns a GORM-backed stats repository.
func NewStatsRepository(db *gorm.DB) StatsRepository {
	return &statsRepository{db: db}
}

// dayExpr is the SQL expression for the UTC day (YYYY-MM-DD text) of a
// time column. SQLite stores times as text with their offset, which its
// date() turns into UTC; Postgres needs the zone named.
func dayExpr(dialect, column string) string {
	if dialect == "postgres" {
		return "to_char(" + column + " AT TIME ZONE 'UTC', 'YYYY-MM-DD')"
	}
	return "date(" + column + ")"
}

func (r *statsRepository) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *statsRepository) DailySignups(ctx context.Context, since time.Time) ([]DayCount, error) {
	day := dayExpr(r.db.Name(), "created_at")
	var ds []DayCount
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Select(day+" AS day, COUNT(*) AS count").
		Where("created_at >= ?", since).
		Group(day).Order("day").
		Scan(&ds).Error
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func (r *statsRepository) SearchUsers(ctx context.Context, q string, id, before int64, limit int) ([]model.User, error) {
	db := r.db.WithContext(ctx)
	if before > 0 {
		db = db.Where("id < ?", before)
	}
	if q != "" {
		like := "%" + escapeLike(strings.ToLower(q)) + "%"
		db = db.Where("LOWER(email) LIKE ? ESCAPE '\\' OR LOWER(name) LIKE ? ESCAPE '\\' OR id = ?", like, like, id)
	}
	var users []model.User
	if err := db.Order("id DESC").Limit(limit).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// escapeLike makes s match itself literally in a LIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s)
}
