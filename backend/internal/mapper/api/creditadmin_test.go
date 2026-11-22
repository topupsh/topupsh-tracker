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
	assert.JSONEq(t, `{"name":"mesher","enabled":true,"defaultOrgLimit":0,"defaultUserLimit":null}`, string(got))
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
	assert.Equal(t, &entity.CreateGrantRequest{RequestID: "g1", ServiceName: "mesher", OrgID: "o1", UserID: "u1", Credits: 100},
		read(`{"requestId":"g1","service":"mesher","org":"o1","user":"u1","credits":100}`))
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
		LedgerEntry: entity.LedgerEntry{RequestID: "g1", BalanceAfter: 150},
	})
	assert.JSONEq(t, `{"id":"`+FromIDToBase62(42)+`","requestId":"g1","service":"mesher","org":"o1","user":"u1","credits":100,"remaining":100,`+
		`"createdAt":"2026-11-21T12:00:00Z","expiresAt":"2026-12-22T00:00:00Z","balanceAfter":150}`, string(got))
}

func TestFromHTTPRequestToSetServiceDefaultsRequestEntity(t *testing.T) {
	read := func(route, target, body string) *entity.SetServiceDefaultsRequest {
		var got *entity.SetServiceDefaultsRequest
		run(t, route, httptest.NewRequest(http.MethodGet, target, strings.NewReader(body)), func(c fiber.Ctx) {
			got = FromHTTPRequestToSetServiceDefaultsRequestEntity(c)
		})
		return got
	}
	route := "/services/:service/defaults"
	user := int64(10)
	assert.Equal(t, &entity.SetServiceDefaultsRequest{ServiceName: "mesher", OrgLimit: 100, UserLimit: &user},
		read(route, "/services/mesher/defaults", `{"orgLimit":100,"userLimit":10}`))
	assert.Equal(t, &entity.SetServiceDefaultsRequest{ServiceName: "mesher"},
		read(route, "/services/mesher/defaults", `{"orgLimit":0,"userLimit":null}`))
	assert.Nil(t, read(route, "/services/mesher/defaults", `{"userLimit":1}`), "orgLimit is required")
	assert.Nil(t, read(route, "/services/mesher/defaults", `nope`))
	assert.Nil(t, read("/defaults", "/defaults", `{"orgLimit":1}`))
}

func TestFromSetServiceDefaultsResponseEntityToHTTPResponse(t *testing.T) {
	user := int64(10)
	got := FromSetServiceDefaultsResponseEntityToHTTPResponse(&entity.SetServiceDefaultsResponse{
		Service: entity.Service{Name: "mesher", DefaultOrgLimit: 100, DefaultUserLimit: &user},
	})
	assert.JSONEq(t, `{"name":"mesher","enabled":false,"defaultOrgLimit":100,"defaultUserLimit":10}`, string(got))
}

func TestFromHTTPRequestToListServicesRequestEntity(t *testing.T) {
	var got *entity.ListServicesRequest
	run(t, "/services", httptest.NewRequest(http.MethodGet, "/services", nil), func(c fiber.Ctx) {
		got = FromHTTPRequestToListServicesRequestEntity(c)
	})
	assert.Equal(t, &entity.ListServicesRequest{}, got)
}

func TestFromListServicesResponseEntityToHTTPResponse(t *testing.T) {
	at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	got := FromListServicesResponseEntityToHTTPResponse(&entity.ListServicesResponse{Services: []entity.ServiceWithKeys{
		{
			Service: entity.Service{Name: "mesher", Enabled: true, CreatedAt: at},
			APIKeys: []entity.ServiceAPIKey{
				{ID: 2, Prefix: "tpk_new", CreatedAt: at},
				{ID: 1, Prefix: "tpk_old", CreatedAt: at, RevokedAt: &at},
			},
		},
		{Service: entity.Service{Name: "tmphost", CreatedAt: at}},
	}})
	assert.JSONEq(t, `{"services":[
		{"name":"mesher","enabled":true,"defaultOrgLimit":0,"defaultUserLimit":null,"createdAt":"2026-11-21T12:00:00Z","apiKeys":[
			{"id":"`+FromIDToBase62(2)+`","service":"mesher","prefix":"tpk_new","createdAt":"2026-11-21T12:00:00Z"},
			{"id":"`+FromIDToBase62(1)+`","service":"mesher","prefix":"tpk_old","createdAt":"2026-11-21T12:00:00Z","revokedAt":"2026-11-21T12:00:00Z"}]},
		{"name":"tmphost","enabled":false,"defaultOrgLimit":0,"defaultUserLimit":null,"createdAt":"2026-11-21T12:00:00Z","apiKeys":[]}]}`, string(got))
}

