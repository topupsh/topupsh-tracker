// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package creditadmin holds the admin's credit settings: the per-service
// switch, org and user limits, and grants.
package creditadmin

//go:generate mockgen -source=creditadmin.go -destination=mocks/creditadmin.go -package=mocks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	mappermodel "github.com/topupsh/topupsh-tracker/backend/internal/mapper/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/idgen"
)

// ActorRoot is the ledger actor of changes the root admin makes.
const ActorRoot = "root"

// _maxSubjectLen is the longest org or user ID a service may use.
const _maxSubjectLen = 64

// MaxUsageUsers is the most users ListUserUsage returns.
const MaxUsageUsers = 500

var (
	// ErrServiceNotFound is returned for an unknown service.
	ErrServiceNotFound = errors.New("service not found")
	// ErrInvalidOrg is returned for an org ID that is empty or longer
	// than 64 characters.
	ErrInvalidOrg = errors.New("invalid org")
	// ErrInvalidUser is returned for a user ID that is empty or longer
	// than 64 characters.
	ErrInvalidUser = errors.New("invalid user")
	// ErrInvalidLimit is returned for a negative limit.
	ErrInvalidLimit = errors.New("limits must not be negative")
	// ErrInvalidCredits is returned for a grant of less than 1 credit.
	ErrInvalidCredits = errors.New("credits must be positive")
)

type (
	// Controller changes the admin's credit settings.
	Controller interface {
		// SetServiceEnabled switches credit limiting on or off for a
		// service. While it is off nothing is charged or recorded.
		SetServiceEnabled(ctx context.Context, rq *entity.SetServiceEnabledRequest) (*entity.SetServiceEnabledResponse, error)
		// SetOrgLimits sets a service's monthly org limit and user limit
		// for one org.
		SetOrgLimits(ctx context.Context, rq *entity.SetOrgLimitsRequest) (*entity.SetOrgLimitsResponse, error)
		// CreateGrant gives a user a bucket of extra credits that expires
		// one month later, at the next 00:00 UTC, and records it in the
		// ledger.
		CreateGrant(ctx context.Context, rq *entity.CreateGrantRequest) (*entity.CreateGrantResponse, error)
		// SetServiceDefaults sets a service's default limits, which apply
		// to the orgs the admin set no limits for.
		SetServiceDefaults(ctx context.Context, rq *entity.SetServiceDefaultsRequest) (*entity.SetServiceDefaultsResponse, error)
		// ListServices returns every service with its API keys.
		ListServices(ctx context.Context, rq *entity.ListServicesRequest) (*entity.ListServicesResponse, error)
		// ListOrgUsage returns a service's orgs with their limits and
		// what they spent this UTC month.
		ListOrgUsage(ctx context.Context, rq *entity.ListOrgUsageRequest) (*entity.ListOrgUsageResponse, error)
		// ListUserUsage returns up to MaxUsageUsers users of a service
		// with unexpired credits, with what they spent this UTC month.
		ListUserUsage(ctx context.Context, rq *entity.ListUserUsageRequest) (*entity.ListUserUsageResponse, error)
	}

	// Params are the controller's dependencies.
	Params struct {
		Services repository.ServiceRepository
		Credits  repository.CreditRepository
		Usage    repository.UsageRepository
		IDGen    idgen.Service
		Now      func() time.Time
	}

	controller struct {
		services repository.ServiceRepository
		credits  repository.CreditRepository
		usage    repository.UsageRepository
		ids      idgen.Service
		now      func() time.Time
	}
)

// New returns the credit admin controller.
func New(p Params) Controller {
	c := &controller{services: p.Services, credits: p.Credits, usage: p.Usage, ids: p.IDGen, now: p.Now}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

func (c *controller) SetServiceEnabled(ctx context.Context, rq *entity.SetServiceEnabledRequest) (*entity.SetServiceEnabledResponse, error) {
	s, err := c.services.SetServiceEnabled(ctx, rq.ServiceName, rq.Enabled)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set service enabled: %w", err)
	}
	return &entity.SetServiceEnabledResponse{Service: mappermodel.FromServiceModelToServiceEntity(s)}, nil
}

