// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	creditadminctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/creditadmin"
	mockcreditadmin "github.com/topupsh/topupsh-tracker/backend/internal/controller/creditadmin/mocks"
	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	mocksetup "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup/mocks"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

func newCreditAdminApp(t *testing.T) (*fiber.App, *mockcreditadmin.MockController, *mocksetup.MockController) {
	t.Helper()
	mc := gomock.NewController(t)
	credits := mockcreditadmin.NewMockController(mc)
	setup := mocksetup.NewMockController(mc)
	app := fiber.New()
	New(Params{Router: app, Setup: setup, CreditAdmin: credits})
	return app, credits, setup
}

// asRoot lets the next request through the root check.
func asRoot(setup *mocksetup.MockController) {
	setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
}

func TestCreditAdmin_RequireRoot(t *testing.T) {
	app, _, setup := newCreditAdminApp(t)
	for _, rq := range []struct{ method, target string }{
		{http.MethodPut, "/api/v1/services/mesher/enabled"},
		{http.MethodPut, "/api/v1/orgs/o1/limits"},
		{http.MethodPost, "/api/v1/grants"},
		{http.MethodGet, "/api/v1/services"},
		{http.MethodPut, "/api/v1/services/mesher/defaults"},
		{http.MethodGet, "/api/v1/services/mesher/orgs"},
		{http.MethodGet, "/api/v1/services/mesher/users"},
	} {
		res, _ := send(t, app, rq.method, rq.target, `{}`)
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode, rq.target)
	}
	setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(setupctrl.ErrUnauthorized)
	res, _ := send(t, app, http.MethodPost, "/api/v1/grants", `{}`, _rootCookie)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestCreditAdmin_UpdateServiceEnabled(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetServiceEnabled(gomock.Any(), &entity.SetServiceEnabledRequest{ServiceName: "mesher", Enabled: true}).
			Return(&entity.SetServiceEnabledResponse{Service: entity.Service{ID: 7, Name: "mesher", Enabled: true}}, nil)
		res, body := send(t, app, http.MethodPut, "/api/v1/services/mesher/enabled", `{"enabled":true}`, _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, fiber.MIMEApplicationJSON, res.Header.Get(fiber.HeaderContentType))
		assert.JSONEq(t, `{"name":"mesher","enabled":true,"defaultOrgLimit":0,"defaultUserLimit":null}`, body)
	})
	t.Run("bad request", func(t *testing.T) {
		app, _, setup := newCreditAdminApp(t)
		asRoot(setup)
		res, _ := send(t, app, http.MethodPut, "/api/v1/services/mesher/enabled", `{}`, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("not found", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetServiceEnabled(gomock.Any(), gomock.Any()).Return(nil, creditadminctrl.ErrServiceNotFound)
		res, body := send(t, app, http.MethodPut, "/api/v1/services/nope/enabled", `{"enabled":false}`, _rootCookie)
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
		assert.JSONEq(t, `{"error":"service not found"}`, body)
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetServiceEnabled(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodPut, "/api/v1/services/mesher/enabled", `{"enabled":true}`, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to update service"}`, body)
	})
}

func TestCreditAdmin_UpdateOrgLimits(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		monthly := int64(1000)
		credits.EXPECT().SetOrgLimits(gomock.Any(), &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1", MonthlyLimit: &monthly}).
			Return(&entity.SetOrgLimitsResponse{
				Service:  entity.Service{Name: "mesher"},
				OrgLimit: entity.OrgLimit{OrgID: "o1", MonthlyLimit: 1000},
			}, nil)
		res, body := send(t, app, http.MethodPut, "/api/v1/orgs/o1/limits", `{"service":"mesher","orgLimit":1000}`, _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"service":"mesher","org":"o1","orgLimit":1000,"userLimit":null}`, body)
	})
	t.Run("bad request", func(t *testing.T) {
		app, _, setup := newCreditAdminApp(t)
		asRoot(setup)
		res, _ := send(t, app, http.MethodPut, "/api/v1/orgs/o1/limits", `{"orgLimit":1000}`, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("invalid limit", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetOrgLimits(gomock.Any(), gomock.Any()).Return(nil, creditadminctrl.ErrInvalidLimit)
		res, body := send(t, app, http.MethodPut, "/api/v1/orgs/o1/limits", `{"service":"mesher","orgLimit":-1}`, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		assert.JSONEq(t, `{"error":"limits must not be negative"}`, body)
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetOrgLimits(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodPut, "/api/v1/orgs/o1/limits", `{"service":"mesher"}`, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to update limits"}`, body)
	})
}

