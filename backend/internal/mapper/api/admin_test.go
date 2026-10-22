// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

func TestFromHTTPRequestToGetAdminStatsRequestEntity(t *testing.T) {
	assert.Equal(t, &entity.GetAdminStatsRequest{}, FromHTTPRequestToGetAdminStatsRequestEntity(nil))
}

func TestFromGetAdminStatsResponseEntityToHTTPResponse(t *testing.T) {
	got := FromGetAdminStatsResponseEntityToHTTPResponse(&entity.GetAdminStatsResponse{
		Users: 3, Signups: 2,
		Days: []entity.AdminStatsDay{{Date: "2026-10-21", Signups: 2}},
	})
	assert.JSONEq(t, `{"stats":{"users":3,"signups":2,"days":[{"date":"2026-10-21","signups":2}]}}`, string(got))
}

func TestFromHTTPRequestToListAdminUsersRequestEntity(t *testing.T) {
	cases := map[string]*entity.ListAdminUsersRequest{
		"/":                              {},
		"/?q=ann&limit=5":                {Query: "ann", Limit: 5},
		"/?before=" + FromIDToBase62(42): {Before: 42},
		"/?limit=x":                      nil,
		"/?before=0":                     nil,
	}
	for target, want := range cases {
		var got *entity.ListAdminUsersRequest
		run(t, "/", httptest.NewRequest(http.MethodGet, target, nil), func(c fiber.Ctx) {
			got = FromHTTPRequestToListAdminUsersRequestEntity(c)
		})
		assert.Equal(t, want, got, target)
	}
}

func TestFromListAdminUsersResponseEntityToHTTPResponse(t *testing.T) {
	at := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	got := FromListAdminUsersResponseEntityToHTTPResponse(&entity.ListAdminUsersResponse{
		Users: []entity.AdminUser{{User: entity.User{ID: 62, Email: "a@b.c", CreatedAt: at}}},
		Next:  62,
	})
	assert.JSONEq(t, `{"users":[{"id":"00000000010","email":"a@b.c","name":"","picture":"","provider":"","createdAt":"2026-10-19T04:00:00Z"}],"next":"00000000010"}`, string(got))

	got = FromListAdminUsersResponseEntityToHTTPResponse(&entity.ListAdminUsersResponse{})
	assert.JSONEq(t, `{"users":[],"next":""}`, string(got))
}

func TestFromHTTPRequestToGetAdminUserRequestEntity(t *testing.T) {
	cases := map[string]*entity.GetAdminUserRequest{
		"/users/" + FromIDToBase62(42): {UserID: 42},
		"/users/0":                     nil,
	}
	for target, want := range cases {
		var got *entity.GetAdminUserRequest
		run(t, "/users/:id", httptest.NewRequest(http.MethodGet, target, nil), func(c fiber.Ctx) {
			got = FromHTTPRequestToGetAdminUserRequestEntity(c)
		})
		assert.Equal(t, want, got, target)
	}
}

func TestFromGetAdminUserResponseEntityToHTTPResponse(t *testing.T) {
	at := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	got := FromGetAdminUserResponseEntityToHTTPResponse(&entity.GetAdminUserResponse{User: entity.AdminUser{User: entity.User{ID: 62, CreatedAt: at}}})
	assert.JSONEq(t, `{"user":{"id":"00000000010","email":"","name":"","picture":"","provider":"","createdAt":"2026-10-19T04:00:00Z"}}`, string(got))
}
