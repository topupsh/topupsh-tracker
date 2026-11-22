// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

func TestFromHTTPRequestToSetServiceEnabledRequestEntity(t *testing.T) {
	read := func(route, target, body string) *entity.SetServiceEnabledRequest {
		var got *entity.SetServiceEnabledRequest
		run(t, route, httptest.NewRequest(http.MethodGet, target, strings.NewReader(body)), func(c fiber.Ctx) {
			got = FromHTTPRequestToSetServiceEnabledRequestEntity(c)
		})
		return got
	}
	route := "/services/:service/enabled"
	assert.Equal(t, &entity.SetServiceEnabledRequest{ServiceName: "mesher", Enabled: true}, read(route, "/services/mesher/enabled", `{"enabled":true}`))
	assert.Equal(t, &entity.SetServiceEnabledRequest{ServiceName: "mesher"}, read(route, "/services/mesher/enabled", `{"enabled":false}`))
	assert.Nil(t, read(route, "/services/mesher/enabled", `{}`), "enabled is required")
	assert.Nil(t, read(route, "/services/mesher/enabled", `nope`))
	assert.Nil(t, read("/enabled", "/enabled", `{"enabled":true}`))
}

func TestFromSetServiceEnabledResponseEntityToHTTPResponse(t *testing.T) {
	got := FromSetServiceEnabledResponseEntityToHTTPResponse(&entity.SetServiceEnabledResponse{Service: entity.Service{ID: 7, Name: "mesher", Enabled: true}})
	assert.JSONEq(t, `{"name":"mesher","enabled":true}`, string(got))
}

func TestFromHTTPRequestToSetOrgLimitsRequestEntity(t *testing.T) {
	read := func(route, target, body string) *entity.SetOrgLimitsRequest {
		var got *entity.SetOrgLimitsRequest
		run(t, route, httptest.NewRequest(http.MethodGet, target, strings.NewReader(body)), func(c fiber.Ctx) {
			got = FromHTTPRequestToSetOrgLimitsRequestEntity(c)
		})
		return got
	}
	route := "/orgs/:org/limits"
	monthly, user := int64(1000), int64(10)
	assert.Equal(t, &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1", MonthlyLimit: &monthly, UserLimit: &user},
		read(route, "/orgs/o1/limits", `{"service":"mesher","orgLimit":1000,"userLimit":10}`))
	assert.Equal(t, &entity.SetOrgLimitsRequest{ServiceName: "mesher", OrgID: "o1"},
		read(route, "/orgs/o1/limits", `{"service":"mesher","userLimit":null}`))
	assert.Nil(t, read(route, "/orgs/o1/limits", `{"orgLimit":1}`), "service is required")
	assert.Nil(t, read(route, "/orgs/o1/limits", `nope`))
	assert.Nil(t, read("/limits", "/limits", `{"service":"mesher"}`))
}

func TestFromSetOrgLimitsResponseEntityToHTTPResponse(t *testing.T) {
	user := int64(10)
	got := FromSetOrgLimitsResponseEntityToHTTPResponse(&entity.SetOrgLimitsResponse{
		Service:  entity.Service{Name: "mesher"},
		OrgLimit: entity.OrgLimit{OrgID: "o1", MonthlyLimit: 1000, UserLimit: &user},
	})
	assert.JSONEq(t, `{"service":"mesher","org":"o1","orgLimit":1000,"userLimit":10}`, string(got))

	got = FromSetOrgLimitsResponseEntityToHTTPResponse(&entity.SetOrgLimitsResponse{
		Service:  entity.Service{Name: "mesher"},
		OrgLimit: entity.OrgLimit{OrgID: "o1"},
	})
	assert.JSONEq(t, `{"service":"mesher","org":"o1","orgLimit":0,"userLimit":null}`, string(got))
}

func TestFromHTTPRequestToCreateGrantRequestEntity(t *testing.T) {
	read := func(body string) *entity.CreateGrantRequest {
		var got *entity.CreateGrantRequest
		run(t, "/grants", httptest.NewRequest(http.MethodGet, "/grants", strings.NewReader(body)), func(c fiber.Ctx) {
			got = FromHTTPRequestToCreateGrantRequestEntity(c)
		})
		return got
	}
	assert.Equal(t, &entity.CreateGrantRequest{ServiceName: "mesher", OrgID: "o1", UserID: "u1", Credits: 100},
		read(`{"service":"mesher","org":"o1","user":"u1","credits":100}`))
	assert.Nil(t, read(`{"org":"o1","user":"u1","credits":100}`), "service is required")
	assert.Nil(t, read(`nope`))
}

func TestFromCreateGrantResponseEntityToHTTPResponse(t *testing.T) {
	at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	got := FromCreateGrantResponseEntityToHTTPResponse(&entity.CreateGrantResponse{
		Service: entity.Service{Name: "mesher"},
		Bucket: entity.CreditBucket{
			ID: 42, OrgID: "o1", UserID: "u1", Amount: 100, Remaining: 100,
			CreatedAt: at, ExpiresAt: time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC),
		},
		LedgerEntry: entity.LedgerEntry{BalanceAfter: 150},
	})
	assert.JSONEq(t, `{"id":"`+FromIDToBase62(42)+`","service":"mesher","org":"o1","user":"u1","credits":100,"remaining":100,`+
		`"createdAt":"2026-11-21T12:00:00Z","expiresAt":"2026-12-22T00:00:00Z","balanceAfter":150}`, string(got))
}
