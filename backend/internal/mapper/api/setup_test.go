// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
)

func runBody(t *testing.T, body string, fn func(c fiber.Ctx)) {
	t.Helper()
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		fn(c)
		return nil
	})
	_, err := app.Test(httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body)))
	require.NoError(t, err)
}

func TestFromHTTPRequestToRootLoginRequestEntity(t *testing.T) {
	cases := map[string]*entity.RootLoginRequest{
		`{"token":"t"}`: {Token: "t"},
		`{"token":""}`:  nil,
		`not json`:      nil,
	}
	for body, want := range cases {
		var got *entity.RootLoginRequest
		runBody(t, body, func(c fiber.Ctx) { got = FromHTTPRequestToRootLoginRequestEntity(c) })
		assert.Equal(t, want, got, body)
	}
}

func TestFromHTTPRequestToAuthenticateRootRequestEntity(t *testing.T) {
	var got *entity.AuthenticateRootRequest
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: RootCookie, Value: "r"})
	run(t, "/", req, func(c fiber.Ctx) { got = FromHTTPRequestToAuthenticateRootRequestEntity(c) })
	assert.Equal(t, &entity.AuthenticateRootRequest{SessionToken: "r"}, got)

	run(t, "/", httptest.NewRequest(http.MethodGet, "/", nil), func(c fiber.Ctx) { got = FromHTTPRequestToAuthenticateRootRequestEntity(c) })
	assert.Nil(t, got)
}

const _settingsJSON = `{
	"domain": {"appUrl": "https://tracker.topup.sh", "allowedOrigins": ["https://topup.sh"]},
	"tls": {"enabled": true, "port": 8443, "email": "ops@topup.sh"},
	"database": {"driver": "postgres", "sqlite": {"dsn": "x.db"},
		"postgres": {"dsn": "postgres://u:p@h/d"}},
	"auth": {"google": {"clientId": "g", "clientSecret": "gs"}, "github": {"clientId": "h", "clientSecret": "hs"},
		"providers": [{"id": "okta", "name": "Okta", "icon": "<svg/>", "clientId": "o", "clientSecret": "os", "authUrl": "https://okta.example.com"}]}
}`

func TestSettingsMapping(t *testing.T) {
	var got *entity.UpdateSettingsRequest
	runBody(t, `{"settings":`+_settingsJSON+`}`, func(c fiber.Ctx) { got = FromHTTPRequestToUpdateSettingsRequestEntity(c) })
	require.NotNil(t, got)
	assert.Equal(t, "postgres://u:p@h/d", got.Settings.Database.Postgres.DSN)
	assert.Equal(t, "hs", got.Settings.Auth.GitHub.ClientSecret)
	assert.Equal(t, entity.DomainSettings{AppURL: "https://tracker.topup.sh", AllowedOrigins: []string{"https://topup.sh"}}, got.Settings.Domain)
	assert.Equal(t, entity.TLSSettings{Enabled: true, Port: 8443, Email: "ops@topup.sh"}, got.Settings.TLS)
	assert.Equal(t, []entity.OAuthProviderSettings{{
		ID: "okta", Name: "Okta", Icon: "<svg/>", ClientID: "o", ClientSecret: "os", AuthURL: "https://okta.example.com",
	}}, got.Settings.Auth.Providers)
	// Entity -> view -> entity is lossless.
	assert.Equal(t, got.Settings, FromSettingsViewToSettingsEntity(FromSettingsEntityToSettingsView(got.Settings)))

	runBody(t, `nope`, func(c fiber.Ctx) { got = FromHTTPRequestToUpdateSettingsRequestEntity(c) })
	assert.Nil(t, got)

}

func TestFromSettingsResponseEntityToHTTPResponse(t *testing.T) {
	rs := &entity.GetSettingsResponse{
		Settings:        FromSettingsViewToSettingsEntity(FromSettingsEntityToSettingsView(entity.Settings{TLS: entity.TLSSettings{Port: 7}})),
		SecretsSet:      entity.SecretsSet{PostgresDSN: true, GoogleClientSecret: true, GitHubClientSecret: true},
		RestartRequired: true,
	}
	body := string(FromUpdateSettingsResponseEntityToHTTPResponse(rs))
	assert.Contains(t, body, `"port":7`)
	assert.Contains(t, body, `"secretsSet":{"postgresDsn":true,"googleClientSecret":true,"githubClientSecret":true,`+
		`"providerClientSecrets":{}}`)
	assert.Contains(t, body, `"providers":[]`)

	rs.SecretsSet = entity.SecretsSet{ProviderClientSecrets: map[string]bool{"okta": true}}
	body = string(FromGetSettingsResponseEntityToHTTPResponse(rs))
	assert.Contains(t, body, `"providerClientSecrets":{"okta":true}`)
	assert.Contains(t, body, `"restartRequired":true`)
}
