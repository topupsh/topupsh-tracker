// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package servicekey

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
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
	// _key is the key made from 32 zero bytes.
	_key = KeyPrefix + strings.Repeat("A", 43)
)

type deps struct {
	services *repomocks.MockServiceRepository
	ids      *idgenmocks.MockService
}

func newController(t *testing.T) (Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		services: repomocks.NewMockServiceRepository(ctrl),
		ids:      idgenmocks.NewMockService(ctrl),
	}
	c := New(Params{
		Services: d.services,
		IDGen:    d.ids,
		Rand:     bytes.NewReader(make([]byte, _keyBytes)),
		Now:      func() time.Time { return _now },
	})
	return c, d
}

func TestNew_Defaults(t *testing.T) {
	c := New(Params{}).(*controller)
	assert.Equal(t, rand.Reader, c.rand)
	assert.WithinDuration(t, time.Now(), c.now(), time.Second)
}

func TestCreateServiceAPIKey(t *testing.T) {
	ctx := context.Background()
	c, d := newController(t)
	gomock.InOrder(
		d.ids.EXPECT().NextID().Return(int64(1)),
		d.ids.EXPECT().NextID().Return(int64(2)),
	)
	d.services.EXPECT().FindOrCreateService(ctx, model.Service{ID: 1, Name: "mesher"}).
		Return(model.Service{ID: 7, Name: "mesher"}, nil)
	stored := model.ServiceAPIKey{
		ID: 2, CreatedAt: _now, ServiceID: 7, Prefix: KeyPrefix + "AAAAAAAA", KeyHash: hashKey(_key),
	}
	d.services.EXPECT().CreateServiceAPIKey(ctx, stored).Return(stored, nil)

	rs, err := c.CreateServiceAPIKey(ctx, &entity.CreateServiceAPIKeyRequest{ServiceName: "mesher"})
	require.NoError(t, err)
	assert.Equal(t, _key, rs.Key)
	assert.Equal(t, entity.Service{ID: 7, Name: "mesher"}, rs.Service)
	assert.Equal(t, entity.ServiceAPIKey{
		ID: 2, ServiceID: 7, Prefix: KeyPrefix + "AAAAAAAA", KeyHash: hashKey(_key), CreatedAt: _now,
	}, rs.APIKey)
	assert.Len(t, hashKey(_key), 64)
}

func TestCreateServiceAPIKey_RandomKeys(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	services := repomocks.NewMockServiceRepository(ctrl)
	ids := idgenmocks.NewMockService(ctrl)
	ids.EXPECT().NextID().Return(int64(1)).AnyTimes()
	services.EXPECT().FindOrCreateService(ctx, gomock.Any()).Return(model.Service{ID: 1}, nil).Times(2)
	services.EXPECT().CreateServiceAPIKey(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, k model.ServiceAPIKey) (model.ServiceAPIKey, error) { return k, nil }).Times(2)
	c := New(Params{Services: services, IDGen: ids})

	a, err := c.CreateServiceAPIKey(ctx, &entity.CreateServiceAPIKeyRequest{ServiceName: "mesher"})
	require.NoError(t, err)
	b, err := c.CreateServiceAPIKey(ctx, &entity.CreateServiceAPIKeyRequest{ServiceName: "mesher"})
	require.NoError(t, err)
	assert.NotEqual(t, a.Key, b.Key)
	assert.Len(t, a.Key, len(KeyPrefix)+43)
}

func TestCreateServiceAPIKey_InvalidName(t *testing.T) {
	for _, name := range []string{"", "Mesher", "-mesher", "me sher", "mesher/x", strings.Repeat("a", 65)} {
		c, _ := newController(t)
		_, err := c.CreateServiceAPIKey(context.Background(), &entity.CreateServiceAPIKeyRequest{ServiceName: name})
		assert.ErrorIs(t, err, ErrInvalidServiceName, name)
	}
}

