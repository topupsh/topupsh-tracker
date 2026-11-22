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
		assert.JSONEq(t, `{"name":"mesher","enabled":true}`, body)
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
