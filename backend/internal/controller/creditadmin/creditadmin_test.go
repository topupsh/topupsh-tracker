// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package creditadmin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	repomocks "github.com/topupsh/topupsh-tracker/backend/internal/repository/mocks"
	idgenmocks "github.com/topupsh/topupsh-tracker/backend/internal/service/idgen/mocks"
)

var (
	_now    = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	errBoom = errors.New("boom")
	_mesher = model.Service{ID: 7, Name: "mesher", Enabled: true}
)

type deps struct {
	services *repomocks.MockServiceRepository
	credits  *repomocks.MockCreditRepository
	usage    *repomocks.MockUsageRepository
	ids      *idgenmocks.MockService
}

func newController(t *testing.T) (Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		services: repomocks.NewMockServiceRepository(ctrl),
		credits:  repomocks.NewMockCreditRepository(ctrl),
		usage:    repomocks.NewMockUsageRepository(ctrl),
		ids:      idgenmocks.NewMockService(ctrl),
	}
	c := New(Params{
		Services: d.services,
		Credits:  d.credits,
		Usage:    d.usage,
		IDGen:    d.ids,
		Now:      func() time.Time { return _now },
	})
	return c, d
}

func ptr(v int64) *int64 { return &v }

func TestNew_Defaults(t *testing.T) {
	c := New(Params{}).(*controller)
	assert.WithinDuration(t, time.Now(), c.now(), time.Second)
}

