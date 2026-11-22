// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package expiry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	repomocks "github.com/topupsh/topupsh-tracker/backend/internal/repository/mocks"
	idgenmocks "github.com/topupsh/topupsh-tracker/backend/internal/service/idgen/mocks"
)

var (
	_now    = time.Date(2026, 12, 1, 0, 0, 0, 0, time.FixedZone("x", 3600))
	errBoom = errors.New("boom")
)

func newController(t *testing.T) (Controller, *repomocks.MockExpiryRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	expiry := repomocks.NewMockExpiryRepository(ctrl)
	ids := idgenmocks.NewMockService(ctrl)
	ids.EXPECT().NextID().Return(int64(42)).AnyTimes()
	return New(Params{Expiry: expiry, IDGen: ids, Now: func() time.Time { return _now }}), expiry
}

func TestNew_Defaults(t *testing.T) {
	c := New(Params{}).(*controller)
	assert.WithinDuration(t, time.Now(), c.now(), time.Second)
}

func TestSpec_RunsHourlyInUTC(t *testing.T) {
	s, err := cron.ParseStandard(Spec)
	require.NoError(t, err)
	from := time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), s.Next(from), "runs at the 00:00 UTC expiries")
	assert.Equal(t, time.Date(2026, 12, 1, 1, 0, 0, 0, time.UTC), s.Next(from.Add(time.Minute)))
}

func TestExpireBuckets(t *testing.T) {
	ctx := context.Background()
	t.Run("expired", func(t *testing.T) {
		c, expiry := newController(t)
		expiry.EXPECT().ExpireBuckets(ctx, _now.UTC(), gomock.Any()).
			DoAndReturn(func(_ context.Context, at time.Time, next func() int64) (repository.ExpiryResult, error) {
				assert.Equal(t, time.UTC, at.Location(), "months follow UTC")
				assert.Equal(t, int64(42), next(), "the repository gets the ID generator")
				return repository.ExpiryResult{Buckets: 2, Credits: 9}, nil
			})
		rs, err := c.ExpireBuckets(ctx)
		require.NoError(t, err)
		assert.Equal(t, &entity.ExpireBucketsResponse{Buckets: 2, Credits: 9}, rs)
	})
	t.Run("error", func(t *testing.T) {
		c, expiry := newController(t)
		expiry.EXPECT().ExpireBuckets(ctx, _now.UTC(), gomock.Any()).Return(repository.ExpiryResult{}, errBoom)
		_, err := c.ExpireBuckets(ctx)
		assert.ErrorIs(t, err, errBoom)
	})
}
