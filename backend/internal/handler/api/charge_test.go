// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chargectrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/charge"
	mockcharge "github.com/topupsh/topupsh-tracker/backend/internal/controller/charge/mocks"
	servicekeyctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey"
	mockservicekey "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey/mocks"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

// newChargeApp returns the API with a service key "tpk_x" for service 7.
func newChargeApp(t *testing.T) (*fiber.App, *mockcharge.MockController) {
	t.Helper()
	mc := gomock.NewController(t)
	charges := mockcharge.NewMockController(mc)
	keys := mockservicekey.NewMockController(mc)
	keys.EXPECT().AuthenticateService(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, rq *entity.AuthenticateServiceRequest) (*entity.AuthenticateServiceResponse, error) {
			if rq.Key != "tpk_x" {
				return nil, servicekeyctrl.ErrInvalidKey
			}
			return &entity.AuthenticateServiceResponse{Service: entity.Service{ID: 7, Name: "mesher"}}, nil
		}).AnyTimes()
	app := fiber.New()
	New(Params{Router: app, ServiceKeys: keys, Charges: charges})
	return app, charges
}

func sendAsService(t *testing.T, app *fiber.App, target, key, body string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+key)
	res, err := app.Test(req)
	require.NoError(t, err)
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res, string(b)
}

func TestCharge_RequireServiceKey(t *testing.T) {
	app, _ := newChargeApp(t)
	for _, target := range []string{"/api/v1/charges", "/api/v1/refunds"} {
		res, _ := sendAsService(t, app, target, "tpk_bad", `{}`)
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode, target)
	}
}

func TestCharge_Create(t *testing.T) {
	body := `{"requestId":"r1","org":"o1","user":"u1","credits":2,"operation":"tool"}`
	t.Run("charged", func(t *testing.T) {
		app, charges := newChargeApp(t)
		charges.EXPECT().CreateCharge(gomock.Any(), &entity.CreateChargeRequest{
			ServiceID: 7, RequestID: "r1", OrgID: "o1", UserID: "u1", Credits: 2, Operation: "tool",
		}).Return(&entity.CreateChargeResponse{Limited: true, Charge: entity.Charge{RequestID: "r1", Balance: 8, OrgRemaining: 90}}, nil)
		res, got := sendAsService(t, app, "/api/v1/charges", "tpk_x", body)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, fiber.MIMEApplicationJSON, res.Header.Get(fiber.HeaderContentType))
		assert.JSONEq(t, `{"requestId":"r1","charged":true,"limited":true,"balance":8,"orgRemaining":90}`, got)
	})
	t.Run("refused", func(t *testing.T) {
		app, charges := newChargeApp(t)
		charges.EXPECT().CreateCharge(gomock.Any(), gomock.Any()).Return(&entity.CreateChargeResponse{
			Limited: true, RenewsAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			Charge: entity.Charge{RequestID: "r1", Refusal: entity.RefusalUserLimit, Balance: 1, OrgRemaining: 90},
		}, nil)
		res, got := sendAsService(t, app, "/api/v1/charges", "tpk_x", body)
		assert.Equal(t, http.StatusPaymentRequired, res.StatusCode)
		assert.JSONEq(t, `{"requestId":"r1","charged":false,"limited":true,"reason":"user_limit","renewsAt":"2026-12-01T00:00:00Z","balance":1,"orgRemaining":90}`, got)
	})
	t.Run("bad request", func(t *testing.T) {
		app, _ := newChargeApp(t)
		res, _ := sendAsService(t, app, "/api/v1/charges", "tpk_x", `nope`)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	for err, status := range map[error]int{
		chargectrl.ErrInvalidRequestID: http.StatusBadRequest,
		chargectrl.ErrInvalidOrg:       http.StatusBadRequest,
		chargectrl.ErrInvalidUser:      http.StatusBadRequest,
		chargectrl.ErrInvalidCredits:   http.StatusBadRequest,
		chargectrl.ErrInvalidOperation: http.StatusBadRequest,
		chargectrl.ErrServiceNotFound:  http.StatusNotFound,
		chargectrl.ErrRequestConflict:  http.StatusConflict,
		errBoom:                        http.StatusInternalServerError,
	} {
		t.Run(err.Error(), func(t *testing.T) {
			app, charges := newChargeApp(t)
			charges.EXPECT().CreateCharge(gomock.Any(), gomock.Any()).Return(nil, err)
			res, got := sendAsService(t, app, "/api/v1/charges", "tpk_x", body)
			assert.Equal(t, status, res.StatusCode)
			if status == http.StatusInternalServerError {
				assert.JSONEq(t, `{"error":"failed to charge"}`, got)
			} else {
				assert.JSONEq(t, `{"error":"`+err.Error()+`"}`, got)
			}
		})
	}
}

func TestCharge_CreateRefund(t *testing.T) {
	t.Run("refunded", func(t *testing.T) {
		app, charges := newChargeApp(t)
		at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
		charges.EXPECT().CreateRefund(gomock.Any(), &entity.CreateRefundRequest{ServiceID: 7, RequestID: "f1", ChargeRequestID: "r1"}).
			Return(&entity.CreateRefundResponse{Limited: true, Refund: entity.Refund{RequestID: "f1", ChargeRequestID: "r1", Credits: 2, Balance: 9, CreatedAt: at}}, nil)
		res, got := sendAsService(t, app, "/api/v1/refunds", "tpk_x", `{"requestId":"f1","chargeRequestId":"r1"}`)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.JSONEq(t, `{"requestId":"f1","chargeRequestId":"r1","limited":true,"credits":2,"refundedAt":"2026-11-21T12:00:00Z","balance":9}`, got)
	})
	t.Run("bad request", func(t *testing.T) {
		app, _ := newChargeApp(t)
		res, _ := sendAsService(t, app, "/api/v1/refunds", "tpk_x", `nope`)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
	for err, status := range map[error]int{
		chargectrl.ErrInvalidRequestID:       http.StatusBadRequest,
		chargectrl.ErrInvalidChargeRequestID: http.StatusBadRequest,
		chargectrl.ErrChargeNotFound:         http.StatusNotFound,
		chargectrl.ErrNotCharged:             http.StatusConflict,
		chargectrl.ErrRequestConflict:        http.StatusConflict,
		errBoom:                              http.StatusInternalServerError,
	} {
		t.Run(err.Error(), func(t *testing.T) {
			app, charges := newChargeApp(t)
			charges.EXPECT().CreateRefund(gomock.Any(), gomock.Any()).Return(nil, err)
			res, _ := sendAsService(t, app, "/api/v1/refunds", "tpk_x", `{"requestId":"r1"}`)
			assert.Equal(t, status, res.StatusCode)
		})
	}
}
