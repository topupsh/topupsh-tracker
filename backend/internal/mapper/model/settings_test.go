// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

func TestSettingsRoundTrip(t *testing.T) {
	e := entity.Settings{
		Domain: entity.DomainSettings{AppURL: "https://tracker.topup.sh", AllowedOrigins: []string{"https://topup.sh"}},
		TLS:    entity.TLSSettings{Enabled: true, Port: 443, Email: "ops@topup.sh"},
		Database: entity.DatabaseSettings{
			Driver:   "postgres",
			SQLite:   entity.SQLiteSettings{DSN: "x.db"},
			Postgres: entity.PostgresSettings{DSN: "postgres://u:p@h:5432/d?sslmode=require"},
		},
		Auth: entity.AuthSettings{
			Google: entity.OAuthClientSettings{ClientID: "g", ClientSecret: "gs"},
			GitHub: entity.OAuthClientSettings{ClientID: "h", ClientSecret: "hs"},
			Providers: []entity.OAuthProviderSettings{{
				ID: "okta", Name: "Okta", Icon: "<svg/>", ClientID: "o", ClientSecret: "os", AuthURL: "https://okta.example.com",
			}},
		},
	}
	m := FromSettingsEntityToSettingsModel(e)
	assert.Equal(t, "gs", m.Auth.Google.ClientSecret)
	assert.Equal(t, e, FromSettingsModelToSettingsEntity(m))
}

func TestSettingsRoundTrip_NoProviders(t *testing.T) {
	m := FromSettingsEntityToSettingsModel(entity.Settings{})
	assert.Equal(t, []model.OAuthProviderSettings{}, m.Auth.Providers, "saved as [] rather than null")
	assert.Nil(t, FromSettingsModelToSettingsEntity(m).Auth.Providers)
}
