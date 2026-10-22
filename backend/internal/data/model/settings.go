// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

type (
	// EncryptedSettings is the settings file: the JSON-encoded Settings,
	// sealed with the master key.
	EncryptedSettings struct {
		Version   int    `json:"version"`
		Algorithm string `json:"algorithm"`
		// Data is base64 of the nonce followed by the ciphertext.
		Data string `json:"data"`
	}

	// Settings is the plaintext (JSON) form of the setup page settings.
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
)
