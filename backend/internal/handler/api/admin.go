// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	adminctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/admin"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// Admin handles the root-only admin page.
type Admin struct {
	controller adminctrl.Controller
}

// GetStats returns the totals and daily stats over every account.
func (h *Admin) GetStats(c fiber.Ctx) error {
	rs, err := h.controller.GetStats(c.Context(), mapper.FromHTTPRequestToGetAdminStatsRequestEntity(c))
	if err != nil {
		logServerError(c, err)
		return sendError(c, fiber.StatusInternalServerError, "failed to get stats")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromGetAdminStatsResponseEntityToHTTPResponse(rs))
}

// ListUsers returns a page of users, newest first, optionally searched.
func (h *Admin) ListUsers(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToListAdminUsersRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.ListUsers(c.Context(), rq)
	if err != nil {
		logServerError(c, err)
		return sendError(c, fiber.StatusInternalServerError, "failed to list users")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromListAdminUsersResponseEntityToHTTPResponse(rs))
}

// GetUser returns one user.
func (h *Admin) GetUser(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToGetAdminUserRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.GetUser(c.Context(), rq)
	if err != nil {
		return h.userError(c, err, "failed to get user")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromGetAdminUserResponseEntityToHTTPResponse(rs))
}

// userError answers a failed lookup of one user.
func (h *Admin) userError(c fiber.Ctx, err error, msg string) error {
	if errors.Is(err, adminctrl.ErrUserNotFound) {
		return sendError(c, fiber.StatusNotFound, "user not found")
	}
	logServerError(c, err)
	return sendError(c, fiber.StatusInternalServerError, msg)
}
