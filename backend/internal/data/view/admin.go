// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// AdminStats is the stats over every account.
	AdminStats struct {
		Users   int64           `json:"users"`
		Signups int64           `json:"signups"`
		Days    []AdminStatsDay `json:"days"`
	}

	// AdminStatsDay is one UTC day of the admin stats.
	AdminStatsDay struct {
		Date    string `json:"date"`
		Signups int64  `json:"signups"`
	}

	// GetAdminStatsResponse wraps the admin stats.
	GetAdminStatsResponse struct {
		Stats AdminStats `json:"stats"`
	}

	// AdminUser is an account as root sees it.
	AdminUser struct {
		User
	}

	// ListAdminUsersResponse is a page of accounts, newest first.
	ListAdminUsersResponse struct {
		Users []AdminUser `json:"users"`
		// Next is the before cursor of the next page; empty on the last.
		Next string `json:"next"`
	}

	// GetAdminUserResponse wraps one account.
	GetAdminUserResponse struct {
		User AdminUser `json:"user"`
	}
)
