// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package api maps between HTTP requests/responses and domain entities.
package api

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/mustafaturan/monoflake"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/view"
)

const (
	// SessionCookie holds the signed session token.
	SessionCookie = "topupsh_tracker_session"
	// StateCookie pins the OAuth state to the browser that started login.
	StateCookie = "topupsh_tracker_oauth_state"
	// LocalUserID is the fiber.Ctx local the auth middleware stores the
	// authenticated user ID under.
	LocalUserID = "userID"
)

// FromHTTPRequestToStartLoginRequestEntity reads GET /auth/:provider/login.
func FromHTTPRequestToStartLoginRequestEntity(c fiber.Ctx) *entity.StartLoginRequest {
	provider := c.Params("provider")
	if provider == "" {
		return nil
	}
	return &entity.StartLoginRequest{Provider: provider, RedirectURL: c.Query("redirect_url")}
}

// FromHTTPRequestToCompleteLoginRequestEntity reads the provider callback.
// It returns nil when the provider sent no code (e.g. the user cancelled).
func FromHTTPRequestToCompleteLoginRequestEntity(c fiber.Ctx) *entity.CompleteLoginRequest {
	provider, code := c.Params("provider"), c.Query("code")
	if provider == "" || code == "" {
		return nil
	}
	return &entity.CompleteLoginRequest{
		Provider:    provider,
		Code:        code,
		State:       c.Query("state"),
		StateCookie: c.Cookies(StateCookie),
	}
}

// FromHTTPRequestToAuthenticateRequestEntity reads the session cookie.
func FromHTTPRequestToAuthenticateRequestEntity(c fiber.Ctx) *entity.AuthenticateRequest {
	tok := c.Cookies(SessionCookie)
	if tok == "" {
		return nil
	}
	return &entity.AuthenticateRequest{SessionToken: tok}
}

// FromGetAuthConfigResponseEntityToHTTPResponse renders GET /auth/config.
func FromGetAuthConfigResponseEntityToHTTPResponse(rs *entity.GetAuthConfigResponse) []byte {
	providers := make([]view.AuthProvider, len(rs.Providers))
	for i, p := range rs.Providers {
		providers[i] = view.AuthProvider{ID: p.ID, Name: p.Name, Icon: p.Icon}
	}
	return marshal(view.AuthConfig{Providers: providers})
}

// FromErrorToHTTPResponse renders an API error body.
func FromErrorToHTTPResponse(msg string) []byte {
	return marshal(view.Error{Error: msg})
}

// FromIDToBase62 renders an int64 ID the way the API exposes it.
func FromIDToBase62(id int64) string {
	return monoflake.ID(id).String()
}

// localUserID is the user ID the auth middleware stored on the request; 0
// when there is none.
func localUserID(c fiber.Ctx) int64 {
	id, ok := c.Locals(LocalUserID).(int64)
	if !ok || id <= 0 {
		return 0
	}
	return id
}

// pageQuery reads the optional before cursor (a base62 ID) and limit of a
// page; ok is false when either is malformed.
func pageQuery(c fiber.Ctx) (before int64, limit int, ok bool) {
	if s := c.Query("before"); s != "" {
		before = monoflake.IDFromBase62(s).Int64()
		if before <= 0 {
			return 0, 0, false
		}
	}
	if s := c.Query("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		limit = n
	}
	return before, limit, true
}

// marshal encodes views, which are plain structs that always encode.
func marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