func (c *controller) SetOrgLimits(ctx context.Context, rq *entity.SetOrgLimitsRequest) (*entity.SetOrgLimitsResponse, error) {
	if !validSubject(rq.OrgID) {
		return nil, ErrInvalidOrg
	}
	if (rq.MonthlyLimit != nil && *rq.MonthlyLimit < 0) || (rq.UserLimit != nil && *rq.UserLimit < 0) {
		return nil, ErrInvalidLimit
	}
	s, err := c.findService(ctx, rq.ServiceName)
	if err != nil {
		return nil, err
	}
	monthly := s.DefaultOrgLimit
	if rq.MonthlyLimit != nil {
		monthly = *rq.MonthlyLimit
	}
	now := c.now().UTC()
	l, err := c.credits.UpsertOrgLimit(ctx, model.OrgLimit{
		ID:           c.ids.NextID(),
		CreatedAt:    now,
		UpdatedAt:    now,
		ServiceID:    s.ID,
		OrgID:        rq.OrgID,
		MonthlyLimit: monthly,
		UserLimit:    rq.UserLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert org limit: %w", err)
	}
	return &entity.SetOrgLimitsResponse{
		Service:  mappermodel.FromServiceModelToServiceEntity(s),
		OrgLimit: mappermodel.FromOrgLimitModelToOrgLimitEntity(l),
	}, nil
}

func (c *controller) CreateGrant(ctx context.Context, rq *entity.CreateGrantRequest) (*entity.CreateGrantResponse, error) {
	if !validSubject(rq.OrgID) {
		return nil, ErrInvalidOrg
	}
	if !validSubject(rq.UserID) {
		return nil, ErrInvalidUser
	}
	if rq.Credits <= 0 {
		return nil, ErrInvalidCredits
	}
	s, err := c.findService(ctx, rq.ServiceName)
	if err != nil {
		return nil, err
	}
	now := c.now().UTC()
	b, e, err := c.credits.CreateGrant(ctx, model.CreditBucket{
		ID:        c.ids.NextID(),
		CreatedAt: now,
		ServiceID: s.ID,
		OrgID:     rq.OrgID,
		UserID:    rq.UserID,
		Kind:      string(entity.BucketKindGrant),
		Amount:    rq.Credits,
		Remaining: rq.Credits,
		ExpiresAt: GrantExpiry(now),
	}, model.LedgerEntry{
		ID:        c.ids.NextID(),
		CreatedAt: now,
		ServiceID: s.ID,
		OrgID:     rq.OrgID,
		UserID:    rq.UserID,
		Kind:      string(entity.LedgerKindGrant),
		Amount:    rq.Credits,
		Actor:     ActorRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("create grant: %w", err)
	}
	return &entity.CreateGrantResponse{
		Service:     mappermodel.FromServiceModelToServiceEntity(s),
		Bucket:      mappermodel.FromCreditBucketModelToCreditBucketEntity(b),
		LedgerEntry: mappermodel.FromLedgerEntryModelToLedgerEntryEntity(e),
	}, nil
}

func (c *controller) SetServiceDefaults(ctx context.Context, rq *entity.SetServiceDefaultsRequest) (*entity.SetServiceDefaultsResponse, error) {
	if rq.OrgLimit < 0 || (rq.UserLimit != nil && *rq.UserLimit < 0) {
		return nil, ErrInvalidLimit
	}
	s, err := c.services.SetServiceDefaults(ctx, rq.ServiceName, rq.OrgLimit, rq.UserLimit)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set service defaults: %w", err)
	}
	return &entity.SetServiceDefaultsResponse{Service: mappermodel.FromServiceModelToServiceEntity(s)}, nil
}

func (c *controller) ListServices(ctx context.Context, _ *entity.ListServicesRequest) (*entity.ListServicesResponse, error) {
	services, err := c.services.ListServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	keys, err := c.services.ListServiceAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	byService := make(map[int64][]entity.ServiceAPIKey, len(services))
	for _, k := range keys {
		byService[k.ServiceID] = append(byService[k.ServiceID], mappermodel.FromServiceAPIKeyModelToServiceAPIKeyEntity(k))
	}
	rs := &entity.ListServicesResponse{Services: make([]entity.ServiceWithKeys, 0, len(services))}
	for _, s := range services {
		rs.Services = append(rs.Services, entity.ServiceWithKeys{
			Service: mappermodel.FromServiceModelToServiceEntity(s),
			APIKeys: byService[s.ID],
		})
	}
	return rs, nil
}

func (c *controller) ListOrgUsage(ctx context.Context, rq *entity.ListOrgUsageRequest) (*entity.ListOrgUsageResponse, error) {
	s, err := c.findService(ctx, rq.ServiceName)
	if err != nil {
		return nil, err
	}
	orgs, err := c.usage.ListOrgUsage(ctx, s, c.now().UTC())
	if err != nil {
		return nil, fmt.Errorf("list org usage: %w", err)
	}
	rs := &entity.ListOrgUsageResponse{Service: mappermodel.FromServiceModelToServiceEntity(s), Orgs: make([]entity.OrgUsage, 0, len(orgs))}
	for _, o := range orgs {
		rs.Orgs = append(rs.Orgs, entity.OrgUsage{
			OrgLimit: mappermodel.FromOrgLimitModelToOrgLimitEntity(o.OrgLimit),
			Custom:   o.Custom,
			Used:     o.Used,
		})
	}
	return rs, nil
}

func (c *controller) ListUserUsage(ctx context.Context, rq *entity.ListUserUsageRequest) (*entity.ListUserUsageResponse, error) {
	if rq.OrgID != "" && !validSubject(rq.OrgID) {
		return nil, ErrInvalidOrg
	}
	s, err := c.findService(ctx, rq.ServiceName)
	if err != nil {
		return nil, err
	}
	// One more than shown tells whether the list was cut short.
	users, err := c.usage.ListUserUsage(ctx, s, rq.OrgID, c.now().UTC(), MaxUsageUsers+1)
	if err != nil {
		return nil, fmt.Errorf("list user usage: %w", err)
	}
	rs := &entity.ListUserUsageResponse{Service: mappermodel.FromServiceModelToServiceEntity(s), Truncated: len(users) > MaxUsageUsers}
	users = users[:min(len(users), MaxUsageUsers)]
	rs.Users = make([]entity.UserUsage, 0, len(users))
	for _, u := range users {
		rs.Users = append(rs.Users, entity.UserUsage{
			OrgID:            u.OrgID,
			UserID:           u.UserID,
			MonthlyAllowance: repository.MonthlyAllowance(u.OrgLimit),
			MonthlyUsed:      u.MonthlyUsed,
			Grants:           u.Grants,
			Balance:          u.Balance,
		})
	}
	return rs, nil
}

// GrantExpiry is when a grant made at t expires: one month later, at the
// next 00:00 UTC. A day the next month lacks (the 31st) becomes its last
// day, so a grant made on Jan 31 expires on Mar 1 at 00:00 UTC.
func GrantExpiry(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	if last := time.Date(y, m+2, 0, 0, 0, 0, 0, time.UTC).Day(); d > last {
		d = last
	}
	return time.Date(y, m+1, d+1, 0, 0, 0, 0, time.UTC)
}

func (c *controller) findService(ctx context.Context, name string) (model.Service, error) {
	s, err := c.services.FindServiceByName(ctx, name)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Service{}, ErrServiceNotFound
	}
	if err != nil {
		return model.Service{}, fmt.Errorf("find service: %w", err)
	}
	return s, nil
}

func validSubject(id string) bool {
	return id != "" && len(id) <= _maxSubjectLen
}
