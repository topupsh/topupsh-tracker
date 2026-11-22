// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	creditadminctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/creditadmin"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// CreditAdmin handles the root-only credit settings routes.
type CreditAdmin struct {
	controller creditadminctrl.Controller
}

// UpdateServiceEnabled switches credit limiting on or off for a service.
func (h *CreditAdmin) UpdateServiceEnabled(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToSetServiceEnabledRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.SetServiceEnabled(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to update service")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromSetServiceEnabledResponseEntityToHTTPResponse(rs))
}

// UpdateOrgLimits sets a service's monthly limits for one org.
func (h *CreditAdmin) UpdateOrgLimits(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToSetOrgLimitsRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.SetOrgLimits(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to update limits")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromSetOrgLimitsResponseEntityToHTTPResponse(rs))
}

// CreateGrant gives a user extra credits.
func (h *CreditAdmin) CreateGrant(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToCreateGrantRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.CreateGrant(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to create grant")
	}
	return sendJSON(c, fiber.StatusCreated, mapper.FromCreateGrantResponseEntityToHTTPResponse(rs))
}

// sendError maps a controller error to its status; anything unexpected is
// logged and answered with msg.
// UpdateServiceDefaults sets the limits of the orgs without their own.
func (h *CreditAdmin) UpdateServiceDefaults(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToSetServiceDefaultsRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.SetServiceDefaults(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to update defaults")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromSetServiceDefaultsResponseEntityToHTTPResponse(rs))
}

// ListServices lists every service with its API keys.
func (h *CreditAdmin) ListServices(c fiber.Ctx) error {
	rs, err := h.controller.ListServices(c.Context(), mapper.FromHTTPRequestToListServicesRequestEntity(c))
	if err != nil {
		return h.sendError(c, err, "failed to list services")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromListServicesResponseEntityToHTTPResponse(rs))
}

// ListOrgUsage lists a service's orgs, their limits and usage.
func (h *CreditAdmin) ListOrgUsage(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToListOrgUsageRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.ListOrgUsage(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to list orgs")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromListOrgUsageResponseEntityToHTTPResponse(rs))
}

// ListUserUsage lists a service's users with credits and their usage.
func (h *CreditAdmin) ListUserUsage(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToListUserUsageRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.ListUserUsage(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to list users")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromListUserUsageResponseEntityToHTTPResponse(rs))
}

func (h *CreditAdmin) sendError(c fiber.Ctx, err error, msg string) error {
	switch {
	case errors.Is(err, creditadminctrl.ErrServiceNotFound):
		return sendError(c, fiber.StatusNotFound, err.Error())
	case errors.Is(err, creditadminctrl.ErrInvalidOrg),
		errors.Is(err, creditadminctrl.ErrInvalidUser),
		errors.Is(err, creditadminctrl.ErrInvalidLimit),
		errors.Is(err, creditadminctrl.ErrInvalidCredits):
		return sendError(c, fiber.StatusBadRequest, err.Error())
	}
	logServerError(c, err)
	return sendError(c, fiber.StatusInternalServerError, msg)
}
