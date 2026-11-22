// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	balancectrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/balance"
	mockbalance "github.com/topupsh/topupsh-tracker/backend/internal/controller/balance/mocks"
	servicekeyctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey"
	mockservicekey "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey/mocks"
	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	mocksetup "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup/mocks"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

var _viewer = entity.Viewer{ServiceID: 7, OrgID: "o1", UserID: "u1"}

// newBalanceApp returns the API with the service key "tpk_x" for service 7,
// the view token "view" for _viewer and the root cookie _rootCookie.
func newBalanceApp(t *testing.T) (*fiber.App, *mockbalance.MockController) {
	t.Helper()
	mc := gomock.NewController(t)
	balances := mockbalance.NewMockController(mc)
	keys := mockservicekey.NewMockController(mc)
	keys.EXPECT().AuthenticateService(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, rq *entity.AuthenticateServiceRequest) (*entity.AuthenticateServiceResponse, error) {
			if rq.Key != "tpk_x" {
				return nil, servicekeyctrl.ErrInvalidKey
			}
			return &entity.AuthenticateServiceResponse{Service: entity.Service{ID: 7, Name: "mesher"}}, nil
		}).AnyTimes()
	balances.EXPECT().AuthenticateViewer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, rq *entity.AuthenticateViewerRequest) (*entity.AuthenticateViewerResponse, error) {
			if rq.Token != "view" {
				return nil, balancectrl.ErrInvalidToken
			}
			return &entity.AuthenticateViewerResponse{Viewer: _viewer}, nil
		}).AnyTimes()
	setup := mocksetup.NewMockController(mc)
	setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, rq *entity.AuthenticateRootRequest) error {
			if rq.SessionToken != _rootCookie.Value {
				return setupctrl.ErrUnauthorized
			}
			return nil
		}).AnyTimes()
	app := fiber.New()
	New(Params{Router: app, ServiceKeys: keys, Setup: setup, Balances: balances})
	return app, balances
}

// read sends a GET with the bearer credential, if any, and the cookies.
func read(t *testing.T, app *fiber.App, target, bearer string, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if bearer != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+bearer)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := app.Test(req)
	require.NoError(t, err)
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res, string(b)
}

func TestAuthenticateReader(t *testing.T) {
	for _, target := range []string{"/api/v1/balances", "/api/v1/ledger"} {
		app, _ := newBalanceApp(t)
		for name, tc := range map[string]struct {
			bearer  string
			cookies []*http.Cookie
		}{
			"nothing":        {},
			"bad key":        {bearer: "tpk_bad"},
			"bad view token": {bearer: "nope"},
			"bad root":       {cookies: []*http.Cookie{{Name: _rootCookie.Name, Value: "x"}}},
		} {
			res, _ := read(t, app, target, tc.bearer, tc.cookies...)
			assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "%s %s", target, name)
		}
	}
}

func TestBalance_Get(t *testing.T) {
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	renews := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	ten := int64(10)
	balance := &entity.GetBalanceResponse{Balance: entity.Balance{
		Service: entity.Service{ID: 7, Name: "mesher", Enabled: true}, OrgID: "o1", UserID: "u1",
		Credits: 13, Available: 11,
		Monthly:  entity.MonthlyCredits{Limit: 10, Used: 3, Remaining: 7},
		Org:      entity.OrgCredits{Limit: 100, UserLimit: &ten, Used: 95, Remaining: 5},
		Grants:   []entity.CreditBucket{{ID: 64, Amount: 7, Remaining: 6, CreatedAt: month, ExpiresAt: renews}},
		RenewsAt: renews,
	}}
	want := `{"service":"mesher","org":"o1","user":"u1","limited":true,"balance":13,"available":11,
		"monthly":{"limit":10,"used":3,"remaining":7},
		"orgUsage":{"limit":100,"userLimit":10,"used":95,"remaining":5},
		"grants":[{"id":"00000000012","credits":7,"remaining":6,"createdAt":"2026-11-01T00:00:00Z","expiresAt":"2026-12-01T00:00:00Z"}],
		"renewsAt":"2026-12-01T00:00:00Z"}`

	for name, tc := range map[string]struct {
		target  string
		bearer  string
		cookies []*http.Cookie
		rq      entity.GetBalanceRequest
	}{
		"service key": {
			target: "/api/v1/balances?org=o1&user=u1", bearer: "tpk_x",
			rq: entity.GetBalanceRequest{Reader: entity.Reader{ServiceID: 7}, OrgID: "o1", UserID: "u1"},
		},
		"view token": {
			target: "/api/v1/balances", bearer: "view",
			rq: entity.GetBalanceRequest{Reader: entity.Reader{Viewer: &_viewer}},
		},
		"root": {
			target: "/api/v1/balances?service=mesher&org=o1&user=u1", cookies: []*http.Cookie{_rootCookie},
			rq: entity.GetBalanceRequest{Reader: entity.Reader{Root: true}, ServiceName: "mesher", OrgID: "o1", UserID: "u1"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			app, balances := newBalanceApp(t)
			balances.EXPECT().GetBalance(gomock.Any(), &tc.rq).Return(balance, nil)
			res, body := read(t, app, tc.target, tc.bearer, tc.cookies...)
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, fiber.MIMEApplicationJSON, res.Header.Get(fiber.HeaderContentType))
			assert.JSONEq(t, want, body)
		})
	}

	for err, status := range map[error]int{
		balancectrl.ErrForbidden:       http.StatusForbidden,
		balancectrl.ErrServiceNotFound: http.StatusNotFound,
		balancectrl.ErrViewerFilter:    http.StatusBadRequest,
		balancectrl.ErrServiceRequired: http.StatusBadRequest,
		balancectrl.ErrInvalidOrg:      http.StatusBadRequest,
		balancectrl.ErrInvalidUser:     http.StatusBadRequest,
		balancectrl.ErrInvalidLimit:    http.StatusBadRequest,
		errors.New("boom"):             http.StatusInternalServerError,
	} {
		app, balances := newBalanceApp(t)
		balances.EXPECT().GetBalance(gomock.Any(), gomock.Any()).Return(nil, err)
		res, body := read(t, app, "/api/v1/balances?org=o1&user=u1", "tpk_x")
		assert.Equal(t, status, res.StatusCode, err.Error())
		if status == http.StatusInternalServerError {
			assert.JSONEq(t, `{"error":"failed to read balance"}`, body)
		} else {
			assert.JSONEq(t, `{"error":"`+err.Error()+`"}`, body)
		}
	}
}

