// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package balance

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
	"github.com/topupsh/topupsh-tracker/backend/internal/service/token"
	tokenmocks "github.com/topupsh/topupsh-tracker/backend/internal/service/token/mocks"
)

var (
	_now     = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_month   = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	_renews  = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	errBoom  = errors.New("boom")
	_mesher  = model.Service{ID: 7, Name: "mesher", Enabled: true}
	_tmphost = model.Service{ID: 8, Name: "tmphost"}
	_asKey   = entity.Reader{ServiceID: 7}
	_asRoot  = entity.Reader{Root: true}
	_viewer  = entity.Viewer{ServiceID: 7, OrgID: "o1", UserID: "u1"}
	_asView  = entity.Reader{Viewer: &_viewer}
)

type deps struct {
	balances *repomocks.MockBalanceRepository
	services *repomocks.MockServiceRepository
	tokens   *tokenmocks.MockService
}

func newController(t *testing.T) (Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		balances: repomocks.NewMockBalanceRepository(ctrl),
		services: repomocks.NewMockServiceRepository(ctrl),
		tokens:   tokenmocks.NewMockService(ctrl),
	}
	c := New(Params{
		Balances: d.balances,
		Services: d.services,
		Token:    d.tokens,
		Now:      func() time.Time { return _now },
	})
	return c, d
}

func ptr(v int64) *int64 { return &v }

func TestNew_Defaults(t *testing.T) {
	c := New(Params{}).(*controller)
	assert.WithinDuration(t, time.Now(), c.now(), time.Second)
}

