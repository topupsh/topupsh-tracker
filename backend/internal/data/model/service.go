// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import "time"

type (
	// Service is a product that charges credits, e.g. mesher or tmphost.
	Service struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time
		UpdatedAt time.Time
		Name      string `gorm:"type:varchar(64);uniqueIndex;not null"`
		// Enabled switches credit limiting on for the service. It is off
		// by default: nothing is charged or recorded while it is off.
		Enabled bool `gorm:"not null;default:false"`
	}

	// ServiceAPIKey is a key a service calls the API with. Only the key's
	// SHA-256 hash is stored; the key itself is shown once, when it is made.
	ServiceAPIKey struct {
		ID        int64 `gorm:"primaryKey;autoIncrement:false"`
		CreatedAt time.Time
		ServiceID int64 `gorm:"index;not null"`
		// Prefix is the start of the key, so admins can tell keys apart.
		Prefix string `gorm:"type:varchar(16);not null"`
		// KeyHash is the hex SHA-256 of the key.
		KeyHash string `gorm:"type:char(64);uniqueIndex;not null"`
		// RevokedAt is set once the key is revoked; nil means it is active.
		RevokedAt *time.Time
	}
)