func TestCreditAdmin_CreateGrant(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().CreateGrant(gomock.Any(), &entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", UserID: "u1", Credits: 100}).
			Return(&entity.CreateGrantResponse{
				Service: entity.Service{Name: "mesher"},
				Bucket: entity.CreditBucket{
					ID: 42, OrgID: "o1", UserID: "u1", Amount: 100, Remaining: 100,
					CreatedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC),
					ExpiresAt: time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC),
				},
				LedgerEntry: entity.LedgerEntry{BalanceAfter: 100},
			}, nil)
		res, body := send(t, app, http.MethodPost, "/api/v1/grants", `{"service":"mesher","org":"o1","user":"u1","credits":100}`, _rootCookie)
		assert.Equal(t, http.StatusCreated, res.StatusCode)
		assert.JSONEq(t, `{"id":"`+mapper.FromIDToBase62(42)+`","service":"mesher","org":"o1","user":"u1","credits":100,"remaining":100,`+
			`"createdAt":"2026-11-21T12:00:00Z","expiresAt":"2026-12-22T00:00:00Z","balanceAfter":100}`, body)
	})
	t.Run("bad request", func(t *testing.T) {
		app, _, setup := newCreditAdminApp(t)
		asRoot(setup)
		res, _ := send(t, app, http.MethodPost, "/api/v1/grants", `nope`, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("invalid", func(t *testing.T) {
		for _, err := range []error{creditadminctrl.ErrInvalidOrg, creditadminctrl.ErrInvalidUser, creditadminctrl.ErrInvalidCredits} {
			app, credits, setup := newCreditAdminApp(t)
			asRoot(setup)
			credits.EXPECT().CreateGrant(gomock.Any(), gomock.Any()).Return(nil, err)
			res, body := send(t, app, http.MethodPost, "/api/v1/grants", `{"service":"mesher"}`, _rootCookie)
			assert.Equal(t, http.StatusBadRequest, res.StatusCode)
			assert.JSONEq(t, `{"error":"`+err.Error()+`"}`, body)
		}
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().CreateGrant(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodPost, "/api/v1/grants", `{"service":"mesher"}`, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to create grant"}`, body)
	})
}

func TestCreditAdmin_UpdateServiceDefaults(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		five := int64(5)
		credits.EXPECT().SetServiceDefaults(gomock.Any(), &entity.SetServiceDefaultsRequest{ServiceName: "mesher", OrgLimit: 100, UserLimit: &five}).
			Return(&entity.SetServiceDefaultsResponse{Service: entity.Service{Name: "mesher", DefaultOrgLimit: 100, DefaultUserLimit: &five}}, nil)
		res, body := send(t, app, http.MethodPut, "/api/v1/services/mesher/defaults", `{"orgLimit":100,"userLimit":5}`, _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"name":"mesher","enabled":false,"defaultOrgLimit":100,"defaultUserLimit":5}`, body)
	})
	t.Run("bad request", func(t *testing.T) {
		app, _, setup := newCreditAdminApp(t)
		asRoot(setup)
		res, _ := send(t, app, http.MethodPut, "/api/v1/services/mesher/defaults", `{"userLimit":5}`, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("invalid limit", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetServiceDefaults(gomock.Any(), gomock.Any()).Return(nil, creditadminctrl.ErrInvalidLimit)
		res, _ := send(t, app, http.MethodPut, "/api/v1/services/mesher/defaults", `{"orgLimit":-1}`, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().SetServiceDefaults(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodPut, "/api/v1/services/mesher/defaults", `{"orgLimit":1}`, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to update defaults"}`, body)
	})
}

