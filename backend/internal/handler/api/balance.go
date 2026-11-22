// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	balancectrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/balance"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// Balance handles the routes that read balances and the ledger, and the
// one services issue view tokens with.
type Balance struct {
	controller balancectrl.Controller
}

// Get returns one user's balance.
func (h *Balance) Get(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToGetBalanceRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.GetBalance(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to read balance")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromGetBalanceResponseEntityToHTTPResponse(rs))
}

// ListLedgerEntries returns a page of the ledger, newest first.
func (h *Balance) ListLedgerEntries(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToListLedgerEntriesRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.ListLedgerEntries(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to read ledger")
	}
	return sendJSON(c, fiber.StatusOK, mapper.FromListLedgerEntriesResponseEntityToHTTPResponse(rs))
}

// CreateViewToken issues a token that lets one of the calling service's
// users read their own credits.
func (h *Balance) CreateViewToken(c fiber.Ctx) error {
	rq := mapper.FromHTTPRequestToCreateViewTokenRequestEntity(c)
	if rq == nil {
		return fiber.ErrBadRequest
	}
	rs, err := h.controller.CreateViewToken(c.Context(), rq)
	if err != nil {
		return h.sendError(c, err, "failed to create view token")
	}
	return sendJSON(c, fiber.StatusCreated, mapper.FromCreateViewTokenResponseEntityToHTTPResponse(rs))
}

// sendError maps a controller error to its status; anything unexpected is
// logged and answered with msg.
func (h *Balance) sendError(c fiber.Ctx, err error, msg string) error {
	switch {
	case errors.Is(err, balancectrl.ErrForbidden):
		return sendError(c, fiber.StatusForbidden, err.Error())
	case errors.Is(err, balancectrl.ErrServiceNotFound):
		return sendError(c, fiber.StatusNotFound, err.Error())
	case errors.Is(err, balancectrl.ErrViewerFilter),
		errors.Is(err, balancectrl.ErrServiceRequired),
		errors.Is(err, balancectrl.ErrInvalidOrg),
		errors.Is(err, balancectrl.ErrInvalidUser),
		errors.Is(err, balancectrl.ErrInvalidLimit):
		return sendError(c, fiber.StatusBadRequest, err.Error())
	}
	logServerError(c, err)
	return sendError(c, fiber.StatusInternalServerError, msg)
}
