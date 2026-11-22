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

// asReader runs fn with the locals the reader auth middleware stores.
func asReader(locals map[string]any, fn func(c fiber.Ctx)) func(c fiber.Ctx) {
	return func(c fiber.Ctx) {
		for k, v := range locals {
			c.Locals(k, v)
		}
		fn(c)
	}
}

func TestFromHTTPRequestToGetBalanceRequestEntity(t *testing.T) {
	viewer := entity.Viewer{ServiceID: 7, OrgID: "o1", UserID: "u1"}
	read := func(locals map[string]any) *entity.GetBalanceRequest {
		var got *entity.GetBalanceRequest
		run(t, "/balances", httptest.NewRequest(http.MethodGet, "/balances?service=s&org=o&user=u", nil), asReader(locals, func(c fiber.Ctx) {
			got = FromHTTPRequestToGetBalanceRequestEntity(c)
		}))
		return got
	}
	assert.Equal(t, &entity.GetBalanceRequest{Reader: entity.Reader{ServiceID: 7}, ServiceName: "s", OrgID: "o", UserID: "u"},
		read(map[string]any{LocalServiceID: int64(7)}))
	assert.Equal(t, &entity.GetBalanceRequest{Reader: entity.Reader{Viewer: &viewer}, ServiceName: "s", OrgID: "o", UserID: "u"},
		read(map[string]any{LocalViewer: viewer}), "the controller rejects a viewer's filters")
	assert.Equal(t, &entity.GetBalanceRequest{Reader: entity.Reader{Root: true}, ServiceName: "s", OrgID: "o", UserID: "u"},
		read(map[string]any{LocalRoot: true}))
	assert.Nil(t, read(nil), "no authenticated reader")
	assert.Nil(t, read(map[string]any{LocalRoot: false}))
}

func TestFromHTTPRequestToListLedgerEntriesRequestEntity(t *testing.T) {
	read := func(target string) *entity.ListLedgerEntriesRequest {
		var got *entity.ListLedgerEntriesRequest
		run(t, "/ledger", httptest.NewRequest(http.MethodGet, target, nil), asReader(map[string]any{LocalRoot: true}, func(c fiber.Ctx) {
			got = FromHTTPRequestToListLedgerEntriesRequestEntity(c)
		}))
		return got
	}
	assert.Equal(t, &entity.ListLedgerEntriesRequest{
		Reader: entity.Reader{Root: true}, ServiceName: "s", OrgID: "o", UserID: "u", Cursor: 62, Limit: 5,
	}, read("/ledger?service=s&org=o&user=u&cursor=10&limit=5"))
	assert.Equal(t, &entity.ListLedgerEntriesRequest{Reader: entity.Reader{Root: true}}, read("/ledger"))
	assert.Nil(t, read("/ledger?limit=0"))
	assert.Nil(t, read("/ledger?cursor=0"))

	var got *entity.ListLedgerEntriesRequest
	run(t, "/ledger", httptest.NewRequest(http.MethodGet, "/ledger", nil), func(c fiber.Ctx) {
		got = FromHTTPRequestToListLedgerEntriesRequestEntity(c)
	})
	assert.Nil(t, got, "no authenticated reader")
}

func TestFromListLedgerEntriesResponseEntityToHTTPResponse(t *testing.T) {
	got := FromListLedgerEntriesResponseEntityToHTTPResponse(&entity.ListLedgerEntriesResponse{Entries: []entity.LedgerEntry{{
		ID: 1, ServiceName: "mesher", OrgID: "o1", UserID: "u1", Kind: entity.LedgerKindGrant, Amount: 5, BalanceAfter: 5,
		Actor: "root", CreatedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.FixedZone("x", 3600)),
	}}})
	assert.JSONEq(t, `{"entries":[{"id":"00000000001","service":"mesher","org":"o1","user":"u1","kind":"grant",
		"amount":5,"balanceAfter":5,"actor":"root","createdAt":"2026-11-21T11:00:00Z"}]}`, string(got))
}

