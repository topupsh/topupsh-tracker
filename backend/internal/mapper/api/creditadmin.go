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
	return marshal(FromServiceEntityToServiceView(rs.Service))
}

// FromServiceEntityToServiceView maps a service to its JSON form.
func FromServiceEntityToServiceView(e entity.Service) view.Service {
	return view.Service{Name: e.Name, Enabled: e.Enabled, DefaultOrgLimit: e.DefaultOrgLimit, DefaultUserLimit: e.DefaultUserLimit}
}

// FromHTTPRequestToSetServiceDefaultsRequestEntity reads PUT
// /services/{service}/defaults; orgLimit is required.
func FromHTTPRequestToSetServiceDefaultsRequestEntity(c fiber.Ctx) *entity.SetServiceDefaultsRequest {
	var rq view.SetServiceDefaultsRequest
	name := c.Params("service")
	if err := json.Unmarshal(c.Body(), &rq); err != nil || name == "" || rq.OrgLimit == nil {
		return nil
	}
	return &entity.SetServiceDefaultsRequest{ServiceName: name, OrgLimit: *rq.OrgLimit, UserLimit: rq.UserLimit}
}

func FromSetServiceDefaultsResponseEntityToHTTPResponse(rs *entity.SetServiceDefaultsResponse) []byte {
	return marshal(FromServiceEntityToServiceView(rs.Service))
}

func FromHTTPRequestToListServicesRequestEntity(fiber.Ctx) *entity.ListServicesRequest {
	return &entity.ListServicesRequest{}
}

func FromListServicesResponseEntityToHTTPResponse(rs *entity.ListServicesResponse) []byte {
	services := make([]view.ServiceDetail, 0, len(rs.Services))
	for _, s := range rs.Services {
		keys := make([]view.ServiceAPIKey, 0, len(s.APIKeys))
		for _, k := range s.APIKeys {
			keys = append(keys, FromServiceAPIKeyEntityToServiceAPIKeyView(s.Service.Name, k))
		}
		services = append(services, view.ServiceDetail{
			Service:   FromServiceEntityToServiceView(s.Service),
			CreatedAt: s.Service.CreatedAt.UTC().Format(time.RFC3339),
			APIKeys:   keys,
		})
	}
	return marshal(view.ListServicesResponse{Services: services})
}

func FromHTTPRequestToListOrgUsageRequestEntity(c fiber.Ctx) *entity.ListOrgUsageRequest {
	name := c.Params("service")
	if name == "" {
		return nil
	}
	return &entity.ListOrgUsageRequest{ServiceName: name}
}

func FromListOrgUsageResponseEntityToHTTPResponse(rs *entity.ListOrgUsageResponse) []byte {
	orgs := make([]view.AdminOrgUsage, 0, len(rs.Orgs))
	for _, o := range rs.Orgs {
		orgs = append(orgs, view.AdminOrgUsage{
			Org:       o.OrgLimit.OrgID,
			OrgLimit:  o.OrgLimit.MonthlyLimit,
			UserLimit: o.OrgLimit.UserLimit,
			Custom:    o.Custom,
			Used:      o.Used,
		})
	}
	return marshal(view.ListOrgUsageResponse{Service: rs.Service.Name, Orgs: orgs})
}

func FromHTTPRequestToListUserUsageRequestEntity(c fiber.Ctx) *entity.ListUserUsageRequest {
	name := c.Params("service")
	if name == "" {
		return nil
	}
	return &entity.ListUserUsageRequest{ServiceName: name, OrgID: c.Query("org")}
}

func FromListUserUsageResponseEntityToHTTPResponse(rs *entity.ListUserUsageResponse) []byte {
	users := make([]view.UserUsage, 0, len(rs.Users))
	for _, u := range rs.Users {
		users = append(users, view.UserUsage{
			Org:              u.OrgID,
			User:             u.UserID,
			MonthlyAllowance: u.MonthlyAllowance,
			MonthlyUsed:      u.MonthlyUsed,
			Grants:           u.Grants,
			Balance:          u.Balance,
		})
	}
	return marshal(view.ListUserUsageResponse{Service: rs.Service.Name, Users: users, Truncated: rs.Truncated})
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
	return &entity.CreateGrantRequest{RequestID: rq.RequestID, ServiceName: rq.Service, OrgID: rq.Org, UserID: rq.User, Credits: rq.Credits}
}

// FromCreateGrantResponseEntityToHTTPResponse renders the new grant.
func FromCreateGrantResponseEntityToHTTPResponse(rs *entity.CreateGrantResponse) []byte {
	return marshal(view.Grant{
		ID:           FromIDToBase62(rs.Bucket.ID),
		RequestID:    rs.LedgerEntry.RequestID,
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
