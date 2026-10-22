// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/mustafaturan/monoflake"
	"github.com/stretchr/testify/assert"

	adminctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/admin"
	mockadmin "github.com/topupsh/topupsh-tracker/backend/internal/controller/admin/mocks"
	mockauth "github.com/topupsh/topupsh-tracker/backend/internal/controller/auth/mocks"
	mocksetup "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup/mocks"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

var _at = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

// newAdminApp is the API with a root session that is valid whenever a
// request carries _rootCookie.
func newAdminApp(t *testing.T) (*fiber.App, *mockadmin.MockController) {
	t.Helper()
	mc := gomock.NewController(t)
	setup := mocksetup.NewMockController(mc)
	setup.EXPECT().Authenticate(gomock.Any(), &entity.AuthenticateRootRequest{SessionToken: "root-session"}).Return(nil).AnyTimes()
	admin := mockadmin.NewMockController(mc)
	app := fiber.New()
	New(Params{Router: app, Auth: mockauth.NewMockController(mc), Setup: setup, Admin: admin})
	return app, admin
}

func TestAdminGetStats(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, admin := newAdminApp(t)
		admin.EXPECT().GetStats(gomock.Any(), &entity.GetAdminStatsRequest{}).
			Return(&entity.GetAdminStatsResponse{Users: 2, Days: []entity.AdminStatsDay{{Date: "2026-10-21", Signups: 3}}}, nil)
		res, body := send(t, app, http.MethodGet, "/admin/v1/stats", "", _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"stats":{"users":2,"signups":0,"days":[{"date":"2026-10-21","signups":3}]}}`, body)
	})
	t.Run("root only", func(t *testing.T) {
		app, _ := newAdminApp(t)
		res, _ := send(t, app, http.MethodGet, "/admin/v1/stats", "")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	})
	t.Run("error", func(t *testing.T) {
		app, admin := newAdminApp(t)
		admin.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
		res, body := send(t, app, http.MethodGet, "/admin/v1/stats", "", _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.NotContains(t, body, "boom")
	})
}

func TestAdminListUsers(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, admin := newAdminApp(t)
		admin.EXPECT().ListUsers(gomock.Any(), &entity.ListAdminUsersRequest{Query: "ann", Limit: 5}).
			Return(&entity.ListAdminUsersResponse{Users: []entity.AdminUser{{User: entity.User{ID: 62, Email: "ann@x", CreatedAt: _at}}}}, nil)
		res, body := send(t, app, http.MethodGet, "/admin/v1/users?q=ann&limit=5", "", _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, body, `"email":"ann@x"`)
	})
	t.Run("root only", func(t *testing.T) {
		app, _ := newAdminApp(t)
		res, _ := send(t, app, http.MethodGet, "/admin/v1/users", "")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	})
	t.Run("bad query", func(t *testing.T) {
		app, _ := newAdminApp(t)
		res, _ := send(t, app, http.MethodGet, "/admin/v1/users?limit=x", "", _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("error", func(t *testing.T) {
		app, admin := newAdminApp(t)
		admin.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
		res, body := send(t, app, http.MethodGet, "/admin/v1/users", "", _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.NotContains(t, body, "boom")
	})
}

func TestAdminGetUser(t *testing.T) {
	path := "/admin/v1/users/" + monoflake.ID(62).String()
	t.Run("ok", func(t *testing.T) {
		app, admin := newAdminApp(t)
		admin.EXPECT().GetUser(gomock.Any(), &entity.GetAdminUserRequest{UserID: 62}).
			Return(&entity.GetAdminUserResponse{User: entity.AdminUser{User: entity.User{ID: 62, Email: "ann@x", CreatedAt: _at}}}, nil)
		res, body := send(t, app, http.MethodGet, path, "", _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, body, `"createdAt":"2026-10-18T12:00:00Z"`)
	})
	t.Run("bad id", func(t *testing.T) {
		app, _ := newAdminApp(t)
		res, _ := send(t, app, http.MethodGet, "/admin/v1/users/0", "", _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	for err, status := range map[error]int{
		adminctrl.ErrUserNotFound: http.StatusNotFound,
		errors.New("boom"):        http.StatusInternalServerError,
	} {
		t.Run(err.Error(), func(t *testing.T) {
			app, admin := newAdminApp(t)
			admin.EXPECT().GetUser(gomock.Any(), gomock.Any()).Return(nil, err)
			res, body := send(t, app, http.MethodGet, path, "", _rootCookie)
			assert.Equal(t, status, res.StatusCode)
			assert.NotContains(t, body, "boom")
		})
	}
}