func TestFromGetBalanceResponseEntityToHTTPResponse_Uncapped(t *testing.T) {
	got := FromGetBalanceResponseEntityToHTTPResponse(&entity.GetBalanceResponse{Balance: entity.Balance{
		Service: entity.Service{Name: "mesher"}, OrgID: "o1", UserID: "u1",
		RenewsAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}})
	assert.JSONEq(t, `{"service":"mesher","org":"o1","user":"u1","limited":false,"balance":0,"available":0,
		"monthly":{"limit":0,"used":0,"remaining":0},"orgUsage":{"limit":0,"userLimit":null,"used":0,"remaining":0},
		"grants":[],"renewsAt":"2026-12-01T00:00:00Z"}`, string(got))
}

func TestFromHTTPRequestToCreateViewTokenRequestEntity(t *testing.T) {
	read := func(serviceID any, body string) *entity.CreateViewTokenRequest {
		var got *entity.CreateViewTokenRequest
		run(t, "/view-tokens", httptest.NewRequest(http.MethodGet, "/view-tokens", strings.NewReader(body)), asService(serviceID, func(c fiber.Ctx) {
			got = FromHTTPRequestToCreateViewTokenRequestEntity(c)
		}))
		return got
	}
	assert.Equal(t, &entity.CreateViewTokenRequest{ServiceID: 7, OrgID: "o1", UserID: "u1"}, read(int64(7), `{"org":"o1","user":"u1"}`))
	assert.Nil(t, read(int64(7), `nope`))
	assert.Nil(t, read(nil, `{"org":"o1","user":"u1"}`), "no authenticated service")
}

func TestFromHTTPRequestToAuthenticateViewerRequestEntity(t *testing.T) {
	read := func(auth string) *entity.AuthenticateViewerRequest {
		var got *entity.AuthenticateViewerRequest
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if auth != "" {
			req.Header.Set(fiber.HeaderAuthorization, auth)
		}
		run(t, "/x", req, func(c fiber.Ctx) { got = FromHTTPRequestToAuthenticateViewerRequestEntity(c) })
		return got
	}
	assert.Equal(t, &entity.AuthenticateViewerRequest{Token: "tok"}, read("Bearer tok"))
	assert.Nil(t, read(""))
	assert.Nil(t, read("Basic tok"))
}

func TestFromGetBalanceResponseEntityToHTTPResponse(t *testing.T) {
	ten := int64(10)
	got := FromGetBalanceResponseEntityToHTTPResponse(&entity.GetBalanceResponse{Balance: entity.Balance{
		Service: entity.Service{Name: "mesher", Enabled: true}, OrgID: "o1", UserID: "u1", Credits: 13, Available: 11,
		Monthly: entity.MonthlyCredits{Limit: 10, Used: 3, Remaining: 7},
		Org:     entity.OrgCredits{Limit: 100, UserLimit: &ten, Used: 95, Remaining: 5},
		Grants: []entity.CreditBucket{{
			ID: 62, Amount: 7, Remaining: 6,
			CreatedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC),
		}},
		RenewsAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}})
	assert.JSONEq(t, `{"service":"mesher","org":"o1","user":"u1","limited":true,"balance":13,"available":11,
		"monthly":{"limit":10,"used":3,"remaining":7},"orgUsage":{"limit":100,"userLimit":10,"used":95,"remaining":5},
		"grants":[{"id":"00000000010","credits":7,"remaining":6,"createdAt":"2026-11-21T12:00:00Z","expiresAt":"2026-12-22T00:00:00Z"}],
		"renewsAt":"2026-12-01T00:00:00Z"}`, string(got))
}

func TestFromListLedgerEntriesResponseEntityToHTTPResponse_NextCursor(t *testing.T) {
	got := FromListLedgerEntriesResponseEntityToHTTPResponse(&entity.ListLedgerEntriesResponse{Entries: []entity.LedgerEntry{}, NextCursor: 62})
	assert.JSONEq(t, `{"entries":[],"nextCursor":"00000000010"}`, string(got))
}

func TestFromCreateViewTokenResponseEntityToHTTPResponse(t *testing.T) {
	got := FromCreateViewTokenResponseEntityToHTTPResponse(&entity.CreateViewTokenResponse{
		Token: "tok", ExpiresAt: time.Date(2026, 11, 21, 14, 0, 0, 0, time.FixedZone("x", 3600)),
	})
	assert.JSONEq(t, `{"token":"tok","expiresAt":"2026-11-21T13:00:00Z"}`, string(got))
}