func TestCreditAdmin_ListServices(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
		credits.EXPECT().ListServices(gomock.Any(), &entity.ListServicesRequest{}).Return(&entity.ListServicesResponse{
			Services: []entity.ServiceWithKeys{{
				Service: entity.Service{Name: "mesher", Enabled: true, CreatedAt: at},
				APIKeys: []entity.ServiceAPIKey{{ID: 1, Prefix: "tpk_abcd", CreatedAt: at, RevokedAt: &at}},
			}},
		}, nil)
		res, body := send(t, app, http.MethodGet, "/api/v1/services", ``, _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"services":[{"name":"mesher","enabled":true,"defaultOrgLimit":0,"defaultUserLimit":null,
			"createdAt":"2026-11-21T12:00:00Z","apiKeys":[{"id":"`+mapper.FromIDToBase62(1)+`","service":"mesher","prefix":"tpk_abcd",
			"createdAt":"2026-11-21T12:00:00Z","revokedAt":"2026-11-21T12:00:00Z"}]}]}`, body)
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().ListServices(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodGet, "/api/v1/services", ``, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to list services"}`, body)
	})
}

func TestCreditAdmin_ListOrgUsage(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		five := int64(5)
		credits.EXPECT().ListOrgUsage(gomock.Any(), &entity.ListOrgUsageRequest{ServiceName: "mesher"}).Return(&entity.ListOrgUsageResponse{
			Service: entity.Service{Name: "mesher"},
			Orgs: []entity.OrgUsage{
				{OrgLimit: entity.OrgLimit{OrgID: "o1", MonthlyLimit: 100, UserLimit: &five}, Custom: true, Used: 7},
			},
		}, nil)
		res, body := send(t, app, http.MethodGet, "/api/v1/services/mesher/orgs", ``, _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"service":"mesher","orgs":[{"org":"o1","orgLimit":100,"userLimit":5,"custom":true,"used":7}]}`, body)
	})
	t.Run("not found", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().ListOrgUsage(gomock.Any(), gomock.Any()).Return(nil, creditadminctrl.ErrServiceNotFound)
		res, _ := send(t, app, http.MethodGet, "/api/v1/services/nope/orgs", ``, _rootCookie)
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().ListOrgUsage(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodGet, "/api/v1/services/mesher/orgs", ``, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to list orgs"}`, body)
	})
}

func TestCreditAdmin_ListUserUsage(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().ListUserUsage(gomock.Any(), &entity.ListUserUsageRequest{ServiceName: "mesher", OrgID: "o1"}).Return(&entity.ListUserUsageResponse{
			Service:   entity.Service{Name: "mesher"},
			Users:     []entity.UserUsage{{OrgID: "o1", UserID: "u1", MonthlyAllowance: 10, MonthlyUsed: 3, Grants: 5, Balance: 12}},
			Truncated: true,
		}, nil)
		res, body := send(t, app, http.MethodGet, "/api/v1/services/mesher/users?org=o1", ``, _rootCookie)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"service":"mesher","truncated":true,"users":[{"org":"o1","user":"u1","monthlyAllowance":10,"monthlyUsed":3,"grants":5,"balance":12}]}`, body)
	})
	t.Run("invalid org", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().ListUserUsage(gomock.Any(), gomock.Any()).Return(nil, creditadminctrl.ErrInvalidOrg)
		res, _ := send(t, app, http.MethodGet, "/api/v1/services/mesher/users", ``, _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("server error", func(t *testing.T) {
		app, credits, setup := newCreditAdminApp(t)
		asRoot(setup)
		credits.EXPECT().ListUserUsage(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodGet, "/api/v1/services/mesher/users", ``, _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to list users"}`, body)
	})
}

func TestCreditAdmin_NilRequests(t *testing.T) {
	// The router never calls these without a service name; mounted bare,
	// the handlers refuse the request.
	h := &CreditAdmin{}
	app := fiber.New()
	app.Get("/orgs", h.ListOrgUsage)
	app.Get("/users", h.ListUserUsage)
	for _, target := range []string{"/orgs", "/users"} {
		res, _ := send(t, app, http.MethodGet, target, ``)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode, target)
	}
}
