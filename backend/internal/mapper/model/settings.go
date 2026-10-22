// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

import (
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// FromSettingsModelToSettingsEntity maps stored settings to the domain.
func FromSettingsModelToSettingsEntity(m model.Settings) entity.Settings {
	return entity.Settings{
		Domain: entity.DomainSettings{AppURL: m.Domain.AppURL, AllowedOrigins: m.Domain.AllowedOrigins},
		TLS: entity.TLSSettings{
			Enabled: m.TLS.Enabled,
			Port:    m.TLS.Port,
			Email:   m.TLS.Email,
		},
		Database: entity.DatabaseSettings{
			Driver:   m.Database.Driver,
			SQLite:   entity.SQLiteSettings{DSN: m.Database.SQLite.DSN},
			Postgres: entity.PostgresSettings{DSN: m.Database.Postgres.DSN},
		},
		Auth: entity.AuthSettings{
			Google:    entity.OAuthClientSettings{ClientID: m.Auth.Google.ClientID, ClientSecret: m.Auth.Google.ClientSecret},
			GitHub:    entity.OAuthClientSettings{ClientID: m.Auth.GitHub.ClientID, ClientSecret: m.Auth.GitHub.ClientSecret},
			Providers: FromOAuthProviderSettingsModelsToOAuthProviderSettingsEntities(m.Auth.Providers),
		},
	}
}

// FromSettingsEntityToSettingsModel maps domain settings to storage.
func FromSettingsEntityToSettingsModel(e entity.Settings) model.Settings {
	return model.Settings{
		Domain: model.DomainSettings{AppURL: e.Domain.AppURL, AllowedOrigins: e.Domain.AllowedOrigins},
		TLS: model.TLSSettings{
			Enabled: e.TLS.Enabled,
			Port:    e.TLS.Port,
			Email:   e.TLS.Email,
		},
		Database: model.DatabaseSettings{
			Driver:   e.Database.Driver,
			SQLite:   model.SQLiteSettings{DSN: e.Database.SQLite.DSN},
			Postgres: model.PostgresSettings{DSN: e.Database.Postgres.DSN},
		},
		Auth: model.AuthSettings{
			Google:    model.OAuthClientSettings{ClientID: e.Auth.Google.ClientID, ClientSecret: e.Auth.Google.ClientSecret},
			GitHub:    model.OAuthClientSettings{ClientID: e.Auth.GitHub.ClientID, ClientSecret: e.Auth.GitHub.ClientSecret},
			Providers: FromOAuthProviderSettingsEntitiesToOAuthProviderSettingsModels(e.Auth.Providers),
		},
	}
}

// FromOAuthProviderSettingsModelsToOAuthProviderSettingsEntities maps custom
// sign-in providers to the domain.
func FromOAuthProviderSettingsModelsToOAuthProviderSettingsEntities(ps []model.OAuthProviderSettings) []entity.OAuthProviderSettings {
	if len(ps) == 0 {
		return nil
	}
	out := make([]entity.OAuthProviderSettings, len(ps))
	for i, p := range ps {
		out[i] = entity.OAuthProviderSettings{
			ID: p.ID, Name: p.Name, Icon: p.Icon, ClientID: p.ClientID, ClientSecret: p.ClientSecret, AuthURL: p.AuthURL,
		}
	}
	return out
}

// FromOAuthProviderSettingsEntitiesToOAuthProviderSettingsModels maps custom
// sign-in providers from the domain.
func FromOAuthProviderSettingsEntitiesToOAuthProviderSettingsModels(es []entity.OAuthProviderSettings) []model.OAuthProviderSettings {
	out := make([]model.OAuthProviderSettings, len(es))
	for i, e := range es {
		out[i] = model.OAuthProviderSettings{
			ID: e.ID, Name: e.Name, Icon: e.Icon, ClientID: e.ClientID, ClientSecret: e.ClientSecret, AuthURL: e.AuthURL,
		}
	}
	return out
}
