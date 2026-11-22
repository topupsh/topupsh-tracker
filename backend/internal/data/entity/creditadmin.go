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
		// MonthlyLimit is nil to use the service's default org limit.
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
		// RequestID makes the grant safe to retry; the controller makes one
		// when it is empty.
		RequestID   string
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

	// SetServiceDefaultsRequest sets a service's default limits, which
	// apply to orgs the admin set no limits for.
	SetServiceDefaultsRequest struct {
		ServiceName string
		OrgLimit    int64
		// UserLimit is nil for uncapped users.
		UserLimit *int64
	}

	// SetServiceDefaultsResponse is the service after the change.
	SetServiceDefaultsResponse struct {
		Service Service
	}

	// ListServicesRequest asks for every service.
	ListServicesRequest struct{}

	// ListServicesResponse is every service, ordered by name.
	ListServicesResponse struct {
		Services []ServiceWithKeys
	}

	// ServiceWithKeys is a service and its API keys, newest first,
	// revoked ones included.
	ServiceWithKeys struct {
		Service Service
		APIKeys []ServiceAPIKey
	}

	// ListOrgUsageRequest asks for a service's orgs.
	ListOrgUsageRequest struct {
		ServiceName string
	}

	// ListOrgUsageResponse is the service's orgs with limits of their own
	// or credits spent this month, ordered by org.
	ListOrgUsageResponse struct {
		Service Service
		Orgs    []OrgUsage
	}

	// OrgUsage is an org's limits and its monthly credits spent this UTC
	// month.
	OrgUsage struct {
		OrgLimit OrgLimit
		// Custom is whether the admin set the org's limits; false means
		// the service's defaults apply.
		Custom bool
		Used   int64
	}

	// ListUserUsageRequest asks for a service's users, of one org when
	// OrgID is set.
	ListUserUsageRequest struct {
		ServiceName string
		OrgID       string
	}

	// ListUserUsageResponse is the users with unexpired credits, ordered
	// by org and user. Truncated is set when there were more than shown.
	ListUserUsageResponse struct {
		Service   Service
		Users     []UserUsage
		Truncated bool
	}

	// UserUsage is a user's credits this UTC month.
	UserUsage struct {
		OrgID  string
		UserID string
		// MonthlyAllowance is the user's monthly credits under the
		// current limits; MonthlyUsed is what was spent of them.
		MonthlyAllowance int64
		MonthlyUsed      int64
		// Grants is what is left of the user's unexpired grants.
		Grants int64
		// Balance is the user's unexpired credits.
		Balance int64
	}
)
