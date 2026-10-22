// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// RootLoginRequest is the root sign-in body.
	RootLoginRequest struct {
		Token string `json:"token"`
	}

	// Settings are the setup page settings. Secrets are write-only: they are
	// always empty in responses (see SecretsSet), and an empty secret in a
	// request keeps the stored value.
	Settings struct {
		Domain   DomainSettings   `json:"domain"`
		TLS      TLSSettings      `json:"tls"`
		Database DatabaseSettings `json:"database"`
		Auth     AuthSettings     `json:"auth"`
	}

	// DomainSettings is the domain section.
	DomainSettings struct {
		AppURL         string   `json:"appUrl"`
		AllowedOrigins []string `json:"allowedOrigins"`
	}

	// TLSSettings is the tls section.
	TLSSettings struct {
		Enabled bool   `json:"enabled"`
		Port    int    `json:"port"`
		Email   string `json:"email"`
	}

	// DatabaseSettings is the database section.
	DatabaseSettings struct {
		Driver   string           `json:"driver"`
		SQLite   SQLiteSettings   `json:"sqlite"`
		Postgres PostgresSettings `json:"postgres"`
	}

	// SQLiteSettings is the database.sqlite section.
	SQLiteSettings struct {
		DSN string `json:"dsn"`
	}

	// PostgresSettings is the database.postgres section.
	PostgresSettings struct {
		DSN string `json:"dsn"`
	}

	// AuthSettings is the auth section.
	AuthSettings struct {
		Google    OAuthClientSettings     `json:"google"`
		GitHub    OAuthClientSettings     `json:"github"`
		Providers []OAuthProviderSettings `json:"providers"`
	}

	// OAuthProviderSettings is a custom OpenID Connect provider.
	OAuthProviderSettings struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Icon         string `json:"icon"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		AuthURL      string `json:"authUrl"`
	}

	// OAuthClientSettings is an OAuth app registration.
	OAuthClientSettings struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}

	// SecretsSet reports which secrets have a stored value.
	SecretsSet struct {
		PostgresDSN           bool            `json:"postgresDsn"`
		GoogleClientSecret    bool            `json:"googleClientSecret"`
		GitHubClientSecret    bool            `json:"githubClientSecret"`
		ProviderClientSecrets map[string]bool `json:"providerClientSecrets"`
	}

	// SettingsRequest is the PUT /setup/settings body.
	SettingsRequest struct {
		Settings Settings `json:"settings"`
	}

	// SettingsResponse is the GET and PUT /setup/settings response.
	SettingsResponse struct {
		Settings        Settings   `json:"settings"`
		SecretsSet      SecretsSet `json:"secretsSet"`
		RestartRequired bool       `json:"restartRequired"`
	}
)
