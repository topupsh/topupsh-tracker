// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	chargectrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/charge"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// Charge handles the routes services charge and refund credits with.
type Charge struct {
	controller chargectrl.Controller
}

// Create charges a user's credits: 200 when charged, 402 with the reason
// and renewal time when a limit refused it.
func (h *Charge) Create(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToCreateChargeRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.CreateCharge(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to charge")
	}
	status := fiber.StatusOK
	if rs.Charge.Refusal != "" {
		status = fiber.StatusPaymentRequired
	}
	return sendJSON(c, status, mapper.FromCreateChargeResponseEntityToHTTPResponse(rs))
}

// CreateRefund refunds a charge.
func (h *Charge) CreateRefund(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToCreateRefundRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.CreateRefund(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to refund")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromCreateRefundResponseEntityToHTTPResponse(rs))
}

// sendError maps a controller error to its status; anything unexpected is
// logged and answered with msg.
func (h *Charge) sendError(c fiber.Ctx, err error, msg string) error {
	switch {
	case errors.Is(err, chargectrl.ErrInvalidRequestID),
		errors.Is(err, chargectrl.ErrInvalidChargeRequestID),
		errors.Is(err, chargectrl.ErrInvalidOrg),
		errors.Is(err, chargectrl.ErrInvalidUser),
		errors.Is(err, chargectrl.ErrInvalidCredits),
		errors.Is(err, chargectrl.ErrInvalidOperation):
		return sendError(c, fiber.StatusBadRequest, err.Error())
	case errors.Is(err, chargectrl.ErrChargeNotFound),
		errors.Is(err, chargectrl.ErrServiceNotFound):
		return sendError(c, fiber.StatusNotFound, err.Error())
	case errors.Is(err, chargectrl.ErrRequestConflict),
		errors.Is(err, chargectrl.ErrNotCharged):
		return sendError(c, fiber.StatusConflict, err.Error())
	}
	logServerError(c, err)
	return sendError(c, fiber.StatusInternalServerError, msg)
}
