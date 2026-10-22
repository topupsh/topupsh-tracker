// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package app wires the services, controllers and handlers into a server.
package app

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"gorm.io/gorm"

	adminctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/admin"
	authctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/auth"
	setupctrl "github.com/topupsh/topupsh-tracker/backend/internal/controller/setup"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/handler/api"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/encryption"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/idgen"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/oauth"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/scheduler"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/tlscert"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/token"
)

// _listenConfig is how every server listens; main logs the address.
var _listenConfig = fiber.ListenConfig{DisableStartupMessage: true}

type (
	// Config is the server configuration, one field per config section.
	Config struct {
		// MasterKey encrypts the settings file. It comes only from the
		// TOPUPSH_MASTER_KEY environment variable, never from YAML.
		MasterKey string
		App       AppConfig
		TLS       TLSConfig
		Auth      AuthConfig
		SQLite    SQLiteConfig
		Postgres  PostgresConfig
		IDGen     IDGenConfig
	}

	// AppConfig is the "app" section.
	AppConfig struct {
		Port int `yaml:"port"`
		// BaseURL is the public URL of the app. Its host is the app host,
		// and OAuth callbacks are built on it.
		BaseURL string `yaml:"baseUrl"`
		// BasePath is the path prefix browsers reach the app under when a
		// proxy strips it before forwarding ("" or "/x").
		BasePath     string `yaml:"basePath"`
		CookieSecure bool   `yaml:"cookieSecure"`
		// SettingsFile is where the setup page saves settings. When it
		// exists, it overrides the matching config values.
		SettingsFile string `yaml:"settingsFile"`
		// TrustedProxies is a comma-separated list of proxy IPs or CIDR
		// ranges. Requests from them are attributed to the client IP in
		// ProxyHeader, which the per-IP rate limits count.
		TrustedProxies string `yaml:"trustedProxies"`
		// ProxyHeader is the header a trusted proxy puts the client IP in.
		ProxyHeader string `yaml:"proxyHeader"`
	}

	// TLSConfig is the "tls" section: HTTPS with Let's Encrypt
	// certificates, obtained and renewed automatically.
	TLSConfig struct {
		Enabled bool `yaml:"enabled"`
		// Port is the HTTPS port; the app port then redirects to it.
		Port int `yaml:"port"`
		// Email is the optional ACME account email.
		Email string `yaml:"email"`
	}

	// AuthConfig is the "auth" section.
	AuthConfig struct {
		JWTSecret string `yaml:"jwtSecret"`
		// RootAccessToken unlocks the setup page. A random one is generated
		// (and logged) at startup when it is empty.
		RootAccessToken string         `yaml:"rootAccessToken"`
		Google          OAuthAppConfig `yaml:"google"`
		GitHub          OAuthAppConfig `yaml:"github"`
		// Providers are the custom OpenID Connect providers, set only on
		// the setup page.
		Providers []entity.OAuthProviderSettings `yaml:"-"`
	}

	// OAuthAppConfig is an OAuth client registration. A provider without a
	// client ID is disabled.
	OAuthAppConfig struct {
		ClientID     string `yaml:"clientId"`
		ClientSecret string `yaml:"clientSecret"`
	}

	// PoolConfig tunes a database connection pool.
	PoolConfig struct {
		MaxIdleConns    int           `yaml:"maxIdleConns"`
		MaxOpenConns    int           `yaml:"maxOpenConns"`
		MaxConnLifetime time.Duration `yaml:"maxConnLifetime"`
	}

	// SQLiteConfig is the "sqlite" section.
	SQLiteConfig struct {
		Enabled    bool   `yaml:"enabled"`
		DSN        string `yaml:"dsn"`
		PoolConfig `yaml:",inline"`
	}

	// PostgresConfig is the "postgres" section. PostgreSQL wins when both
	// databases are enabled.
	PostgresConfig struct {
		Enabled bool `yaml:"enabled"`
		// DSN is a postgres:// URL or a key=value connection string.
		DSN        string `yaml:"dsn"`
		PoolConfig `yaml:",inline"`
	}

	// IDGenConfig is the "idgen" section.
	IDGenConfig struct {
		Node uint16 `yaml:"node"`
	}

	// App is a configured, runnable server.
	App struct {
		fiber     *fiber.App
		addr      string
		db        *gorm.DB
		scheduler scheduler.Service
		// The rest is set only with TLS enabled.
		tlsAddr  string
		certs    tlscert.Service
		tlsPort  int
		redirect *fiber.App
	}
)

