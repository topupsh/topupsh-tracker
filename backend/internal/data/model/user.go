// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package model holds the GORM database models.
package model

import "time"

// User is an account. Accounts are keyed by verified email, so signing in
// with Google and GitHub using the same address reaches the same account.
type User struct {
	ID        int64 `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Email     string `gorm:"type:varchar(255);uniqueIndex;not null"`
	Name      string `gorm:"type:varchar(255)"`
	Picture   string `gorm:"type:text"`
	// Provider is the identity provider the account was registered with.
	Provider string `gorm:"type:varchar(32)"`
}
