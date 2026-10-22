// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/mustafaturan/monoflake"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/view"
)

// FromHTTPRequestToGetAdminStatsRequestEntity reads GET /admin/v1/stats,
// which takes nothing.
func FromHTTPRequestToGetAdminStatsRequestEntity(_ fiber.Ctx) *entity.GetAdminStatsRequest {
	return &entity.GetAdminStatsRequest{}
}

// FromGetAdminStatsResponseEntityToHTTPResponse renders the admin stats.
func FromGetAdminStatsResponseEntityToHTTPResponse(rs *entity.GetAdminStatsResponse) []byte {
	days := make([]view.AdminStatsDay, len(rs.Days))
	for i, d := range rs.Days {
		days[i] = view.AdminStatsDay{Date: d.Date, Signups: d.Signups}
	}
	return marshal(view.GetAdminStatsResponse{Stats: view.AdminStats{
		Users:   rs.Users,
		Signups: rs.Signups,
		Days:    days,
	}})
}

// FromHTTPRequestToListAdminUsersRequestEntity reads the optional q
// search, before cursor and limit of GET /admin/v1/users.
func FromHTTPRequestToListAdminUsersRequestEntity(c fiber.Ctx) *entity.ListAdminUsersRequest {
	before, limit, ok := pageQuery(c)
	if !ok {
		return nil
	}
	return &entity.ListAdminUsersRequest{Query: c.Query("q"), Before: before, Limit: limit}
}

// FromListAdminUsersResponseEntityToHTTPResponse renders a page of users.
func FromListAdminUsersResponseEntityToHTTPResponse(rs *entity.ListAdminUsersResponse) []byte {
	out := view.ListAdminUsersResponse{Users: make([]view.AdminUser, len(rs.Users))}
	for i, u := range rs.Users {
		out.Users[i] = FromAdminUserEntityToAdminUserView(u)
	}
	if rs.Next > 0 {
		out.Next = FromIDToBase62(rs.Next)
	}
	return marshal(out)
}

// FromHTTPRequestToGetAdminUserRequestEntity reads the user ID of GET
// /admin/v1/users/:id.
func FromHTTPRequestToGetAdminUserRequestEntity(c fiber.Ctx) *entity.GetAdminUserRequest {
	id := paramUserID(c)
	if id == 0 {
		return nil
	}
	return &entity.GetAdminUserRequest{UserID: id}
}

// FromGetAdminUserResponseEntityToHTTPResponse renders one user.
func FromGetAdminUserResponseEntityToHTTPResponse(rs *entity.GetAdminUserResponse) []byte {
	return marshal(view.GetAdminUserResponse{User: FromAdminUserEntityToAdminUserView(rs.User)})
}

// FromAdminUserEntityToAdminUserView maps an admin user to its JSON view.
func FromAdminUserEntityToAdminUserView(e entity.AdminUser) view.AdminUser {
	return view.AdminUser{User: FromUserEntityToUserView(e.User)}
}

// paramUserID is the base62 user ID in the :id route parameter; 0 when it
// is missing or not positive. Other junk decodes to an ID no user has.
func paramUserID(c fiber.Ctx) int64 {
	return max(monoflake.IDFromBase62(c.Params("id")).Int64(), 0)
}
