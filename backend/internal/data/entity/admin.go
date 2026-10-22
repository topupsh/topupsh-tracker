// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

type (
	// GetAdminStatsRequest asks for the stats over every account.
	GetAdminStatsRequest struct{}

	// GetAdminStatsResponse is the stats over every account.
	GetAdminStatsResponse struct {
		// Users is how many accounts there are.
		Users int64
		// Signups is how many accounts registered over Days.
		Signups int64
		// Days has one entry per UTC day, oldest first, ending today.
		Days []AdminStatsDay
	}

	// AdminStatsDay is one UTC day of the admin stats.
	AdminStatsDay struct {
		// Date is the day, YYYY-MM-DD.
		Date string
		// Signups is how many accounts registered that day.
		Signups int64
	}

	// AdminUser is an account as root sees it.
	AdminUser struct {
		User
	}

	// ListAdminUsersRequest asks for a page of accounts, newest first.
	ListAdminUsersRequest struct {
		// Query keeps the users whose email or name contains it, or whose
		// ID it is; empty keeps every user.
		Query string
		// Before is the ID the page starts below; 0 starts at the newest.
		Before int64
		// Limit is the page size; 0 means the default.
		Limit int
	}

	// ListAdminUsersResponse is a page of accounts.
	ListAdminUsersResponse struct {
		Users []AdminUser
		// Next is the Before of the next page; 0 when this is the last.
		Next int64
	}

	// GetAdminUserRequest asks for one account.
	GetAdminUserRequest struct {
		UserID int64
	}

	// GetAdminUserResponse carries one account.
	GetAdminUserResponse struct {
		User AdminUser
	}
)