func TestFromHTTPRequestToListOrgUsageRequestEntity(t *testing.T) {
	read := func(route, target string) *entity.ListOrgUsageRequest {
		var got *entity.ListOrgUsageRequest
		run(t, route, httptest.NewRequest(http.MethodGet, target, nil), func(c fiber.Ctx) {
			got = FromHTTPRequestToListOrgUsageRequestEntity(c)
		})
		return got
	}
	assert.Equal(t, &entity.ListOrgUsageRequest{ServiceName: "mesher"}, read("/services/:service/orgs", "/services/mesher/orgs"))
	assert.Nil(t, read("/orgs", "/orgs"))
}

func TestFromListOrgUsageResponseEntityToHTTPResponse(t *testing.T) {
	user := int64(10)
	got := FromListOrgUsageResponseEntityToHTTPResponse(&entity.ListOrgUsageResponse{
		Service: entity.Service{Name: "mesher"},
		Orgs: []entity.OrgUsage{
			{OrgLimit: entity.OrgLimit{OrgID: "o1", MonthlyLimit: 100, UserLimit: &user}, Custom: true, Used: 7},
			{OrgLimit: entity.OrgLimit{OrgID: "o2"}, Used: 0},
		},
	})
	assert.JSONEq(t, `{"service":"mesher","orgs":[
		{"org":"o1","orgLimit":100,"userLimit":10,"custom":true,"used":7},
		{"org":"o2","orgLimit":0,"userLimit":null,"custom":false,"used":0}]}`, string(got))

	got = FromListOrgUsageResponseEntityToHTTPResponse(&entity.ListOrgUsageResponse{Service: entity.Service{Name: "mesher"}})
	assert.JSONEq(t, `{"service":"mesher","orgs":[]}`, string(got))
}

func TestFromHTTPRequestToListUserUsageRequestEntity(t *testing.T) {
	read := func(route, target string) *entity.ListUserUsageRequest {
		var got *entity.ListUserUsageRequest
		run(t, route, httptest.NewRequest(http.MethodGet, target, nil), func(c fiber.Ctx) {
			got = FromHTTPRequestToListUserUsageRequestEntity(c)
		})
		return got
	}
	route := "/services/:service/users"
	assert.Equal(t, &entity.ListUserUsageRequest{ServiceName: "mesher", OrgID: "o1"}, read(route, "/services/mesher/users?org=o1"))
	assert.Equal(t, &entity.ListUserUsageRequest{ServiceName: "mesher"}, read(route, "/services/mesher/users"))
	assert.Nil(t, read("/users", "/users"))
}

func TestFromListUserUsageResponseEntityToHTTPResponse(t *testing.T) {
	got := FromListUserUsageResponseEntityToHTTPResponse(&entity.ListUserUsageResponse{
		Service:   entity.Service{Name: "mesher"},
		Users:     []entity.UserUsage{{OrgID: "o1", UserID: "u1", MonthlyAllowance: 10, MonthlyUsed: 3, Grants: 5, Balance: 12}},
		Truncated: true,
	})
	assert.JSONEq(t, `{"service":"mesher","truncated":true,"users":[
		{"org":"o1","user":"u1","monthlyAllowance":10,"monthlyUsed":3,"grants":5,"balance":12}]}`, string(got))

	got = FromListUserUsageResponseEntityToHTTPResponse(&entity.ListUserUsageResponse{Service: entity.Service{Name: "mesher"}})
	assert.JSONEq(t, `{"service":"mesher","truncated":false,"users":[]}`, string(got))
}
