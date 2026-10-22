// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package brand holds the topupsh-tracker icon, built into the binary so the
// server can show it without the frontend bundle.
package brand

import (
	_ "embed"
	"encoding/base64"
)

// The icon is favicon-light.svg at the repository root; favicon.ico (16, 32
// and 48 px) and icon.png (128 px) are rendered from it.
var (
	//go:embed favicon.ico
	Favicon []byte
	//go:embed icon.svg
	IconSVG []byte
	//go:embed icon.png
	IconPNG []byte
)

// Media types of the icons.
const (
	FaviconType = "image/x-icon"
	IconSVGType = "image/svg+xml"
	IconPNGType = "image/png"
)

// DataURI returns data as a base64 data URI of the given media type.
func DataURI(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}
