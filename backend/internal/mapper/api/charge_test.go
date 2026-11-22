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

// asService runs fn with the service ID the service auth middleware stores,
// unless serviceID is nil.
func asService(serviceID any, fn func(c fiber.Ctx)) func(c fiber.Ctx) {
	return func(c fiber.Ctx) {
		if serviceID != nil {
			c.Locals(LocalServiceID, serviceID)
		}
		fn(c)
	}
}

func TestFromHTTPRequestToCreateChargeRequestEntity(t *testing.T) {
	read := func(serviceID any, body string) *entity.CreateChargeRequest {
		var got *entity.CreateChargeRequest
		run(t, "/charges", httptest.NewRequest(http.MethodGet, "/charges", strings.NewReader(body)), asService(serviceID, func(c fiber.Ctx) {
			got = FromHTTPRequestToCreateChargeRequestEntity(c)
		}))
		return got
	}
	body := `{"requestId":"r1","org":"o1","user":"u1","credits":2,"operation":"tool"}`
	assert.Equal(t, &entity.CreateChargeRequest{ServiceID: 7, RequestID: "r1", OrgID: "o1", UserID: "u1", Credits: 2, Operation: "tool"}, read(int64(7), body))
	assert.Equal(t, &entity.CreateChargeRequest{ServiceID: 7}, read(int64(7), `{}`), "the controller checks the fields")
	assert.Nil(t, read(int64(7), `nope`))
	assert.Nil(t, read(nil, body), "no authenticated service")
}

func TestFromCreateChargeResponseEntityToHTTPResponse(t *testing.T) {
	renews := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	charged := FromCreateChargeResponseEntityToHTTPResponse(&entity.CreateChargeResponse{
		Limited: true, RenewsAt: renews, Charge: entity.Charge{RequestID: "r1", Balance: 8, OrgRemaining: 90},
	})
	assert.JSONEq(t, `{"requestId":"r1","charged":true,"limited":true,"balance":8,"orgRemaining":90}`, string(charged))

	refused := FromCreateChargeResponseEntityToHTTPResponse(&entity.CreateChargeResponse{
		Limited: true, RenewsAt: renews, Charge: entity.Charge{RequestID: "r1", Refusal: entity.RefusalOrgLimit, Balance: 0, OrgRemaining: 0},
	})
	assert.JSONEq(t, `{"requestId":"r1","charged":false,"limited":true,"reason":"org_limit","renewsAt":"2026-12-01T00:00:00Z","balance":0,"orgRemaining":0}`, string(refused))

	off := FromCreateChargeResponseEntityToHTTPResponse(&entity.CreateChargeResponse{Charge: entity.Charge{RequestID: "r1"}})
	assert.JSONEq(t, `{"requestId":"r1","charged":true,"limited":false}`, string(off))
}

func TestFromHTTPRequestToCreateRefundRequestEntity(t *testing.T) {
	read := func(serviceID any, body string) *entity.CreateRefundRequest {
		var got *entity.CreateRefundRequest
		run(t, "/refunds", httptest.NewRequest(http.MethodGet, "/refunds", strings.NewReader(body)), asService(serviceID, func(c fiber.Ctx) {
			got = FromHTTPRequestToCreateRefundRequestEntity(c)
		}))
		return got
	}
	assert.Equal(t, &entity.CreateRefundRequest{ServiceID: 7, RequestID: "f1", ChargeRequestID: "r1"}, read(int64(7), `{"requestId":"f1","chargeRequestId":"r1"}`))
	assert.Nil(t, read(int64(7), `nope`))
	assert.Nil(t, read("7", `{"requestId":"r1"}`), "the service ID must be an int64")
}

func TestFromCreateRefundResponseEntityToHTTPResponse(t *testing.T) {
	at := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	got := FromCreateRefundResponseEntityToHTTPResponse(&entity.CreateRefundResponse{
		Limited: true, Refund: entity.Refund{RequestID: "f1", ChargeRequestID: "r1", Credits: 2, Balance: 9, CreatedAt: at},
	})
	assert.JSONEq(t, `{"requestId":"f1","chargeRequestId":"r1","limited":true,"credits":2,"refundedAt":"2026-11-21T12:00:00Z","balance":9}`, string(got))

	off := FromCreateRefundResponseEntityToHTTPResponse(&entity.CreateRefundResponse{Refund: entity.Refund{RequestID: "f1", ChargeRequestID: "r1"}})
	assert.JSONEq(t, `{"requestId":"f1","chargeRequestId":"r1","limited":false,"credits":0}`, string(off))
}
