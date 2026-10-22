// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package api is the HTTP (Fiber) transport for the REST API.
package api

import (
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	zlog "github.com/rs/zerolog/log"

	adminctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/admin"
	authctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/auth"
	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	mapper "github.com/topupsh/topupsh-tracker/backend/internal/mapper/api"
)

// RoutePrefix is where the API is mounted.
const RoutePrefix = "/api/v1"

// AdminPrefix is where the root-only admin API is mounted, apart from the
// public API.
const AdminPrefix = "/admin/v1"

// AuthPrefix is where signing in and out is mounted, outside the API:
// /auth/config, /auth/<provider>/login, the provider's
// /auth/<provider>/callback and /auth/logout.
const AuthPrefix = "/auth"

// Root sign-in attempts allowed per client IP per window, to slow down
// guessing the root access token.
const (
	_rootLoginMax    = 10
	_rootLoginWindow = time.Minute
)

// Params are the API's dependencies.
type Params struct {
	Router fiber.Router
	// Hosts checks the Host header of every request. It may be nil.
	Hosts fiber.Handler
	Auth  authctrl.Controller
	Setup setupctrl.Controller
	Admin adminctrl.Controller
	// AllowedOrigins returns the other web origins that may call the API
	// from a browser. It may be nil.
	AllowedOrigins func() []string
	// BasePath is the path prefix the app is served under ("" or "/x").
	BasePath string
	// CookieSecure marks cookies Secure; enable it behind HTTPS.
	CookieSecure bool
}

// New registers every API route on p.Router.
func New(p Params) {
	basePath := strings.TrimRight(p.BasePath, "/")
	auth := &Auth{
		controller:   p.Auth,
		basePath:     basePath,
		cookieSecure: p.CookieSecure,
	}
	user := &User{controller: p.Auth}
	admin := &Admin{controller: p.Admin}
	setup := &Setup{controller: p.Setup, basePath: basePath, cookieSecure: p.CookieSecure}

	if p.Hosts != nil {
		p.Router.Use(p.Hosts)
	}
	p.Router.Get(FaviconPath, Favicon)

	p.Router.Get(AuthPrefix+"/config", auth.Config)
	p.Router.Get(AuthPrefix+"/:provider/login", auth.Login)
	p.Router.Get(AuthPrefix+"/:provider/callback", auth.Callback)
	p.Router.Post(AuthPrefix+"/logout", auth.Logout)

	r := p.Router.Group(RoutePrefix)
	r.Use(allowedOriginsCORS(p.AllowedOrigins))

	authn := Authenticate(p.Auth)
	r.Get("/users/me", authn, user.Get)

	r.Post("/setup/login", limiter.New(limiter.Config{
		Max:          _rootLoginMax,
		Expiration:   _rootLoginWindow,
		LimitReached: func(c fiber.Ctx) error { return sendError(c, fiber.StatusTooManyRequests, "too many attempts") },
	}), setup.Login)
	r.Post("/setup/logout", setup.Logout)
	root := AuthenticateRoot(p.Setup)
	r.Get("/setup/settings", root, setup.Get)
	r.Put("/setup/settings", root, setup.Update)

	a := p.Router.Group(AdminPrefix, root)
	a.Get("/stats", admin.GetStats)
	a.Get("/users", admin.ListUsers)
	a.Get("/users/:id", admin.GetUser)
}

// allowedOriginsCORS lets the allowed origins call the API from a browser,
// cookies included. Requests from the app's own origin need no CORS.
func allowedOriginsCORS(allowed func() []string) fiber.Handler {
	return cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return allowed != nil && slices.ContainsFunc(allowed(), func(o string) bool { return strings.EqualFold(origin, o) })
		},
		AllowCredentials: true,
		AllowHeaders:     []string{fiber.HeaderContentType, fiber.HeaderAuthorization},
	})
}

func sendJSON(c fiber.Ctx, status int, payload []byte) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Status(status).Send(payload)
}

func sendError(c fiber.Ctx, status int, msg string) error {
	return sendJSON(c, status, mapper.FromErrorToHTTPResponse(msg))
}

// logServerError logs the cause of a 500, which the response hides.
func logServerError(c fiber.Ctx, err error) {
	zlog.Error().Err(err).Str("method", c.Method()).Str("host", c.Hostname()).Str("path", c.Path()).Msg("server error")
}
