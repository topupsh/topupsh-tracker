// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

// ExpireBucketsResponse is what an expiry run took: the buckets it expired
// and the credits that were left in them.
type ExpireBucketsResponse struct {
	Buckets int
	Credits int64
}
