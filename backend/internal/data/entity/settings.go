// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

type (
	// Settings are the server settings root can change from the setup page.
	Settings struct {
		Domain   DomainSettings
		TLS      TLSSettings
		Database DatabaseSettings
		Auth     AuthSettings
	}

	// DomainSettings choose where the app is served.
	DomainSettings struct {
		// AppURL is the public URL of the app, e.g. https://tracker.topup.sh.
		// Its host is the app host, and OAuth callbacks are built on it.
		AppURL string
		// AllowedOrigins are other web origins, e.g. https://topup.sh,
		// that may call the API from a browser (CORS, with credentials).
		// They apply without a restart.
		AllowedOrigins []string
	}

	// TLSSettings make the server terminate HTTPS itself, with Let's
	// Encrypt certificates it obtains and renews over the TLS-ALPN-01
	// challenge.
	TLSSettings struct {
		Enabled bool
		// Port is the HTTPS port; the app port then redirects to it.
		Port int
		// Email is the optional ACME account email.
		Email string
	}

	// DatabaseSettings choose and configure the database.
	DatabaseSettings struct {
		// Driver is "sqlite" or "postgres".
		Driver   string
		SQLite   SQLiteSettings
		Postgres PostgresSettings
	}

	// SQLiteSettings configure SQLite.
	SQLiteSettings struct {
		DSN string
	}

	// PostgresSettings configure PostgreSQL.
	PostgresSettings struct {
		// DSN is a postgres:// URL or a key=value connection string. It
		// usually holds the password, so it is a secret.
		DSN string
	}

	// AuthSettings configure the sign-in providers.
	AuthSettings struct {
		Google OAuthClientSettings
		GitHub OAuthClientSettings
		// Providers are custom OpenID Connect providers.
		Providers []OAuthProviderSettings
	}

	// OAuthProviderSettings are a custom OpenID Connect sign-in provider.
	OAuthProviderSettings struct {
		// ID is the provider's route name, derived from Name.
		ID   string
		Name string
		// Icon is the SVG markup shown on the sign-in button.
		Icon         string
		ClientID     string
		ClientSecret string
		// AuthURL is the OpenID Connect issuer; the endpoints are discovered
		// from its /.well-known/openid-configuration.
		AuthURL string
	}

	// OAuthClientSettings are an OAuth app registration.
	OAuthClientSettings struct {
		ClientID     string
		ClientSecret string
	}

	// SecretsSet reports which secrets have a value, since secrets are never
	// returned.
	SecretsSet struct {
		PostgresDSN        bool
		GoogleClientSecret bool
		GitHubClientSecret bool
		// ProviderClientSecrets is keyed by custom provider ID.
		ProviderClientSecrets map[string]bool
	}

	// RootLoginRequest carries a root access token.
	RootLoginRequest struct {
		Token string
	}

	// RootLoginResponse carries a root session.
	RootLoginResponse struct {
		SessionToken string
	}

	// AuthenticateRootRequest carries a root session token.
	AuthenticateRootRequest struct {
		SessionToken string
	}

	// GetSettingsResponse carries the settings with secrets removed.
	GetSettingsResponse struct {
		Settings   Settings
		SecretsSet SecretsSet
		// RestartRequired is true when saved settings differ from those the
		// server started with in a way that only a restart applies.
		RestartRequired bool
	}

	// UpdateSettingsRequest replaces the settings. Empty secrets keep their
	// current value.
	UpdateSettingsRequest struct {
		Settings Settings
	}

	// UpdateSettingsResponse is the saved state.
	UpdateSettingsResponse = GetSettingsResponse
)
