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
		OrgMappingID int64
		OrgID        string
		UserID       string
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
		OrgMappingID int64
		Used         int64
	}
	if err := db.Model(&model.CreditBucket{}).
		Where("service_id = ? AND bucket_type = ? AND created_at = ?", s.ID, model.BucketTypeMonthly, MonthStart(at)).
		Group("org_mapping_id").Select("org_mapping_id, SUM(amount - remaining) AS used").Scan(&spent).Error; err != nil {
		return nil, err
	}
	byOrg := make(map[int64]*OrgUsage, len(limits)+len(spent))
	for _, l := range limits {
		byOrg[l.OrgMappingID] = &OrgUsage{OrgLimit: l, Custom: true}
	}
	var unlimited []int64
	for _, sp := range spent {
		if _, ok := byOrg[sp.OrgMappingID]; !ok {
			unlimited = append(unlimited, sp.OrgMappingID)
		}
	}
	orgIDs, err := externalIDs(db, s.ID, unlimited)
	if err != nil {
		return nil, err
	}
	for _, sp := range spent {
		u, ok := byOrg[sp.OrgMappingID]
		if !ok {
			u = &OrgUsage{OrgLimit: defaultOrgLimit(s, orgIDs[sp.OrgMappingID].OrgID)}
			byOrg[sp.OrgMappingID] = u
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
	q := db.Model(&model.CreditBucket{}).
		Joins("JOIN external_id_mappings m ON m.mapping_id = credit_buckets.user_mapping_id AND m.service_id = credit_buckets.service_id AND m.user_id <> ''").
		Where("credit_buckets.service_id = ? AND credit_buckets.expires_at > ?", s.ID, at)
	if orgID != "" {
		q = q.Where("credit_buckets.org_mapping_id = ?", OrgMappingID(s.ID, orgID))
	}
	var users []UserUsage
	if err := q.Group("credit_buckets.org_mapping_id, m.org_id, m.user_id").Order("m.org_id, m.user_id").Limit(limit).
		Select(`credit_buckets.org_mapping_id, m.org_id, m.user_id,
			SUM(CASE WHEN credit_buckets.bucket_type = ? AND credit_buckets.created_at = ? THEN credit_buckets.amount - credit_buckets.remaining ELSE 0 END) AS monthly_used,
			SUM(CASE WHEN credit_buckets.bucket_type = ? THEN credit_buckets.remaining ELSE 0 END) AS grants,
			SUM(credit_buckets.remaining) AS balance`,
			model.BucketTypeMonthly, MonthStart(at), model.BucketTypeGrant).
		Scan(&users).Error; err != nil {
		return nil, err
	}
	limits, err := orgLimits(db, s.ID)
	if err != nil {
		return nil, err
	}
	byOrg := make(map[int64]model.OrgLimit, len(limits))
	for _, l := range limits {
		byOrg[l.OrgMappingID] = l
	}
	for i, u := range users {
		l, ok := byOrg[u.OrgMappingID]
		if !ok {
			l = defaultOrgLimit(s, u.OrgID)
		}
		users[i].OrgLimit = l
	}
	return users, nil
}

// orgLimits returns the limits rows of a service's orgs, with OrgID set.
func orgLimits(db *gorm.DB, serviceID int64) ([]model.OrgLimit, error) {
	var limits []model.OrgLimit
	if err := db.Where("service_id = ?", serviceID).Find(&limits).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, len(limits))
	for i, l := range limits {
		ids[i] = l.OrgMappingID
	}
	orgIDs, err := externalIDs(db, serviceID, ids)
	if err != nil {
		return nil, err
	}
	for i, l := range limits {
		limits[i].OrgID = orgIDs[l.OrgMappingID].OrgID
	}
	return limits, nil
}
