// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	expirymocks "github.com/topupsh/topupsh-tracker/backend/internal/controller/expiry/mocks"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

func TestExpiryJob(t *testing.T) {
	for name, rs := range map[string]*entity.ExpireBucketsResponse{
		"expired": {Buckets: 2, Credits: 9},
		"nothing": {},
	} {
		t.Run(name, func(t *testing.T) {
			expiry := expirymocks.NewMockController(gomock.NewController(t))
			expiry.EXPECT().ExpireBuckets(gomock.Any()).DoAndReturn(func(ctx context.Context) (*entity.ExpireBucketsResponse, error) {
				deadline, ok := ctx.Deadline()
				assert.True(t, ok, "a run is bounded")
				assert.WithinDuration(t, time.Now().Add(_jobTimeout), deadline, time.Minute)
				return rs, nil
			})
			expiryJob(expiry)()
		})
	}
	t.Run("error", func(t *testing.T) {
		expiry := expirymocks.NewMockController(gomock.NewController(t))
		expiry.EXPECT().ExpireBuckets(gomock.Any()).Return(nil, errors.New("boom"))
		expiryJob(expiry)()
	})
}
