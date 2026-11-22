// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	servicekeyctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// ServiceKey handles the root-only service API key routes.
type ServiceKey struct {
	controller servicekeyctrl.Controller
}

// Create issues a new API key for a service and shows it, the only time
// it is shown.
func (h *ServiceKey) Create(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToCreateServiceAPIKeyRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.CreateServiceAPIKey(c.Context(), rq)
	if errors.Is(err, servicekeyctrl.ErrInvalidServiceName) {
		return sendError(c, fiber.StatusBadRequest, err.Error())
	}
	if err != nil {
		logServerError(c, err)
		return sendError(c, fiber.StatusInternalServerError, "failed to create api key")
	}
	return sendJSON(c, fiber.StatusCreated, mapper.FromCreateServiceAPIKeyResponseEntityToHTTPResponse(rs))
}

// Delete revokes one of a service's API keys.
func (h *ServiceKey) Delete(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToRevokeServiceAPIKeyRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	_, err := h.controller.RevokeServiceAPIKey(c.Context(), rq)
	if errors.Is(err, servicekeyctrl.ErrServiceNotFound) || errors.Is(err, servicekeyctrl.ErrKeyNotFound) {
		return sendError(c, fiber.StatusNotFound, err.Error())
	}
	if err != nil {
		logServerError(c, err)
		return sendError(c, fiber.StatusInternalServerError, "failed to revoke api key")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
