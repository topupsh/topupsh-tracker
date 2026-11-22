// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"time"

	zlog "github.com/rs/zerolog/log"

	expiryctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/expiry"
)

// _jobTimeout bounds one run of a scheduled job.
const _jobTimeout = 10 * time.Minute

// expiryJob expires the credit buckets that are past their expiry. A
// failed run is logged; the next run picks up what it left.
func expiryJob(expiry expiryctrl.Controller) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), _jobTimeout)
		defer cancel()
		rs, err := expiry.ExpireBuckets(ctx)
		if err != nil {
			zlog.Error().Err(err).Msg("credits: expiry job failed")
			return
		}
		if rs.Buckets > 0 {
			zlog.Info().Int("buckets", rs.Buckets).Int64("credits", rs.Credits).Msg("credits: expired buckets")
		}
	}
}
