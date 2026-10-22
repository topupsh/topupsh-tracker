// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/topupsh/topupsh-tracker/backend/internal/brand"
)

// FaviconPath is where browsers look for a site's icon.
const FaviconPath = "/favicon.ico"

// Favicon sends the app icon.
func Favicon(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, brand.FaviconType)
	c.Set(fiber.HeaderCacheControl, "public, max-age=86400")
	return c.Send(brand.Favicon)
}
