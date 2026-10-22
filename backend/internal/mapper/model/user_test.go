// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

func TestFromUserModelToUserEntity(t *testing.T) {
	now := time.Now()
	got := FromUserModelToUserEntity(model.User{ID: 7, Email: "a@b.c", Name: "A", Picture: "p", Provider: "google", CreatedAt: now})
	assert.Equal(t, entity.User{ID: 7, Email: "a@b.c", Name: "A", Picture: "p", Provider: "google", CreatedAt: now}, got)
}
