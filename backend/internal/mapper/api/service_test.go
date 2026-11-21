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

func TestFromHTTPRequestToCreateServiceAPIKeyRequestEntity(t *testing.T) {
	var got *entity.CreateServiceAPIKeyRequest
	run(t, "/services/:service/keys", httptest.NewRequest(http.MethodGet, "/services/mesher/keys", nil), func(c fiber.Ctx) {
		got = FromHTTPRequestToCreateServiceAPIKeyRequestEntity(c)
	})
	assert.Equal(t, &entity.CreateServiceAPIKeyRequest{ServiceName: "mesher"}, got)

	run(t, "/keys", httptest.NewRequest(http.MethodGet, "/keys", nil), func(c fiber.Ctx) {
		got = FromHTTPRequestToCreateServiceAPIKeyRequestEntity(c)
	})
	assert.Nil(t, got)
}

func TestFromCreateServiceAPIKeyResponseEntityToHTTPResponse(t *testing.T) {
	got := FromCreateServiceAPIKeyResponseEntityToHTTPResponse(&entity.CreateServiceAPIKeyResponse{
		Service: entity.Service{ID: 7, Name: "mesher"},
		APIKey: entity.ServiceAPIKey{
			ID: 42, ServiceID: 7, Prefix: "tpk_abcdefgh", KeyHash: "secret-hash",
			CreatedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.FixedZone("x", 3600)),
		},
		Key: "tpk_abcdefghXYZ",
	})
	assert.JSONEq(t, `{"apiKey":{"id":"`+FromIDToBase62(42)+`","service":"mesher","prefix":"tpk_abcdefgh","createdAt":"2026-11-21T11:00:00Z"},"key":"tpk_abcdefghXYZ"}`, string(got))
}

func TestFromHTTPRequestToRevokeServiceAPIKeyRequestEntity(t *testing.T) {
	cases := map[string]*entity.RevokeServiceAPIKeyRequest{
		"/services/mesher/keys/" + FromIDToBase62(42): {ServiceName: "mesher", KeyID: 42},
		"/services/mesher/keys/0":                     nil,
	}
	for target, want := range cases {
		var got *entity.RevokeServiceAPIKeyRequest
		run(t, "/services/:service/keys/:id", httptest.NewRequest(http.MethodGet, target, nil), func(c fiber.Ctx) {
			got = FromHTTPRequestToRevokeServiceAPIKeyRequestEntity(c)
		})
		assert.Equal(t, want, got, target)
	}

	var got *entity.RevokeServiceAPIKeyRequest
	run(t, "/keys/:id", httptest.NewRequest(http.MethodGet, "/keys/"+FromIDToBase62(42), nil), func(c fiber.Ctx) {
		got = FromHTTPRequestToRevokeServiceAPIKeyRequestEntity(c)
	})
	assert.Nil(t, got, "missing service")
}

func TestFromHTTPRequestToAuthenticateServiceRequestEntity(t *testing.T) {
	cases := map[string]*entity.AuthenticateServiceRequest{
		"Bearer tpk_abc":   {Key: "tpk_abc"},
		"bearer  tpk_abc ": {Key: "tpk_abc"},
		"Bearer ":          nil,
		"Bearer":           nil,
		"Bearertpk_abc":    nil,
		"Basic dXNlcg==":   nil,
		"":                 nil,
	}
	for header, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set(fiber.HeaderAuthorization, header)
		}
		var got *entity.AuthenticateServiceRequest
		run(t, "/", req, func(c fiber.Ctx) {
			got = FromHTTPRequestToAuthenticateServiceRequestEntity(c)
		})
		assert.Equal(t, want, got, header)
	}
}
