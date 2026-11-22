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

// FromHTTPRequestToSetServiceEnabledRequestEntity reads PUT
// /api/v1/services/:service/enabled; enabled is required.
func FromHTTPRequestToSetServiceEnabledRequestEntity(c fiber.Ctx) *entity.SetServiceEnabledRequest {
	var rq view.SetServiceEnabledRequest
	name := c.Params("service")
	if err := json.Unmarshal(c.Body(), &rq); err != nil || name == "" || rq.Enabled == nil {
		return nil
	}
	return &entity.SetServiceEnabledRequest{ServiceName: name, Enabled: *rq.Enabled}
}

// FromSetServiceEnabledResponseEntityToHTTPResponse renders the service.
func FromSetServiceEnabledResponseEntityToHTTPResponse(rs *entity.SetServiceEnabledResponse) []byte {
	return marshal(view.Service{Name: rs.Service.Name, Enabled: rs.Service.Enabled})
}

// FromHTTPRequestToSetOrgLimitsRequestEntity reads PUT
// /api/v1/orgs/:org/limits; service is required.
func FromHTTPRequestToSetOrgLimitsRequestEntity(c fiber.Ctx) *entity.SetOrgLimitsRequest {
	var rq view.SetOrgLimitsRequest
	org := c.Params("org")
	if err := json.Unmarshal(c.Body(), &rq); err != nil || org == "" || rq.Service == "" {
		return nil
	}
	return &entity.SetOrgLimitsRequest{
		ServiceName:  rq.Service,
		OrgID:        org,
		MonthlyLimit: rq.OrgLimit,
		UserLimit:    rq.UserLimit,
	}
}

// FromSetOrgLimitsResponseEntityToHTTPResponse renders the org's limits.
func FromSetOrgLimitsResponseEntityToHTTPResponse(rs *entity.SetOrgLimitsResponse) []byte {
	return marshal(view.OrgLimits{
		Service:   rs.Service.Name,
		Org:       rs.OrgLimit.OrgID,
		OrgLimit:  rs.OrgLimit.MonthlyLimit,
		UserLimit: rs.OrgLimit.UserLimit,
	})
}

// FromHTTPRequestToCreateGrantRequestEntity reads POST /api/v1/grants;
// service is required, the controller checks the rest.
func FromHTTPRequestToCreateGrantRequestEntity(c fiber.Ctx) *entity.CreateGrantRequest {
	var rq view.CreateGrantRequest
	if err := json.Unmarshal(c.Body(), &rq); err != nil || rq.Service == "" {
		return nil
	}
	return &entity.CreateGrantRequest{ServiceName: rq.Service, OrgID: rq.Org, UserID: rq.User, Credits: rq.Credits}
}

// FromCreateGrantResponseEntityToHTTPResponse renders the new grant.
func FromCreateGrantResponseEntityToHTTPResponse(rs *entity.CreateGrantResponse) []byte {
	return marshal(view.Grant{
		ID:           FromIDToBase62(rs.Bucket.ID),
		Service:      rs.Service.Name,
		Org:          rs.Bucket.OrgID,
		User:         rs.Bucket.UserID,
		Credits:      rs.Bucket.Amount,
		Remaining:    rs.Bucket.Remaining,
		CreatedAt:    rs.Bucket.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt:    rs.Bucket.ExpiresAt.UTC().Format(time.RFC3339),
		BalanceAfter: rs.LedgerEntry.BalanceAfter,
	})
}
