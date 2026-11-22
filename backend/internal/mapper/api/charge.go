// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/view"
)

// FromHTTPRequestToCreateChargeRequestEntity reads POST /api/v1/charges for
// the authenticated service; the controller checks the fields.
func FromHTTPRequestToCreateChargeRequestEntity(c fiber.Ctx) *entity.CreateChargeRequest {
	var rq view.CreateChargeRequest
	serviceID, ok := c.Locals(LocalServiceID).(int64)
	if err := json.Unmarshal(c.Body(), &rq); err != nil || !ok {
		return nil
	}
	return &entity.CreateChargeRequest{
		ServiceID: serviceID,
		RequestID: rq.RequestID,
		OrgID:     rq.Org,
		UserID:    rq.User,
		Credits:   rq.Credits,
		Operation: rq.Operation,
	}
}

// FromCreateChargeResponseEntityToHTTPResponse renders a charge's outcome;
// balances are left out while credit limiting is off.
func FromCreateChargeResponseEntityToHTTPResponse(rs *entity.CreateChargeResponse) []byte {
	v := view.Charge{RequestID: rs.Charge.RequestID, Charged: rs.Charge.Refusal == "", Limited: rs.Limited}
	if rs.Limited {
		v.Balance, v.OrgRemaining = &rs.Charge.Balance, &rs.Charge.OrgRemaining
	}
	if !v.Charged {
		v.Reason = string(rs.Charge.Refusal)
		v.RenewsAt = rs.RenewsAt.UTC().Format(time.RFC3339)
	}
	return marshal(v)
}

// FromHTTPRequestToCreateRefundRequestEntity reads POST /api/v1/refunds
// for the authenticated service.
func FromHTTPRequestToCreateRefundRequestEntity(c fiber.Ctx) *entity.CreateRefundRequest {
	var rq view.CreateRefundRequest
	serviceID, ok := c.Locals(LocalServiceID).(int64)
	if err := json.Unmarshal(c.Body(), &rq); err != nil || !ok {
		return nil
	}
	return &entity.CreateRefundRequest{ServiceID: serviceID, RequestID: rq.RequestID, ChargeRequestID: rq.ChargeRequestID}
}

// FromCreateRefundResponseEntityToHTTPResponse renders a refund.
func FromCreateRefundResponseEntityToHTTPResponse(rs *entity.CreateRefundResponse) []byte {
	v := view.Refund{RequestID: rs.Refund.RequestID, ChargeRequestID: rs.Refund.ChargeRequestID, Limited: rs.Limited, Credits: rs.Refund.Credits}
	if rs.Limited {
		v.Balance = &rs.Refund.Balance
		v.RefundedAt = rs.Refund.CreatedAt.UTC().Format(time.RFC3339)
	}
	return marshal(v)
}
