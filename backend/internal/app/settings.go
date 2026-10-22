// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"crypto/rand"
	"encoding/hex"

	zlog "github.com/rs/zerolog/log"

	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

const _defaultSettingsFile = "./_storage/settings.json"

func settingsFile(cfg Config) string {
	if cfg.App.SettingsFile != "" {
		return cfg.App.SettingsFile
	}
	return _defaultSettingsFile
}

// rootAccessToken returns the configured root token, or generates and logs
// one so the setup page can always be reached.
func rootAccessToken(cfg Config) string {
	if cfg.Auth.RootAccessToken != "" {
		return cfg.Auth.RootAccessToken
	}
	b := make([]byte, 24)
	// crypto/rand.Read never returns an error.
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	zlog.Warn().Str("token", tok).Msg("setup: generated root access token (set TOPUPSH_AUTH_ROOT_ACCESS_TOKEN to keep one across restarts)")
	return tok
}

// settingsFromConfig is what the setup page shows before anything is saved.
func settingsFromConfig(cfg Config) entity.Settings {
	driver := setupctrl.DriverSQLite
	if cfg.Postgres.Enabled {
		driver = setupctrl.DriverPostgres
	}
	tlsPort := cfg.TLS.Port
	if tlsPort == 0 {
		tlsPort = setupctrl.DefaultTLSPort
	}
	return entity.Settings{
		Domain: entity.DomainSettings{AppURL: cfg.App.BaseURL},
		TLS:    entity.TLSSettings{Enabled: cfg.TLS.Enabled, Port: tlsPort, Email: cfg.TLS.Email},
		Database: entity.DatabaseSettings{
			Driver:   driver,
			SQLite:   entity.SQLiteSettings{DSN: cfg.SQLite.DSN},
			Postgres: entity.PostgresSettings{DSN: cfg.Postgres.DSN},
		},
		Auth: entity.AuthSettings{
			Google: entity.OAuthClientSettings{ClientID: cfg.Auth.Google.ClientID, ClientSecret: cfg.Auth.Google.ClientSecret},
			GitHub: entity.OAuthClientSettings{ClientID: cfg.Auth.GitHub.ClientID, ClientSecret: cfg.Auth.GitHub.ClientSecret},
		},
	}
}

// applySettings overlays the effective settings onto the config.
func applySettings(cfg Config, s entity.Settings) Config {
	cfg.SQLite.Enabled = s.Database.Driver == setupctrl.DriverSQLite
	cfg.SQLite.DSN = s.Database.SQLite.DSN
	cfg.Postgres.Enabled = s.Database.Driver == setupctrl.DriverPostgres
	cfg.Postgres.DSN = s.Database.Postgres.DSN
	cfg.Auth.Google = OAuthAppConfig{ClientID: s.Auth.Google.ClientID, ClientSecret: s.Auth.Google.ClientSecret}
	cfg.Auth.GitHub = OAuthAppConfig{ClientID: s.Auth.GitHub.ClientID, ClientSecret: s.Auth.GitHub.ClientSecret}
	cfg.Auth.Providers = s.Auth.Providers
	cfg.App.BaseURL = s.Domain.AppURL
	cfg.TLS = TLSConfig{Enabled: s.TLS.Enabled, Port: s.TLS.Port, Email: s.TLS.Email}
	return cfg
}
