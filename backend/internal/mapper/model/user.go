// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package model maps between database models and domain entities.
package model

import (
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// FromUserModelToUserEntity maps a stored user to the domain user.
func FromUserModelToUserEntity(m model.User) entity.User {
	return entity.User{
		ID:        m.ID,
		Email:     m.Email,
		Name:      m.Name,
		Picture:   m.Picture,
		Provider:  m.Provider,
		CreatedAt: m.CreatedAt,
	}
}
