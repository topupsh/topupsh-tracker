// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// Service is a service, its credit limiting switch and its default
	// limits.
	Service struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
		// DefaultOrgLimit is the monthly org limit of orgs without limits
		// of their own.
		DefaultOrgLimit int64 `json:"defaultOrgLimit"`
		// DefaultUserLimit is the user limit of those orgs; null means
		// uncapped.
		DefaultUserLimit *int64 `json:"defaultUserLimit"`
	}

	// ServiceDetail is a service with its API keys, for the admin.
	ServiceDetail struct {
		Service
		CreatedAt string          `json:"createdAt"`
		APIKeys   []ServiceAPIKey `json:"apiKeys"`
	}

	// ListServicesResponse is the body of GET /api/v1/services.
	ListServicesResponse struct {
		Services []ServiceDetail `json:"services"`
	}

	// SetServiceDefaultsRequest is the body of PUT
	// /api/v1/services/{service}/defaults.
	SetServiceDefaultsRequest struct {
		OrgLimit *int64 `json:"orgLimit"`
		// UserLimit is omitted or null for uncapped users.
		UserLimit *int64 `json:"userLimit"`
	}

	// AdminOrgUsage is an org's limits and its monthly credits spent this UTC
	// month.
	AdminOrgUsage struct {
		Org       string `json:"org"`
		OrgLimit  int64  `json:"orgLimit"`
		UserLimit *int64 `json:"userLimit"`
		// Custom is false when the service's default limits apply.
		Custom bool  `json:"custom"`
		Used   int64 `json:"used"`
	}

	// ListOrgUsageResponse is the body of GET
	// /api/v1/services/{service}/orgs.
	ListOrgUsageResponse struct {
		Service string          `json:"service"`
		Orgs    []AdminOrgUsage `json:"orgs"`
	}

	// UserUsage is a user's credits this UTC month.
	UserUsage struct {
		Org              string `json:"org"`
		User             string `json:"user"`
		MonthlyAllowance int64  `json:"monthlyAllowance"`
		MonthlyUsed      int64  `json:"monthlyUsed"`
		Grants           int64  `json:"grants"`
		Balance          int64  `json:"balance"`
	}

	// ListUserUsageResponse is the body of GET
	// /api/v1/services/{service}/users.
	ListUserUsageResponse struct {
		Service string      `json:"service"`
		Users   []UserUsage `json:"users"`
		// Truncated is true when more users have credits than are listed.
		Truncated bool `json:"truncated"`
	}

	// SetServiceEnabledRequest is the body of PUT
	// /api/v1/services/{service}/enabled.
	SetServiceEnabledRequest struct {
		Enabled *bool `json:"enabled"`
	}

	// SetOrgLimitsRequest is the body of PUT /api/v1/orgs/{org}/limits.
	SetOrgLimitsRequest struct {
		Service string `json:"service"`
		// OrgLimit is omitted or null for the default.
		OrgLimit *int64 `json:"orgLimit"`
		// UserLimit is omitted or null for uncapped users.
		UserLimit *int64 `json:"userLimit"`
	}

	// OrgLimits are a service's monthly limits for one org.
	OrgLimits struct {
		Service   string `json:"service"`
		Org       string `json:"org"`
		OrgLimit  int64  `json:"orgLimit"`
		UserLimit *int64 `json:"userLimit"`
	}

	// CreateGrantRequest is the body of POST /api/v1/grants.
	CreateGrantRequest struct {
		// RequestID makes the grant safe to retry; topup.sh makes one when
		// it is empty.
		RequestID string `json:"requestId,omitempty"`
		Service   string `json:"service"`
		Org       string `json:"org"`
		User      string `json:"user"`
		Credits   int64  `json:"credits"`
	}

	// Grant is a bucket of extra credits an admin gave a user.
	Grant struct {
		ID        string `json:"id"`
		RequestID string `json:"requestId"`
		Service   string `json:"service"`
		Org       string `json:"org"`
		User      string `json:"user"`
		Credits   int64  `json:"credits"`
		Remaining int64  `json:"remaining"`
		CreatedAt string `json:"createdAt"`
		ExpiresAt string `json:"expiresAt"`
		// BalanceAfter is the user's unexpired credits with the grant.
		BalanceAfter int64 `json:"balanceAfter"`
	}
)