// New builds the server. web holds the built frontend; it may be empty.
func New(cfg Config, web fs.FS) (*App, error) {
	tokens, err := token.New(token.Config{Secret: cfg.Auth.JWTSecret})
	if err != nil {
		return nil, err
	}
	enc, err := encryption.New(cfg.MasterKey)
	if err != nil {
		return nil, err
	}
	setup, err := setupctrl.New(setupctrl.Params{
		Repository:      repository.NewSettingsRepository(settingsFile(cfg), enc),
		Token:           tokens,
		RootAccessToken: rootAccessToken(cfg),
		Defaults:        settingsFromConfig(cfg),
	})
	if err != nil {
		return nil, err
	}
	// Saved settings override the config file and environment.
	cfg = applySettings(cfg, setup.Current())

	db, err := openDatabase(cfg)
	if err != nil {
		return nil, err
	}

	ids := idgen.New(cfg.IDGen.Node)
	users := repository.NewUserRepository(db)
	basePath := strings.TrimRight(cfg.App.BasePath, "/")
	auth := authctrl.New(authctrl.Params{
		Providers: providers(cfg),
		Token:     tokens,
		Users:     users,
		IDGen:     ids,
		BasePath:  basePath,
	})
	admin := adminctrl.New(adminctrl.Params{
		Users: users,
		Stats: repository.NewStatsRepository(db),
	})

	h := newHosts(cfg.App.BaseURL)
	f := fiber.New(fiberConfig(cfg.App))
	f.Use(recover.New())
	api.New(api.Params{
		Router:         f,
		Hosts:          hostRouter(h),
		Auth:           auth,
		Setup:          setup,
		Admin:          admin,
		AllowedOrigins: setup.AllowedOrigins,
		BasePath:       basePath,
		// A session cookie for an https app must never travel over plain
		// http.
		CookieSecure: cfg.App.CookieSecure || strings.HasPrefix(cfg.App.BaseURL, "https://"),
	})
	f.Get("/*", spaHandler(web, basePath))

	a := &App{
		fiber:     f,
		addr:      net.JoinHostPort("", strconv.Itoa(cfg.App.Port)),
		db:        db,
		scheduler: scheduler.New(),
	}
	if cfg.TLS.Enabled {
		a.setupTLS(cfg, enc, h)
	}
	return a, nil
}

// fiberConfig is the app server's config. With trusted proxies, c.IP() is
// the client IP that they forward in the proxy header.
func fiberConfig(app AppConfig) fiber.Config {
	var cfg fiber.Config
	var proxies []string
	for p := range strings.SplitSeq(app.TrustedProxies, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	if len(proxies) == 0 {
		return cfg
	}
	cfg.TrustProxy = true
	cfg.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: proxies}
	cfg.ProxyHeader = app.ProxyHeader
	if cfg.ProxyHeader == "" {
		cfg.ProxyHeader = fiber.HeaderXForwardedFor
	}
	// The header may list several hops; c.IP() then takes the rightmost IP
	// that is not a trusted proxy, so a client can't spoof its own.
	cfg.EnableIPValidation = true
	return cfg
}

// Handler exposes the HTTP handler, for tests.
func (a *App) Handler() *fiber.App { return a.fiber }

// Listen starts the background jobs and serves until Shutdown is called.
// With TLS enabled the app serves HTTPS, and the app port redirects to it.
func (a *App) Listen() error {
	a.scheduler.Start()
	if a.certs == nil {
		return a.fiber.Listen(a.addr, _listenConfig)
	}
	return a.listenTLS()
}

// Shutdown stops the server and the jobs gracefully, then closes the
// database.
func (a *App) Shutdown(ctx context.Context) error {
	err := a.fiber.ShutdownWithContext(ctx)
	if a.certs != nil {
		err = errors.Join(err, a.redirect.ShutdownWithContext(ctx))
		a.certs.Stop()
	}
	a.scheduler.Stop()
	return errors.Join(err, a.closeDB())
}

func (a *App) closeDB() error {
	// The dialectors are backed by *sql.DB, so DB() cannot fail.
	sqlDB, _ := a.db.DB()
	return sqlDB.Close()
}

func providers(cfg Config) []oauth.Provider {
	callback := func(name string) string {
		return strings.TrimRight(cfg.App.BaseURL, "/") + api.AuthPrefix + "/" + name + "/callback"
	}
	var out []oauth.Provider
	if g := cfg.Auth.Google; g.ClientID != "" {
		out = append(out, oauth.NewGoogle(oauth.Credentials{ClientID: g.ClientID, ClientSecret: g.ClientSecret, RedirectURL: callback(oauth.ProviderGoogle)}))
	}
	if g := cfg.Auth.GitHub; g.ClientID != "" {
		out = append(out, oauth.NewGitHub(oauth.Credentials{ClientID: g.ClientID, ClientSecret: g.ClientSecret, RedirectURL: callback(oauth.ProviderGitHub)}))
	}
	for _, p := range cfg.Auth.Providers {
		out = append(out, oauth.NewOIDC(oauth.OIDCParams{
			Name: p.ID, Label: p.Name, Icon: p.Icon, Issuer: p.AuthURL,
			Credentials: oauth.Credentials{ClientID: p.ClientID, ClientSecret: p.ClientSecret, RedirectURL: callback(p.ID)},
		}))
	}
	return out
}
