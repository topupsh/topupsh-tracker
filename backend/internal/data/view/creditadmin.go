// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// Service is a service and its credit limiting switch.
	Service struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
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
		Service string `json:"service"`
		Org     string `json:"org"`
		User    string `json:"user"`
		Credits int64  `json:"credits"`
	}

	// Grant is a bucket of extra credits an admin gave a user.
	Grant struct {
		ID        string `json:"id"`
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