func TestCreateServiceAPIKey_Errors(t *testing.T) {
	ctx := context.Background()
	rq := &entity.CreateServiceAPIKeyRequest{ServiceName: "mesher"}

	t.Run("rand", func(t *testing.T) {
		c := New(Params{Rand: iotest.ErrReader(errBoom)})
		_, err := c.CreateServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, errBoom)
	})
	t.Run("service", func(t *testing.T) {
		c, d := newController(t)
		d.ids.EXPECT().NextID().Return(int64(1))
		d.services.EXPECT().FindOrCreateService(ctx, gomock.Any()).Return(model.Service{}, errBoom)
		_, err := c.CreateServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, errBoom)
	})
	t.Run("key", func(t *testing.T) {
		c, d := newController(t)
		d.ids.EXPECT().NextID().Return(int64(1)).Times(2)
		d.services.EXPECT().FindOrCreateService(ctx, gomock.Any()).Return(model.Service{ID: 1}, nil)
		d.services.EXPECT().CreateServiceAPIKey(ctx, gomock.Any()).Return(model.ServiceAPIKey{}, errBoom)
		_, err := c.CreateServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestRevokeServiceAPIKey(t *testing.T) {
	ctx := context.Background()
	rq := &entity.RevokeServiceAPIKeyRequest{ServiceName: "mesher", KeyID: 2}

	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{ID: 7}, nil)
		d.services.EXPECT().RevokeServiceAPIKey(ctx, int64(7), int64(2), _now).Return(nil)
		rs, err := c.RevokeServiceAPIKey(ctx, rq)
		require.NoError(t, err)
		assert.NotNil(t, rs)
	})
	t.Run("unknown service", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{}, repository.ErrNotFound)
		_, err := c.RevokeServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, ErrServiceNotFound)
	})
	t.Run("find error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{}, errBoom)
		_, err := c.RevokeServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, errBoom)
	})
	t.Run("unknown key", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{ID: 7}, nil)
		d.services.EXPECT().RevokeServiceAPIKey(ctx, int64(7), int64(2), _now).Return(repository.ErrNotFound)
		_, err := c.RevokeServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})
	t.Run("revoke error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByName(ctx, "mesher").Return(model.Service{ID: 7}, nil)
		d.services.EXPECT().RevokeServiceAPIKey(ctx, int64(7), int64(2), _now).Return(errBoom)
		_, err := c.RevokeServiceAPIKey(ctx, rq)
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestAuthenticateService(t *testing.T) {
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByActiveKeyHash(ctx, hashKey(_key)).Return(model.Service{ID: 7, Name: "mesher"}, nil)
		rs, err := c.AuthenticateService(ctx, &entity.AuthenticateServiceRequest{Key: _key})
		require.NoError(t, err)
		assert.Equal(t, entity.Service{ID: 7, Name: "mesher"}, rs.Service)
	})
	t.Run("not a key", func(t *testing.T) {
		c, _ := newController(t)
		_, err := c.AuthenticateService(ctx, &entity.AuthenticateServiceRequest{Key: "session-token"})
		assert.ErrorIs(t, err, ErrInvalidKey)
	})
	t.Run("unknown or revoked", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByActiveKeyHash(ctx, hashKey(_key)).Return(model.Service{}, repository.ErrNotFound)
		_, err := c.AuthenticateService(ctx, &entity.AuthenticateServiceRequest{Key: _key})
		assert.ErrorIs(t, err, ErrInvalidKey)
	})
	t.Run("error", func(t *testing.T) {
		c, d := newController(t)
		d.services.EXPECT().FindServiceByActiveKeyHash(ctx, hashKey(_key)).Return(model.Service{}, errBoom)
		_, err := c.AuthenticateService(ctx, &entity.AuthenticateServiceRequest{Key: _key})
		assert.ErrorIs(t, err, errBoom)
		assert.NotErrorIs(t, err, ErrInvalidKey)
	})
}
