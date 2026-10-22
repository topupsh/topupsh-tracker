// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"net"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// hosts tells the app host apart from hosts the server does not answer
// for.
type hosts struct {
	app string
	// open skips the host check: the app URL names this machine or an IP,
	// as in local development.
	open bool
}

func newHosts(appURL string) hosts {
	app := strings.ToLower(setupctrl.AppHost(appURL))
	return hosts{app: app, open: isLocal(app) || net.ParseIP(app) != nil}
}

// isApp reports whether host (a Host header, port allowed) reaches the app.
func (h hosts) isApp(host string) bool {
	name := strings.ToLower(host)
	if n, _, err := net.SplitHostPort(name); err == nil {
		name = n
	}
	// This machine always reaches the app, e.g. for health checks.
	return h.open || name == h.app || isLocal(name)
}

func isLocal(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

// hostRouter lets app host requests through and refuses every other host.
func hostRouter(h hosts) fiber.Handler {
	return func(c fiber.Ctx) error {
		if h.isApp(string(c.Request().Host())) {
			return c.Next()
		}
		return sendJSONError(c, fiber.StatusNotFound, "host not allowed")
	}
}

func sendJSONError(c fiber.Ctx, status int, msg string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Status(status).Send(mapper.FromErrorToHTTPResponse(msg))
}

// httpsRedirect is the plain http server while TLS is on: it sends
// requests for the app host to https on port.
func httpsRedirect(h hosts, port int) *fiber.App {
	f := fiber.New()
	f.Use(func(c fiber.Ctx) error {
		host := string(c.Request().Host())
		if !h.isApp(host) {
			return sendJSONError(c, fiber.StatusNotFound, "host not allowed")
		}
		if n, _, err := net.SplitHostPort(host); err == nil {
			host = n
		}
		if port != 443 {
			host = net.JoinHostPort(host, strconv.Itoa(port))
		}
		return c.Redirect().Status(fiber.StatusMovedPermanently).To("https://" + host + string(c.Request().RequestURI()))
	})
	return f
}
