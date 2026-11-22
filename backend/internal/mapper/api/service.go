// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mustafaturan/monoflake"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/view"
)

// LocalServiceID is the fiber.Ctx local the service auth middleware stores
// the calling service's ID under.
const LocalServiceID = "serviceID"

// FromHTTPRequestToCreateServiceAPIKeyRequestEntity reads the service of
// POST /api/v1/services/:service/keys.
func FromHTTPRequestToCreateServiceAPIKeyRequestEntity(c fiber.Ctx) *entity.CreateServiceAPIKeyRequest {
	name := c.Params("service")
	if name == "" {
		return nil
	}
	return &entity.CreateServiceAPIKeyRequest{ServiceName: name}
}

// FromCreateServiceAPIKeyResponseEntityToHTTPResponse renders a new key,
// the key itself included.
func FromCreateServiceAPIKeyResponseEntityToHTTPResponse(rs *entity.CreateServiceAPIKeyResponse) []byte {
	return marshal(view.CreateServiceAPIKeyResponse{
		APIKey: view.ServiceAPIKey{
			ID:        FromIDToBase62(rs.APIKey.ID),
			Service:   rs.Service.Name,
			Prefix:    rs.APIKey.Prefix,
			CreatedAt: rs.APIKey.CreatedAt.UTC().Format(time.RFC3339),
		},
		Key: rs.Key,
	})
}

// FromHTTPRequestToRevokeServiceAPIKeyRequestEntity reads the service and
// base62 key ID of DELETE /api/v1/services/:service/keys/:id.
func FromHTTPRequestToRevokeServiceAPIKeyRequestEntity(c fiber.Ctx) *entity.RevokeServiceAPIKeyRequest {
	name := c.Params("service")
	id := monoflake.IDFromBase62(c.Params("id")).Int64()
	if name == "" || id <= 0 {
		return nil
	}
	return &entity.RevokeServiceAPIKeyRequest{ServiceName: name, KeyID: id}
}

// FromHTTPRequestToAuthenticateServiceRequestEntity reads the key in an
// "Authorization: Bearer <key>" header.
func FromHTTPRequestToAuthenticateServiceRequestEntity(c fiber.Ctx) *entity.AuthenticateServiceRequest {
	scheme, key, ok := strings.Cut(c.Get(fiber.HeaderAuthorization), " ")
	key = strings.TrimSpace(key)
	if !ok || !strings.EqualFold(scheme, "bearer") || key == "" {
		return nil
	}
	return &entity.AuthenticateServiceRequest{Key: key}
}
