// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

type (
	// CreateServiceAPIKeyRequest asks for a new API key for a service,
	// creating the service when it is new.
	CreateServiceAPIKeyRequest struct {
		ServiceName string
	}

	// CreateServiceAPIKeyResponse carries the new key. Key is the only
	// time the key itself is shown.
	CreateServiceAPIKeyResponse struct {
		Service Service
		APIKey  ServiceAPIKey
		Key     string
	}

	// RevokeServiceAPIKeyRequest asks to revoke one of a service's keys.
	RevokeServiceAPIKeyRequest struct {
		ServiceName string
		KeyID       int64
	}

	// RevokeServiceAPIKeyResponse confirms a revocation.
	RevokeServiceAPIKeyResponse struct{}

	// AuthenticateServiceRequest carries the API key a service called with.
	AuthenticateServiceRequest struct {
		Key string
	}

	// AuthenticateServiceResponse is the service the key belongs to.
	AuthenticateServiceResponse struct {
		Service Service
	}
)