func TestGetBalance(t *testing.T) {
	ctx := context.Background()
	grant := model.CreditBucket{ID: 11, ServiceID: 7, OrgID: "o1", UserID: "u1", Kind: "grant", Amount: 7, Remaining: 6, CreatedAt: _now, ExpiresAt: _renews}

	t.Run("service key", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().GetBalance(ctx, int64(7), "o1", "u1", _now).Return(repository.Balance{
			Service:  _mesher,
			OrgLimit: model.OrgLimit{MonthlyLimit: 100, UserLimit: ptr(10)},
			OrgUsed:  95,
			Buckets: []model.CreditBucket{
				{ID: 10, Kind: "monthly", Amount: 10, Remaining: 7, CreatedAt: _month, ExpiresAt: _renews},
				grant,
			},
		}, nil)
		rs, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asKey, OrgID: "o1", UserID: "u1"})
		require.NoError(t, err)
		assert.Equal(t, entity.Balance{
			Service:   entity.Service{ID: 7, Name: "mesher", Enabled: true},
			OrgID:     "o1",
			UserID:    "u1",
			Credits:   13,
			Available: 11, // 6 of grants and 5 of the monthly 7 the org has left
			Monthly:   entity.MonthlyCredits{Limit: 10, Used: 3, Remaining: 7},
			Org:       entity.OrgCredits{Limit: 100, UserLimit: ptr(10), Used: 95, Remaining: 5},
			Grants: []entity.CreditBucket{{
				ID: 11, ServiceID: 7, OrgID: "o1", UserID: "u1", Kind: entity.BucketKindGrant,
				Amount: 7, Remaining: 6, CreatedAt: _now, ExpiresAt: _renews,
			}},
			RenewsAt: _renews,
		}, rs.Balance)
	})

	t.Run("no charge this month", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().GetBalance(ctx, int64(7), "o1", "u1", _now).Return(repository.Balance{
			Service: _mesher, OrgLimit: model.OrgLimit{MonthlyLimit: 100}, OrgUsed: 120,
		}, nil)
		rs, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asView})
		require.NoError(t, err)
		assert.Equal(t, entity.MonthlyCredits{Limit: 100, Used: 0, Remaining: 100}, rs.Balance.Monthly, "uncapped users get the org limit")
		assert.Equal(t, entity.OrgCredits{Limit: 100, Used: 120, Remaining: 0}, rs.Balance.Org, "never below 0")
		assert.Zero(t, rs.Balance.Available)
		assert.Zero(t, rs.Balance.Credits)
		assert.Equal(t, []entity.CreditBucket{}, rs.Balance.Grants)
	})

	t.Run("lowered limit", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().GetBalance(ctx, int64(7), "o1", "u1", _now).Return(repository.Balance{
			Service:  _mesher,
			OrgLimit: model.OrgLimit{MonthlyLimit: 100, UserLimit: ptr(2)},
			Buckets:  []model.CreditBucket{{Kind: "monthly", Amount: 10, Remaining: 6}},
		}, nil)
		rs, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asKey, OrgID: "o1", UserID: "u1"})
		require.NoError(t, err)
		assert.Equal(t, entity.MonthlyCredits{Limit: 4, Used: 4, Remaining: 0}, rs.Balance.Monthly, "the limit stays at what was used")
		assert.Equal(t, int64(6), rs.Balance.Credits, "the bucket is resized by the next charge")
	})

	t.Run("root names the service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "tmphost").Return(_tmphost, nil)
		d.balances.EXPECT().GetBalance(ctx, int64(8), "o1", "u1", _now).Return(repository.Balance{Service: _tmphost}, nil)
		rs, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asRoot, ServiceName: "tmphost", OrgID: "o1", UserID: "u1"})
		require.NoError(t, err)
		assert.Equal(t, "tmphost", rs.Balance.Service.Name)
	})

	t.Run("service key naming its own service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(_mesher, nil)
		d.balances.EXPECT().GetBalance(ctx, int64(7), "o1", "u1", _now).Return(repository.Balance{Service: _mesher}, nil)
		_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asKey, ServiceName: "mesher", OrgID: "o1", UserID: "u1"})
		require.NoError(t, err)
	})

	t.Run("rejected", func(t *testing.T) {
		long := strings.Repeat("x", 65)
		for name, tc := range map[string]struct {
			rq   entity.GetBalanceRequest
			want error
		}{
			"viewer names a user":  {entity.GetBalanceRequest{Reader: _asView, UserID: "u2"}, ErrViewerFilter},
			"viewer names an org":  {entity.GetBalanceRequest{Reader: _asView, OrgID: "o2"}, ErrViewerFilter},
			"viewer names service": {entity.GetBalanceRequest{Reader: _asView, ServiceName: "mesher"}, ErrViewerFilter},
			"no reader":            {entity.GetBalanceRequest{OrgID: "o1", UserID: "u1"}, ErrForbidden},
			"root without service": {entity.GetBalanceRequest{Reader: _asRoot, OrgID: "o1", UserID: "u1"}, ErrServiceRequired},
			"no org":               {entity.GetBalanceRequest{Reader: _asKey, UserID: "u1"}, ErrInvalidOrg},
			"long org":             {entity.GetBalanceRequest{Reader: _asKey, OrgID: long, UserID: "u1"}, ErrInvalidOrg},
			"no user":              {entity.GetBalanceRequest{Reader: _asKey, OrgID: "o1"}, ErrInvalidUser},
			"long user":            {entity.GetBalanceRequest{Reader: _asKey, OrgID: "o1", UserID: long}, ErrInvalidUser},
		} {
			c, _ := newController(t)
			_, err := c.GetBalance(ctx, &tc.rq)
			assert.ErrorIs(t, err, tc.want, name)
		}
	})

	t.Run("service key naming another service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "tmphost").Return(_tmphost, nil)
		_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asKey, ServiceName: "tmphost", OrgID: "o1", UserID: "u1"})
		assert.ErrorIs(t, err, ErrForbidden)
	})

	t.Run("service key naming an unknown service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "nope").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asKey, ServiceName: "nope", OrgID: "o1", UserID: "u1"})
		assert.ErrorIs(t, err, ErrForbidden, "a service learns nothing about other services")
	})

	t.Run("service lookup fails", func(t *testing.T) {
		for _, rd := range []entity.Reader{_asKey, _asRoot} {
			c, d := newController(t)
			d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{}, errBoom)
			_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: rd, ServiceName: "mesher", OrgID: "o1", UserID: "u1"})
			assert.ErrorIs(t, err, errBoom)
		}
	})

	t.Run("root names an unknown service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "nope").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asRoot, ServiceName: "nope", OrgID: "o1", UserID: "u1"})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})

	t.Run("service gone", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().GetBalance(ctx, int64(7), "o1", "u1", _now).Return(repository.Balance{}, repository.ErrNotFound)
		_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asView})
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})

	t.Run("repository error", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().GetBalance(ctx, int64(7), "o1", "u1", _now).Return(repository.Balance{}, errBoom)
		_, err := c.GetBalance(ctx, &entity.GetBalanceRequest{Reader: _asView})
		assert.ErrorIs(t, err, errBoom)
	})
}

