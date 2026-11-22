// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	authctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/auth"
	balancectrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/balance"
	servicekeyctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/servicekey"
	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// Authenticate rejects requests without a valid session cookie and stores
// the user ID for the handlers behind it.
func Authenticate(controller authctrl.Controller) fiber.Handler {
	return func(c fiber.Ctx) error {
		rq := mapper.FromHTTPRequestToAuthenticateRequestEntity(c)
		if rq == nil {
			return sendError(c, fiber.StatusUnauthorized, "unauthorized")
		}
		rs, err := controller.Authenticate(c.Context(), rq)
		if err != nil {
			return sendError(c, fiber.StatusUnauthorized, "unauthorized")
		}
		c.Locals(mapper.LocalUserID, rs.UserID)
		return c.Next()
	}
}

// AuthenticateService rejects requests without an active service API key
// in "Authorization: Bearer <key>" and stores the calling service's ID for
// the handlers behind it, so they only reach that service's credits.
func AuthenticateService(controller servicekeyctrl.Controller) fiber.Handler {
	return func(c fiber.Ctx) error {
		rq := mapper.FromHTTPRequestToAuthenticateServiceRequestEntity(c)
		if rq == nil {
			return sendError(c, fiber.StatusUnauthorized, "unauthorized")
		}
		rs, err := controller.AuthenticateService(c.Context(), rq)
		if errors.Is(err, servicekeyctrl.ErrInvalidKey) {
			return sendError(c, fiber.StatusUnauthorized, "unauthorized")
		}
		if err != nil {
			logServerError(c, err)
			return sendError(c, fiber.StatusInternalServerError, "failed to authenticate")
		}
		c.Locals(mapper.LocalServiceID, rs.Service.ID)
		return c.Next()
	}
}

// AuthenticateReader lets three kinds of readers through and stores which
// one called: a service with its API key in "Authorization: Bearer tpk_..."
// (as AuthenticateService), a user with a view token their service issued
// in "Authorization: Bearer <token>", or, without an Authorization header,
// the root admin with the root session cookie.
func AuthenticateReader(keys servicekeyctrl.Controller, setup setupctrl.Controller, balances balancectrl.Controller) fiber.Handler {
	service, root := AuthenticateService(keys), AuthenticateRoot(setup)
	return func(c fiber.Ctx) error {
		rq := mapper.FromHTTPRequestToAuthenticateViewerRequestEntity(c)
		switch {
		case rq == nil:
			c.Locals(mapper.LocalRoot, true)
			return root(c)
		case strings.HasPrefix(rq.Token, servicekeyctrl.KeyPrefix):
			return service(c)
		}
		rs, err := balances.AuthenticateViewer(c.Context(), rq)
		if err != nil {
			return sendError(c, fiber.StatusUnauthorized, "unauthorized")
		}
		c.Locals(mapper.LocalViewer, rs.Viewer)
		return c.Next()
	}
}
