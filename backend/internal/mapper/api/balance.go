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

const (
	// LocalRoot is the fiber.Ctx local the reader auth middleware sets to
	// true for the root admin.
	LocalRoot = "root"
	// LocalViewer is the fiber.Ctx local the reader auth middleware stores
	// the entity.Viewer of a view token under.
	LocalViewer = "viewer"
)

// FromHTTPRequestToGetBalanceRequestEntity reads GET /api/v1/balances for
// the authenticated reader; the controller checks the fields.
func FromHTTPRequestToGetBalanceRequestEntity(c fiber.Ctx) *entity.GetBalanceRequest {
	rd, ok := localReader(c)
	if !ok {
		return nil
	}
	return &entity.GetBalanceRequest{Reader: rd, ServiceName: c.Query("service"), OrgID: c.Query("org"), UserID: c.Query("user")}
}

// FromGetBalanceResponseEntityToHTTPResponse renders a balance.
func FromGetBalanceResponseEntityToHTTPResponse(rs *entity.GetBalanceResponse) []byte {
	b := rs.Balance
	grants := make([]view.BalanceGrant, len(b.Grants))
	for i, g := range b.Grants {
		grants[i] = view.BalanceGrant{
			ID:        FromIDToBase62(g.ID),
			Credits:   g.Amount,
			Remaining: g.Remaining,
			CreatedAt: g.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: g.ExpiresAt.UTC().Format(time.RFC3339),
		}
	}
	return marshal(view.Balance{
		Service:   b.Service.Name,
		Org:       b.OrgID,
		User:      b.UserID,
		Limited:   b.Service.Enabled,
		Balance:   b.Credits,
		Available: b.Available,
		Monthly:   view.MonthlyCredits{Limit: b.Monthly.Limit, Used: b.Monthly.Used, Remaining: b.Monthly.Remaining},
		OrgUsage:  view.OrgUsage{Limit: b.Org.Limit, UserLimit: b.Org.UserLimit, Used: b.Org.Used, Remaining: b.Org.Remaining},
		Grants:    grants,
		RenewsAt:  b.RenewsAt.UTC().Format(time.RFC3339),
	})
}

// FromHTTPRequestToListLedgerEntriesRequestEntity reads GET /api/v1/ledger
// for the authenticated reader. It returns nil for a malformed cursor or
// limit.
func FromHTTPRequestToListLedgerEntriesRequestEntity(c fiber.Ctx) *entity.ListLedgerEntriesRequest {
	rd, ok := localReader(c)
	if !ok {
		return nil
	}
	cursor, limit, ok := pageQuery(c, "cursor")
	if !ok {
		return nil
	}
	return &entity.ListLedgerEntriesRequest{
		Reader:      rd,
		ServiceName: c.Query("service"),
		OrgID:       c.Query("org"),
		UserID:      c.Query("user"),
		Cursor:      cursor,
		Limit:       limit,
	}
}

// FromListLedgerEntriesResponseEntityToHTTPResponse renders a ledger page.
func FromListLedgerEntriesResponseEntityToHTTPResponse(rs *entity.ListLedgerEntriesResponse) []byte {
	v := view.LedgerPage{Entries: make([]view.LedgerEntry, len(rs.Entries))}
	for i, e := range rs.Entries {
		v.Entries[i] = view.LedgerEntry{
			ID:           FromIDToBase62(e.ID),
			Service:      e.ServiceName,
			Org:          e.OrgID,
			User:         e.UserID,
			Kind:         string(e.Kind),
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			Actor:        e.Actor,
			Operation:    e.Operation,
			CreatedAt:    e.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	if rs.NextCursor != 0 {
		v.NextCursor = FromIDToBase62(rs.NextCursor)
	}
	return marshal(v)
}

// FromHTTPRequestToCreateViewTokenRequestEntity reads POST
// /api/v1/view-tokens for the authenticated service.
func FromHTTPRequestToCreateViewTokenRequestEntity(c fiber.Ctx) *entity.CreateViewTokenRequest {
	var rq view.CreateViewTokenRequest
	serviceID, ok := c.Locals(LocalServiceID).(int64)
	if err := json.Unmarshal(c.Body(), &rq); err != nil || !ok {
		return nil
	}
	return &entity.CreateViewTokenRequest{ServiceID: serviceID, OrgID: rq.Org, UserID: rq.User}
}

// FromCreateViewTokenResponseEntityToHTTPResponse renders a view token.
func FromCreateViewTokenResponseEntityToHTTPResponse(rs *entity.CreateViewTokenResponse) []byte {
	return marshal(view.ViewToken{Token: rs.Token, ExpiresAt: rs.ExpiresAt.UTC().Format(time.RFC3339)})
}

// FromHTTPRequestToAuthenticateViewerRequestEntity reads the view token in
// "Authorization: Bearer <token>".
func FromHTTPRequestToAuthenticateViewerRequestEntity(c fiber.Ctx) *entity.AuthenticateViewerRequest {
	rq := FromHTTPRequestToAuthenticateServiceRequestEntity(c)
	if rq == nil {
		return nil
	}
	return &entity.AuthenticateViewerRequest{Token: rq.Key}
}

// localReader is who the reader auth middleware authenticated; ok is false
// when it authenticated no one.
func localReader(c fiber.Ctx) (entity.Reader, bool) {
	if id, ok := c.Locals(LocalServiceID).(int64); ok {
		return entity.Reader{ServiceID: id}, true
	}
	if v, ok := c.Locals(LocalViewer).(entity.Viewer); ok {
		return entity.Reader{Viewer: &v}, true
	}
	if root, _ := c.Locals(LocalRoot).(bool); root {
		return entity.Reader{Root: true}, true
	}
	return entity.Reader{}, false
}