func ledgerRows(ids ...int64) []repository.LedgerRow {
	rows := make([]repository.LedgerRow, len(ids))
	for i, id := range ids {
		rows[i] = repository.LedgerRow{
			LedgerEntry: model.LedgerEntry{ID: id, ServiceID: 7, OrgID: "o1", UserID: "u1", Kind: "spend", Amount: -1, CreatedAt: _now},
			ServiceName: "mesher",
		}
	}
	return rows
}

func TestListLedgerEntries(t *testing.T) {
	ctx := context.Background()

	t.Run("more pages", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().ListLedgerEntries(ctx, repository.LedgerFilter{ServiceID: 7, OrgID: "o1", BeforeID: 50, Limit: 3}).Return(ledgerRows(9, 8, 7), nil)
		rs, err := c.ListLedgerEntries(ctx, &entity.ListLedgerEntriesRequest{Reader: _asKey, OrgID: "o1", Cursor: 50, Limit: 2})
		require.NoError(t, err)
		require.Len(t, rs.Entries, 2)
		assert.Equal(t, entity.LedgerEntry{
			ID: 9, ServiceID: 7, ServiceName: "mesher", OrgID: "o1", UserID: "u1",
			Kind: entity.LedgerKindSpend, Amount: -1, CreatedAt: _now,
		}, rs.Entries[0])
		assert.Equal(t, int64(8), rs.NextCursor)
	})

	t.Run("last page with the default size", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().ListLedgerEntries(ctx, repository.LedgerFilter{ServiceID: 7, OrgID: "o1", UserID: "u1", Limit: DefaultLedgerLimit + 1}).Return(ledgerRows(9), nil)
		rs, err := c.ListLedgerEntries(ctx, &entity.ListLedgerEntriesRequest{Reader: _asView})
		require.NoError(t, err)
		assert.Len(t, rs.Entries, 1)
		assert.Zero(t, rs.NextCursor)
	})

	t.Run("root across every service", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().ListLedgerEntries(ctx, repository.LedgerFilter{Limit: MaxLedgerLimit + 1}).Return(nil, nil)
		rs, err := c.ListLedgerEntries(ctx, &entity.ListLedgerEntriesRequest{Reader: _asRoot, Limit: MaxLedgerLimit})
		require.NoError(t, err)
		assert.Equal(t, []entity.LedgerEntry{}, rs.Entries)
	})

	t.Run("root names a service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "tmphost").Return(_tmphost, nil)
		d.balances.EXPECT().ListLedgerEntries(ctx, repository.LedgerFilter{ServiceID: 8, OrgID: "o1", UserID: "u1", Limit: DefaultLedgerLimit + 1}).Return(nil, nil)
		_, err := c.ListLedgerEntries(ctx, &entity.ListLedgerEntriesRequest{Reader: _asRoot, ServiceName: "tmphost", OrgID: "o1", UserID: "u1"})
		require.NoError(t, err)
	})

	t.Run("rejected", func(t *testing.T) {
		long := strings.Repeat("x", 65)
		for name, tc := range map[string]struct {
			rq   entity.ListLedgerEntriesRequest
			want error
		}{
			"viewer filter":    {entity.ListLedgerEntriesRequest{Reader: _asView, OrgID: "o2"}, ErrViewerFilter},
			"long org":         {entity.ListLedgerEntriesRequest{Reader: _asKey, OrgID: long}, ErrInvalidOrg},
			"long user":        {entity.ListLedgerEntriesRequest{Reader: _asKey, OrgID: "o1", UserID: long}, ErrInvalidUser},
			"user without org": {entity.ListLedgerEntriesRequest{Reader: _asKey, UserID: "u1"}, ErrInvalidUser},
			"negative limit":   {entity.ListLedgerEntriesRequest{Reader: _asKey, Limit: -1}, ErrInvalidLimit},
			"limit too big":    {entity.ListLedgerEntriesRequest{Reader: _asKey, Limit: MaxLedgerLimit + 1}, ErrInvalidLimit},
		} {
			c, _ := newController(t)
			_, err := c.ListLedgerEntries(ctx, &tc.rq)
			assert.ErrorIs(t, err, tc.want, name)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		c, d := newController(t)
		d.balances.EXPECT().ListLedgerEntries(ctx, gomock.Any()).Return(nil, errBoom)
		_, err := c.ListLedgerEntries(ctx, &entity.ListLedgerEntriesRequest{Reader: _asKey})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestCreateViewToken(t *testing.T) {
	ctx := context.Background()
	exp := _now.Add(token.ViewTTL)

	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.tokens.EXPECT().CreateViewToken(token.Viewer{ServiceID: 7, OrgID: "o1", UserID: "u1"}).Return("tok", exp, nil)
		rs, err := c.CreateViewToken(ctx, &entity.CreateViewTokenRequest{ServiceID: 7, OrgID: "o1", UserID: "u1"})
		require.NoError(t, err)
		assert.Equal(t, &entity.CreateViewTokenResponse{Token: "tok", ExpiresAt: exp}, rs)
	})
	t.Run("invalid", func(t *testing.T) {
		c, _ := newController(t)
		_, err := c.CreateViewToken(ctx, &entity.CreateViewTokenRequest{ServiceID: 7, UserID: "u1"})
		assert.ErrorIs(t, err, ErrInvalidOrg)
		_, err = c.CreateViewToken(ctx, &entity.CreateViewTokenRequest{ServiceID: 7, OrgID: "o1"})
		assert.ErrorIs(t, err, ErrInvalidUser)
	})
	t.Run("signing fails", func(t *testing.T) {
		c, d := newController(t)
		d.tokens.EXPECT().CreateViewToken(gomock.Any()).Return("", time.Time{}, errBoom)
		_, err := c.CreateViewToken(ctx, &entity.CreateViewTokenRequest{ServiceID: 7, OrgID: "o1", UserID: "u1"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestAuthenticateViewer(t *testing.T) {
	ctx := context.Background()
	c, d := newController(t)
	d.tokens.EXPECT().ValidateViewToken("good").Return(token.Viewer{ServiceID: 7, OrgID: "o1", UserID: "u1"}, nil)
	d.tokens.EXPECT().ValidateViewToken("bad").Return(token.Viewer{}, token.ErrInvalidToken)

	rs, err := c.AuthenticateViewer(ctx, &entity.AuthenticateViewerRequest{Token: "good"})
	require.NoError(t, err)
	assert.Equal(t, _viewer, rs.Viewer)
	_, err = c.AuthenticateViewer(ctx, &entity.AuthenticateViewerRequest{Token: "bad"})
	assert.ErrorIs(t, err, ErrInvalidToken)
}