func TestBalance_ListLedgerEntries(t *testing.T) {
	at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	t.Run("ok", func(t *testing.T) {
		app, balances := newBalanceApp(t)
		balances.EXPECT().ListLedgerEntries(gomock.Any(), &entity.ListLedgerEntriesRequest{
			Reader: entity.Reader{ServiceID: 7}, OrgID: "o1", UserID: "u1", Cursor: 62, Limit: 1,
		}).Return(&entity.ListLedgerEntriesResponse{
			Entries: []entity.LedgerEntry{{
				ID: 9, ServiceName: "mesher", OrgID: "o1", UserID: "u1", Kind: entity.LedgerKindSpend,
				Amount: -1, BalanceAfter: 4, Actor: entity.ActorKindService, Operation: "tool", CreatedAt: at,
			}},
			NextCursor: 9,
		}, nil)
		res, body := read(t, app, "/api/v1/ledger?org=o1&user=u1&cursor=10&limit=1", "tpk_x")
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"entries":[{"id":"00000000009","service":"mesher","org":"o1","user":"u1","kind":"spend",
			"amount":-1,"balanceAfter":4,"actor":"mesher","operation":"tool","createdAt":"2026-11-21T12:00:00Z"}],
			"nextCursor":"00000000009"}`, body)
	})
	t.Run("view token", func(t *testing.T) {
		app, balances := newBalanceApp(t)
		balances.EXPECT().ListLedgerEntries(gomock.Any(), &entity.ListLedgerEntriesRequest{Reader: entity.Reader{Viewer: &_viewer}}).
			Return(&entity.ListLedgerEntriesResponse{Entries: []entity.LedgerEntry{}}, nil)
		res, body := read(t, app, "/api/v1/ledger", "view")
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"entries":[]}`, body)
	})
	t.Run("bad cursor", func(t *testing.T) {
		app, _ := newBalanceApp(t)
		res, _ := read(t, app, "/api/v1/ledger?limit=x", "tpk_x")
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("controller error", func(t *testing.T) {
		app, balances := newBalanceApp(t)
		balances.EXPECT().ListLedgerEntries(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))
		res, body := read(t, app, "/api/v1/ledger", "", _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to read ledger"}`, body)
	})
}

func TestBalance_CreateViewToken(t *testing.T) {
	exp := time.Date(2026, 11, 21, 13, 0, 0, 0, time.UTC)
	t.Run("ok", func(t *testing.T) {
		app, balances := newBalanceApp(t)
		balances.EXPECT().CreateViewToken(gomock.Any(), &entity.CreateViewTokenRequest{ServiceID: 7, OrgID: "o1", UserID: "u1"}).
			Return(&entity.CreateViewTokenResponse{Token: "tok", ExpiresAt: exp}, nil)
		res, body := sendAsService(t, app, "/api/v1/view-tokens", "tpk_x", `{"org":"o1","user":"u1"}`)
		assert.Equal(t, http.StatusCreated, res.StatusCode)
		assert.JSONEq(t, `{"token":"tok","expiresAt":"2026-11-21T13:00:00Z"}`, body)
	})
	t.Run("needs a service key", func(t *testing.T) {
		app, _ := newBalanceApp(t)
		res, _ := sendAsService(t, app, "/api/v1/view-tokens", "view", `{"org":"o1","user":"u1"}`)
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "a view token can't mint view tokens")
	})
	t.Run("bad request", func(t *testing.T) {
		app, _ := newBalanceApp(t)
		res, _ := sendAsService(t, app, "/api/v1/view-tokens", "tpk_x", `nope`)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	t.Run("invalid", func(t *testing.T) {
		app, balances := newBalanceApp(t)
		balances.EXPECT().CreateViewToken(gomock.Any(), gomock.Any()).Return(nil, balancectrl.ErrInvalidOrg)
		res, body := sendAsService(t, app, "/api/v1/view-tokens", "tpk_x", `{"user":"u1"}`)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		assert.JSONEq(t, `{"error":"invalid org"}`, body)
	})
}

func TestBalance_GetWithoutReader(t *testing.T) {
	app := fiber.New()
	app.Get("/balances", (&Balance{}).Get)
	res, _ := read(t, app, "/balances", "")
	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "routes always authenticate a reader first")
}
