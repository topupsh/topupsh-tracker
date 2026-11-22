// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package expiry expires credit buckets on schedule.
package expiry

//go:generate mockgen -source=expiry.go -destination=mocks/expiry.go -package=mocks

import (
	"context"
	"fmt"
	"time"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/idgen"
)

// Spec is the job's cron schedule, in UTC: every hour on the hour. Buckets
// expire at 00:00 UTC (monthly ones on the 1st, grants a month after they
// were given), and the other runs catch up after downtime.
const Spec = "0 * * * *"

type (
	// Controller expires credit buckets.
	Controller interface {
		// ExpireBuckets takes what is left of every bucket that has
		// expired, writing an expiry ledger entry for each. It is safe to
		// run again and on several replicas at once.
		ExpireBuckets(ctx context.Context) (*entity.ExpireBucketsResponse, error)
	}

	// Params are the controller's dependencies.
	Params struct {
		Expiry repository.ExpiryRepository
		IDGen  idgen.Service
		Now    func() time.Time
	}

	controller struct {
		expiry repository.ExpiryRepository
		ids    idgen.Service
		now    func() time.Time
	}
)

// New returns the expiry controller.
func New(p Params) Controller {
	c := &controller{expiry: p.Expiry, ids: p.IDGen, now: p.Now}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

func (c *controller) ExpireBuckets(ctx context.Context) (*entity.ExpireBucketsResponse, error) {
	rs, err := c.expiry.ExpireBuckets(ctx, c.now().UTC(), c.ids.NextID)
	if err != nil {
		return nil, fmt.Errorf("expire buckets: %w", err)
	}
	return &entity.ExpireBucketsResponse{Buckets: rs.Buckets, Credits: rs.Credits}, nil
}
