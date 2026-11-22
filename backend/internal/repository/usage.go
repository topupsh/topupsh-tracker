// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=usage.go -destination=mocks/usage.go -package=mocks

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

type (
	// UsageRepository reads what a service's orgs and users spent, for the
	// admin screens.
	UsageRepository interface {
		// ListOrgUsage returns every org of s with a limits row
		// or monthly credits spent in at's UTC month, ordered by org.
		ListOrgUsage(ctx context.Context, s model.Service, at time.Time) ([]OrgUsage, error)
		// ListUserUsage returns up to limit users of s with
		// unexpired buckets at at, ordered by org and user. A non-empty
		// orgID keeps that org's users only.
		ListUserUsage(ctx context.Context, s model.Service, orgID string, at time.Time, limit int) ([]UserUsage, error)
	}

	// OrgUsage is an org's limits and its monthly credits spent this month.
	OrgUsage struct {
		// OrgLimit is the org's limits row, or the service's defaults
		// when the admin set none.
		OrgLimit model.OrgLimit
		// Custom is whether the admin set the org's limits.
		Custom bool
		Used   int64
	}

	// UserUsage is a user's credits this month.
	UserUsage struct {
		OrgID  string
		UserID string
		// OrgLimit is the user's org's limits, the defaults when unset.
		OrgLimit model.OrgLimit `gorm:"-"`
		// MonthlyUsed is the credits spent from this month's allowance.
		MonthlyUsed int64
		// Grants is what is left of the user's unexpired grants.
		Grants int64
		// Balance is the user's unexpired credits.
		Balance int64
	}

	usageRepository struct {
		db *gorm.DB
	}
)

// NewUsageRepository returns a GORM-backed usage repository.
func NewUsageRepository(db *gorm.DB) UsageRepository {
	return &usageRepository{db: db}
}

func (r *usageRepository) ListOrgUsage(ctx context.Context, s model.Service, at time.Time) ([]OrgUsage, error) {
	db := r.db.WithContext(ctx)
	limits, err := orgLimits(db, s.ID)
	if err != nil {
		return nil, err
	}
	var spent []struct {
		OrgID string
		Used  int64
	}
	if err := db.Model(&model.CreditBucket{}).
		Where("service_id = ? AND kind = ? AND created_at = ?", s.ID, entity.BucketKindMonthly, MonthStart(at)).
		Group("org_id").Select("org_id, SUM(amount - remaining) AS used").Scan(&spent).Error; err != nil {
		return nil, err
	}
	byOrg := make(map[string]*OrgUsage, len(limits)+len(spent))
	for _, l := range limits {
		byOrg[l.OrgID] = &OrgUsage{OrgLimit: l, Custom: true}
	}
	for _, sp := range spent {
		u, ok := byOrg[sp.OrgID]
		if !ok {
			u = &OrgUsage{OrgLimit: defaultOrgLimit(s, sp.OrgID)}
			byOrg[sp.OrgID] = u
		}
		u.Used = sp.Used
	}
	orgs := make([]OrgUsage, 0, len(byOrg))
	for _, u := range byOrg {
		orgs = append(orgs, *u)
	}
	sort.Slice(orgs, func(i, j int) bool { return orgs[i].OrgLimit.OrgID < orgs[j].OrgLimit.OrgID })
	return orgs, nil
}

func (r *usageRepository) ListUserUsage(ctx context.Context, s model.Service, orgID string, at time.Time, limit int) ([]UserUsage, error) {
	db := r.db.WithContext(ctx)
	q := db.Model(&model.CreditBucket{}).Where("service_id = ? AND expires_at > ?", s.ID, at)
	if orgID != "" {
		q = q.Where("org_id = ?", orgID)
	}
	var users []UserUsage
	if err := q.Group("org_id, user_id").Order("org_id, user_id").Limit(limit).
		Select(`org_id, user_id,
			SUM(CASE WHEN kind = ? AND created_at = ? THEN amount - remaining ELSE 0 END) AS monthly_used,
			SUM(CASE WHEN kind = ? THEN remaining ELSE 0 END) AS grants,
			SUM(remaining) AS balance`,
			entity.BucketKindMonthly, MonthStart(at), entity.BucketKindGrant).
		Scan(&users).Error; err != nil {
		return nil, err
	}
	limits, err := orgLimits(db, s.ID)
	if err != nil {
		return nil, err
	}
	byOrg := make(map[string]model.OrgLimit, len(limits))
	for _, l := range limits {
		byOrg[l.OrgID] = l
	}
	for i, u := range users {
		l, ok := byOrg[u.OrgID]
		if !ok {
			l = defaultOrgLimit(s, u.OrgID)
		}
		users[i].OrgLimit = l
	}
	return users, nil
}

// orgLimits returns the limits rows of a service's orgs.
func orgLimits(db *gorm.DB, serviceID int64) ([]model.OrgLimit, error) {
	var limits []model.OrgLimit
	if err := db.Where("service_id = ?", serviceID).Find(&limits).Error; err != nil {
		return nil, err
	}
	return limits, nil
}
