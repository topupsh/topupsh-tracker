// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servicekeyctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey"
	mockservicekey "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey/mocks"
	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	mocksetup "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup/mocks"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

func newServiceKeyApp(t *testing.T) (*fiber.App, *mockservicekey.MockController, *mocksetup.MockController) {
	t.Helper()
	mc := gomock.NewController(t)
	keys := mockservicekey.NewMockController(mc)
	setup := mocksetup.NewMockController(mc)
	app := fiber.New()
	New(Params{Router: app, Setup: setup, ServiceKeys: keys})
	return app, keys, setup
}

func TestServiceKeys_RequireRoot(t *testing.T) {
	app, _, setup := newServiceKeyApp(t)
	res, _ := send(t, app, http.MethodPost, "/api/v1/services/mesher/keys", "")
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

	setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(setupctrl.ErrUnauthorized)
	res, _ = send(t, app, http.MethodDelete, "/api/v1/services/mesher/keys/abc", "", _rootCookie)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

	// A service key is not an admin credential.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/mesher/keys", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer tpk_x")
	res, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestServiceKeys_Create(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		app, keys, setup := newServiceKeyApp(t)
		setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		keys.EXPECT().CreateServiceAPIKey(gomock.Any(), &entity.CreateServiceAPIKeyRequest{ServiceName: "mesher"}).
			Return(&entity.CreateServiceAPIKeyResponse{
				Service: entity.Service{ID: 7, Name: "mesher"},
				APIKey:  entity.ServiceAPIKey{ID: 42, ServiceID: 7, Prefix: "tpk_abcdefgh", CreatedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)},
				Key:     "tpk_abcdefghXYZ",
			}, nil)
		res, body := send(t, app, http.MethodPost, "/api/v1/services/mesher/keys", "", _rootCookie)
		assert.Equal(t, http.StatusCreated, res.StatusCode)
		assert.Equal(t, fiber.MIMEApplicationJSON, res.Header.Get(fiber.HeaderContentType))
		assert.JSONEq(t, `{"apiKey":{"id":"`+mapper.FromIDToBase62(42)+`","service":"mesher","prefix":"tpk_abcdefgh","createdAt":"2026-11-21T12:00:00Z"},"key":"tpk_abcdefghXYZ"}`, body)
	})
	t.Run("invalid name", func(t *testing.T) {
		app, keys, setup := newServiceKeyApp(t)
		setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		keys.EXPECT().CreateServiceAPIKey(gomock.Any(), gomock.Any()).Return(nil, servicekeyctrl.ErrInvalidServiceName)
		res, body := send(t, app, http.MethodPost, "/api/v1/services/Mesher/keys", "", _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		assert.JSONEq(t, `{"error":"invalid service name"}`, body)
	})
	t.Run("server error", func(t *testing.T) {
		app, keys, setup := newServiceKeyApp(t)
		setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		keys.EXPECT().CreateServiceAPIKey(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodPost, "/api/v1/services/mesher/keys", "", _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to create api key"}`, body)
	})
	t.Run("missing service", func(t *testing.T) {
		app := fiber.New()
		h := &ServiceKey{}
		app.Post("/keys", h.Create)
		res, _ := send(t, app, http.MethodPost, "/keys", "")
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}

func TestServiceKeys_Delete(t *testing.T) {
	target := "/api/v1/services/mesher/keys/" + mapper.FromIDToBase62(42)
	t.Run("ok", func(t *testing.T) {
		app, keys, setup := newServiceKeyApp(t)
		setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		keys.EXPECT().RevokeServiceAPIKey(gomock.Any(), &entity.RevokeServiceAPIKeyRequest{ServiceName: "mesher", KeyID: 42}).
			Return(&entity.RevokeServiceAPIKeyResponse{}, nil)
		res, body := send(t, app, http.MethodDelete, target, "", _rootCookie)
		assert.Equal(t, http.StatusNoContent, res.StatusCode)
		assert.Empty(t, body)
	})
	for _, err := range []error{servicekeyctrl.ErrServiceNotFound, servicekeyctrl.ErrKeyNotFound} {
		t.Run(err.Error(), func(t *testing.T) {
			app, keys, setup := newServiceKeyApp(t)
			setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
			keys.EXPECT().RevokeServiceAPIKey(gomock.Any(), gomock.Any()).Return(nil, err)
			res, body := send(t, app, http.MethodDelete, target, "", _rootCookie)
			assert.Equal(t, http.StatusNotFound, res.StatusCode)
			assert.JSONEq(t, fmt.Sprintf(`{"error":%q}`, err.Error()), body)
		})
	}
	t.Run("server error", func(t *testing.T) {
		app, keys, setup := newServiceKeyApp(t)
		setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		keys.EXPECT().RevokeServiceAPIKey(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := send(t, app, http.MethodDelete, target, "", _rootCookie)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to revoke api key"}`, body)
	})
	t.Run("bad id", func(t *testing.T) {
		app, _, setup := newServiceKeyApp(t)
		setup.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		res, _ := send(t, app, http.MethodDelete, "/api/v1/services/mesher/keys/0", "", _rootCookie)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}

func TestAuthenticateService(t *testing.T) {
	newApp := func(t *testing.T) (*fiber.App, *mockservicekey.MockController) {
		t.Helper()
		keys := mockservicekey.NewMockController(gomock.NewController(t))
		app := fiber.New()
		app.Get("/x", AuthenticateService(keys), func(c fiber.Ctx) error {
			return c.SendString(fmt.Sprint(fiber.Locals[int64](c, mapper.LocalServiceID)))
		})
		return app, keys
	}
	call := func(t *testing.T, app *fiber.App, auth string) (*http.Response, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if auth != "" {
			req.Header.Set(fiber.HeaderAuthorization, auth)
		}
		res, err := app.Test(req)
		require.NoError(t, err)
		body := make([]byte, 256)
		n, _ := res.Body.Read(body)
		return res, string(body[:n])
	}

	t.Run("ok", func(t *testing.T) {
		app, keys := newApp(t)
		keys.EXPECT().AuthenticateService(gomock.Any(), &entity.AuthenticateServiceRequest{Key: "tpk_x"}).
			Return(&entity.AuthenticateServiceResponse{Service: entity.Service{ID: 7, Name: "mesher"}}, nil)
		res, body := call(t, app, "Bearer tpk_x")
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "7", body, "the calling service is in the request locals")
	})
	t.Run("missing header", func(t *testing.T) {
		app, _ := newApp(t)
		res, body := call(t, app, "")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
		assert.JSONEq(t, `{"error":"unauthorized"}`, body)
	})
	t.Run("invalid key", func(t *testing.T) {
		app, keys := newApp(t)
		keys.EXPECT().AuthenticateService(gomock.Any(), gomock.Any()).Return(nil, servicekeyctrl.ErrInvalidKey)
		res, body := call(t, app, "Bearer tpk_revoked")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
		assert.JSONEq(t, `{"error":"unauthorized"}`, body)
	})
	t.Run("server error", func(t *testing.T) {
		app, keys := newApp(t)
		keys.EXPECT().AuthenticateService(gomock.Any(), gomock.Any()).Return(nil, errBoom)
		res, body := call(t, app, "Bearer tpk_x")
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.JSONEq(t, `{"error":"failed to authenticate"}`, body)
	})
}
