// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/brand"
	"github.com/topupsh/topupsh-tracker/backend/internal/handler/api"
)

func TestHostsIsApp(t *testing.T) {
	h := newHosts("https://App.Example.com")
	assert.False(t, h.open)
	cases := map[string]bool{
		"app.example.com":     true,
		"APP.example.com:443": true,
		"localhost:2027":      true,
		"127.0.0.1":           true,
		"[::1]:2027":          true,
		"www.example.com":     false,
		"example.com":         false,
		"evil.com":            false,
	}
	for host, want := range cases {
		assert.Equal(t, want, h.isApp(host), host)
	}

	// An app URL on this machine or an IP answers any host, as in
	// development.
	for _, appURL := range []string{"http://localhost:2027", "http://192.168.1.2:2027"} {
		open := newHosts(appURL)
		assert.True(t, open.open)
		assert.True(t, open.isApp("anything.lan"))
	}
}

func TestHostRouter(t *testing.T) {
	cfg := testConfig(t)
	cfg.App.BaseURL = "https://app.example.com"
	a, err := New(cfg, fstest.MapFS{})
	require.NoError(t, err)

	request := func(host, path string) (*http.Response, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = host
		res, err := a.Handler().Test(req)
		require.NoError(t, err)
		b, _ := io.ReadAll(res.Body)
		return res, string(b)
	}

	res, _ := request("app.example.com", "/auth/config")
	assert.Equal(t, http.StatusOK, res.StatusCode)

	res, body := request("evil.com", "/auth/config")
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	assert.Equal(t, fiber.MIMEApplicationJSON, res.Header.Get(fiber.HeaderContentType))
	assert.JSONEq(t, `{"error":"host not allowed"}`, body)

	res, body = request("app.example.com", api.FaviconPath)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, brand.FaviconType, res.Header.Get(fiber.HeaderContentType))
	assert.Equal(t, string(brand.Favicon), body)
	res, _ = request("evil.com", api.FaviconPath)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	// An https app URL makes the cookies Secure.
	logout := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logout.Host = "app.example.com"
	res, err = a.Handler().Test(logout)
	require.NoError(t, err)
	require.NotEmpty(t, res.Cookies())
	assert.True(t, res.Cookies()[0].Secure)
}

func TestHTTPSRedirect(t *testing.T) {
	h := newHosts("https://app.example.com")
	cases := []struct {
		host string
		port int
		want string
	}{
		{"app.example.com", 443, "https://app.example.com/x?y=1"},
		{"app.example.com:2027", 8443, "https://app.example.com:8443/x?y=1"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x?y=1", nil)
		req.Host = tc.host
		res, err := httpsRedirect(h, tc.port).Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, res.StatusCode)
		assert.Equal(t, tc.want, res.Header.Get("Location"))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "evil.com"
	res, err := httpsRedirect(h, 443).Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
}
