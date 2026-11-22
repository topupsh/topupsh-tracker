// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

type (
	// SetServiceEnabledRequest switches credit limiting on or off for a
	// service.
	SetServiceEnabledRequest struct {
		ServiceName string
		Enabled     bool
	}

	// SetServiceEnabledResponse is the service after the switch.
	SetServiceEnabledResponse struct {
		Service Service
	}

	// SetOrgLimitsRequest sets a service's monthly limits for one org.
	SetOrgLimitsRequest struct {
		ServiceName string
		OrgID       string
		// MonthlyLimit is nil to use DefaultOrgMonthlyLimit.
		MonthlyLimit *int64
		// UserLimit is nil for uncapped users.
		UserLimit *int64
	}

	// SetOrgLimitsResponse is the org's limits after the change.
	SetOrgLimitsResponse struct {
		Service  Service
		OrgLimit OrgLimit
	}

	// CreateGrantRequest gives a user extra credits of a service.
	CreateGrantRequest struct {
		ServiceName string
		OrgID       string
		UserID      string
		Credits     int64
	}

	// CreateGrantResponse is the new grant bucket and its ledger entry.
	CreateGrantResponse struct {
		Service     Service
		Bucket      CreditBucket
		LedgerEntry LedgerEntry
	}
)