func TestSetServiceEnabled(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().SetServiceEnabled(ctx, "mesher", true).Return(_mesher, nil)
		rs, err := c.SetServiceEnabled(ctx, &entity.SetServiceEnabledRequest{ServiceName: "mesher", Enabled: true})
		require.NoError(t, err)
		assert.Equal(t, entity.Service{ID: 7, Name: "mesher", Enabled: true}, rs.Service)
	})
	t.Run("not found", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().SetServiceEnabled(ctx, "nope", false).Return(model.Service{}, repository.ErrNotFound)
		_, err := c.SetServiceEnabled(ctx, &entity.SetServiceEnabledRequest{ServiceName: "nope"})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("db error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().SetServiceEnabled(ctx, "mesher", false).Return(model.Service{}, errBoom)
		_, err := c.SetServiceEnabled(ctx, &entity.SetServiceEnabledRequest{ServiceName: "mesher"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestSetOrgLimits(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.ids.EXPECT().NextID().Return(int64(1))
		d.credits.EXPECT().UpsertOrgLimit(ctx, model.OrgLimit{
			ID: 1, CreatedAt: _now, UpdatedAt: _now, ServiceID: 7, OrgID: "o1", MonthlyLimit: 1000, UserLimit: ptr(10),
		}).Return(model.OrgLimit{ID: 9, ServiceID: 7, OrgID: "o1", MonthlyLimit: 1000, UserLimit: ptr(10)}, nil)
		rs, err := c.SetOrgLimits(ctx, &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1", MonthlyLimit: ptr(1000), UserLimit: ptr(10)})
		require.NoError(t, err)
		assert.Equal(t, "mesher", rs.Service.Name)
		assert.Equal(t, entity.OrgLimit{ID: 9, ServiceID: 7, OrgID: "o1", MonthlyLimit: 1000, UserLimit: ptr(10)}, rs.OrgLimit)
	})
	t.Run("defaults", func(t *testing.T) {
		c, d := newController(t)
		withDefaults := _mesher
		withDefaults.DefaultOrgLimit = 50
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(withDefaults, nil)
		d.ids.EXPECT().NextID().Return(int64(1))
		d.credits.EXPECT().UpsertOrgLimit(ctx, model.OrgLimit{
			ID: 1, CreatedAt: _now, UpdatedAt: _now, ServiceID: 7, OrgID: "o1", MonthlyLimit: 50,
		}).Return(model.OrgLimit{ID: 1, ServiceID: 7, OrgID: "o1", MonthlyLimit: 50}, nil)
		rs, err := c.SetOrgLimits(ctx, &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1"})
		require.NoError(t, err)
		assert.Nil(t, rs.OrgLimit.UserLimit, "users are uncapped")
	})
	t.Run("invalid", func(t *testing.T) {
		c, _ := newController(t)
		for _, tc := range []struct {
			rq  entity.SetOrgLimitsRequest
			err error
		}{
			{entity.SetOrgLimitsRequest{ServiceName: "mesher"}, ErrInvalidOrg},
			{entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: strings.Repeat("o", 65)}, ErrInvalidOrg},
			{entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1", MonthlyLimit: ptr(-1)}, ErrInvalidLimit},
			{entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1", UserLimit: ptr(-1)}, ErrInvalidLimit},
		} {
			_, err := c.SetOrgLimits(ctx, &tc.rq)
			assert.ErrorIs(t, err, tc.err)
		}
	})
	t.Run("service not found", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "nope").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.SetOrgLimits(ctx, &entity.SetOrgLimitsRequest{ServiceName: "nope", OrgID: "o1"})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("find service error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{}, errBoom)
		_, err := c.SetOrgLimits(ctx, &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1"})
		assert.ErrorIs(t, err, errBoom)
		assert.NotErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("upsert error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.ids.EXPECT().NextID().Return(int64(1))
		d.credits.EXPECT().UpsertOrgLimit(ctx, gomock.Any()).Return(model.OrgLimit{}, errBoom)
		_, err := c.SetOrgLimits(ctx, &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestCreateGrant(t *testing.T) {
	ctx := context.Background()
	rq := &entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", UserID: "u1", Credits: 100}
	expires := time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC)
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		gomock.InOrder(
			d.ids.EXPECT().NextID().Return(int64(1)),
			d.ids.EXPECT().NextID().Return(int64(2)),
		)
		b := model.CreditBucket{
			ID: 1, CreatedAt: _now, ServiceID: 7, OrgID: "o1", UserID: "u1",
			Kind: "grant", Amount: 100, Remaining: 100, ExpiresAt: expires,
		}
		e := model.LedgerEntry{
			ID: 2, CreatedAt: _now, ServiceID: 7, OrgID: "o1", UserID: "u1",
			Kind: "grant", Amount: 100, Actor: ActorRoot,
		}
		saved := e
		saved.BalanceAfter = 150
		d.credits.EXPECT().CreateGrant(ctx, b, e).Return(b, saved, nil)
		rs, err := c.CreateGrant(ctx, rq)
		require.NoError(t, err)
		assert.Equal(t, "mesher", rs.Service.Name)
		assert.Equal(t, entity.BucketKindGrant, rs.Bucket.Kind)
		assert.Equal(t, expires, rs.Bucket.ExpiresAt)
		assert.Equal(t, entity.LedgerKindGrant, rs.LedgerEntry.Kind)
		assert.Equal(t, int64(150), rs.LedgerEntry.BalanceAfter)
	})
	t.Run("invalid", func(t *testing.T) {
		c, _ := newController(t)
		for _, tc := range []struct {
			rq  entity.CreateGrantRequest
			err error
		}{
			{entity.CreateGrantRequest{ServiceName: "mesher", UserID: "u1", Credits: 1}, ErrInvalidOrg},
			{entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", Credits: 1}, ErrInvalidUser},
			{entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", UserID: strings.Repeat("u", 65), Credits: 1}, ErrInvalidUser},
			{entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", UserID: "u1"}, ErrInvalidCredits},
			{entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", UserID: "u1", Credits: -5}, ErrInvalidCredits},
		} {
			_, err := c.CreateGrant(ctx, &tc.rq)
			assert.ErrorIs(t, err, tc.err)
		}
	})
	t.Run("service not found", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.CreateGrant(ctx, rq)
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("create error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.ids.EXPECT().NextID().Return(int64(1)).Times(2)
		d.credits.EXPECT().CreateGrant(ctx, gomock.Any(), gomock.Any()).Return(model.CreditBucket{}, model.LedgerEntry{}, errBoom)
		_, err := c.CreateGrant(ctx, rq)
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestGrantExpiry(t *testing.T) {
	for _, tc := range []struct{ at, want string }{
		{"2026-11-21T12:00:00Z", "2026-12-22T00:00:00Z"},
		{"2026-11-21T00:00:00Z", "2026-12-22T00:00:00Z"},
		{"2026-12-15T23:59:59Z", "2027-01-16T00:00:00Z"},
		{"2026-01-31T08:00:00Z", "2026-03-01T00:00:00Z"},
		{"2028-01-31T08:00:00Z", "2028-03-01T00:00:00Z"},
		{"2026-03-31T08:00:00Z", "2026-05-01T00:00:00Z"},
		{"2026-11-21T23:30:00-05:00", "2026-12-23T00:00:00Z"},
	} {
		at, err := time.Parse(time.RFC3339, tc.at)
		require.NoError(t, err)
		assert.Equal(t, tc.want, GrantExpiry(at).Format(time.RFC3339), tc.at)
	}
}

func TestSetServiceDefaults(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		saved := _mesher
		saved.DefaultOrgLimit, saved.DefaultUserLimit = 100, ptr(10)
		d.services.EXPECT().SetServiceDefaults(ctx, "mesher", int64(100), ptr(10)).Return(saved, nil)
		rs, err := c.SetServiceDefaults(ctx, &entity.SetServiceDefaultsRequest{ServiceName: "mesher", OrgLimit: 100, UserLimit: ptr(10)})
		require.NoError(t, err)
		assert.Equal(t, entity.Service{ID: 7, Name: "mesher", Enabled: true, DefaultOrgLimit: 100, DefaultUserLimit: ptr(10)}, rs.Service)
	})
	t.Run("invalid", func(t *testing.T) {
		c, _ := newController(t)
		_, err := c.SetServiceDefaults(ctx, &entity.SetServiceDefaultsRequest{ServiceName: "mesher", OrgLimit: -1})
		assert.ErrorIs(t, err, ErrInvalidLimit)
		_, err = c.SetServiceDefaults(ctx, &entity.SetServiceDefaultsRequest{ServiceName: "mesher", UserLimit: ptr(-1)})
		assert.ErrorIs(t, err, ErrInvalidLimit)
	})
	t.Run("not found", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().SetServiceDefaults(ctx, "nope", int64(0), nil).Return(model.Service{}, repository.ErrNotFound)
		_, err := c.SetServiceDefaults(ctx, &entity.SetServiceDefaultsRequest{ServiceName: "nope"})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("db error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().SetServiceDefaults(ctx, "mesher", int64(0), nil).Return(model.Service{}, errBoom)
		_, err := c.SetServiceDefaults(ctx, &entity.SetServiceDefaultsRequest{ServiceName: "mesher"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestListServices(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		tmphost := model.Service{ID: 8, Name: "tmphost"}
		d.services.EXPECT().ListServices(ctx).Return([]model.Service{_mesher, tmphost}, nil)
		d.services.EXPECT().ListServiceAPIKeys(ctx).Return([]model.ServiceAPIKey{
			{ID: 2, ServiceID: 7, Prefix: "tpk_b"},
			{ID: 1, ServiceID: 7, Prefix: "tpk_a", RevokedAt: &_now},
		}, nil)
		rs, err := c.ListServices(ctx, &entity.ListServicesRequest{})
		require.NoError(t, err)
		require.Len(t, rs.Services, 2)
		assert.Equal(t, "mesher", rs.Services[0].Service.Name)
		assert.Equal(t, []entity.ServiceAPIKey{
			{ID: 2, ServiceID: 7, Prefix: "tpk_b"},
			{ID: 1, ServiceID: 7, Prefix: "tpk_a", RevokedAt: &_now},
		}, rs.Services[0].APIKeys)
		assert.Equal(t, "tmphost", rs.Services[1].Service.Name)
		assert.Empty(t, rs.Services[1].APIKeys)
	})
	t.Run("list services error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().ListServices(ctx).Return(nil, errBoom)
		_, err := c.ListServices(ctx, &entity.ListServicesRequest{})
		assert.ErrorIs(t, err, errBoom)
	})
	t.Run("list keys error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().ListServices(ctx).Return(nil, nil)
		d.services.EXPECT().ListServiceAPIKeys(ctx).Return(nil, errBoom)
		_, err := c.ListServices(ctx, &entity.ListServicesRequest{})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestListOrgUsage(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.usage.EXPECT().ListOrgUsage(ctx, _mesher, _now).Return([]repository.OrgUsage{
			{OrgLimit: model.OrgLimit{ID: 1, ServiceID: 7, OrgID: "o1", MonthlyLimit: 100, UserLimit: ptr(10)}, Custom: true, Used: 7},
			{OrgLimit: model.OrgLimit{ServiceID: 7, OrgID: "o2"}, Used: 3},
		}, nil)
		rs, err := c.ListOrgUsage(ctx, &entity.ListOrgUsageRequest{ServiceName: "mesher"})
		require.NoError(t, err)
		assert.Equal(t, "mesher", rs.Service.Name)
		assert.Equal(t, []entity.OrgUsage{
			{OrgLimit: entity.OrgLimit{ID: 1, ServiceID: 7, OrgID: "o1", MonthlyLimit: 100, UserLimit: ptr(10)}, Custom: true, Used: 7},
			{OrgLimit: entity.OrgLimit{ServiceID: 7, OrgID: "o2"}, Used: 3},
		}, rs.Orgs)
	})
	t.Run("service not found", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "nope").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.ListOrgUsage(ctx, &entity.ListOrgUsageRequest{ServiceName: "nope"})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("db error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.usage.EXPECT().ListOrgUsage(ctx, _mesher, _now).Return(nil, errBoom)
		_, err := c.ListOrgUsage(ctx, &entity.ListOrgUsageRequest{ServiceName: "mesher"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestListUserUsage(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.usage.EXPECT().ListUserUsage(ctx, _mesher, "o1", _now, MaxUsageUsers+1).Return([]repository.UserUsage{
			{OrgID: "o1", UserID: "u1", OrgLimit: model.OrgLimit{MonthlyLimit: 100, UserLimit: ptr(10)}, MonthlyUsed: 3, Grants: 5, Balance: 12},
			{OrgID: "o1", UserID: "u2", OrgLimit: model.OrgLimit{MonthlyLimit: 100}, Balance: 100},
		}, nil)
		rs, err := c.ListUserUsage(ctx, &entity.ListUserUsageRequest{ServiceName: "mesher", OrgID: "o1"})
		require.NoError(t, err)
		assert.False(t, rs.Truncated)
		assert.Equal(t, []entity.UserUsage{
			{OrgID: "o1", UserID: "u1", MonthlyAllowance: 10, MonthlyUsed: 3, Grants: 5, Balance: 12},
			{OrgID: "o1", UserID: "u2", MonthlyAllowance: 100, Balance: 100},
		}, rs.Users)
	})
	t.Run("truncated", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.usage.EXPECT().ListUserUsage(ctx, _mesher, "", _now, MaxUsageUsers+1).Return(make([]repository.UserUsage, MaxUsageUsers+1), nil)
		rs, err := c.ListUserUsage(ctx, &entity.ListUserUsageRequest{ServiceName: "mesher"})
		require.NoError(t, err)
		assert.True(t, rs.Truncated)
		assert.Len(t, rs.Users, MaxUsageUsers)
	})
	t.Run("invalid org", func(t *testing.T) {
		c, _ := newController(t)
		_, err := c.ListUserUsage(ctx, &entity.ListUserUsageRequest{ServiceName: "mesher", OrgID: strings.Repeat("o", 65)})
		assert.ErrorIs(t, err, ErrInvalidOrg)
	})
	t.Run("service not found", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "nope").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.ListUserUsage(ctx, &entity.ListUserUsageRequest{ServiceName: "nope"})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("db error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.usage.EXPECT().ListUserUsage(ctx, _mesher, "", _now, MaxUsageUsers+1).Return(nil, errBoom)
		_, err := c.ListUserUsage(ctx, &entity.ListUserUsageRequest{ServiceName: "mesher"})
		assert.ErrorIs(t, err, errBoom)
	})
}
