// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// ServiceAPIKey is a service's API key without the key itself.
	ServiceAPIKey struct {
		ID      string `json:"id"`
		Service string `json:"service"`
		// Prefix is the start of the key, to tell keys apart.
		Prefix    string `json:"prefix"`
		CreatedAt string `json:"createdAt"`
		// RevokedAt is set once the key is revoked.
		RevokedAt string `json:"revokedAt,omitempty"`
	}

	// CreateServiceAPIKeyResponse carries a new key. Key is shown only
	// here, once.
	CreateServiceAPIKeyResponse struct {
		APIKey ServiceAPIKey `json:"apiKey"`
		Key    string        `json:"key"`
	}
)
