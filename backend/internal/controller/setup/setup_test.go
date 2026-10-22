// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package setup

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	mappermodel "github.com/topupsh/topupsh-tracker/backend/internal/mapper/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	mockrepo "github.com/topupsh/topupsh-tracker/backend/internal/repository/mocks"
	mocktoken "github.com/topupsh/topupsh-tracker/backend/internal/service/token/mocks"
)

var errBoom = errors.New("boom")

func defaults() entity.Settings {
	return entity.Settings{
		Domain: entity.DomainSettings{AppURL: "http://localhost:2027"},
		TLS:    entity.TLSSettings{Port: DefaultTLSPort},
		Database: entity.DatabaseSettings{
			Driver:   DriverSQLite,
			SQLite:   entity.SQLiteSettings{DSN: "./_storage/topupsh-tracker.db"},
			Postgres: entity.PostgresSettings{DSN: "postgres://postgres:pgpass@localhost:5432/topupsh_tracker?sslmode=disable"},
		},
		Auth: entity.AuthSettings{
			Google: entity.OAuthClientSettings{ClientID: "g", ClientSecret: "gsecret"},
		},
	}
}

type env struct {
	ctrl  Controller
	repo  *mockrepo.MockSettingsRepository
	token *mocktoken.MockService
}

func newEnv(t *testing.T) *env {
	t.Helper()
	mc := gomock.NewController(t)
	e := &env{repo: mockrepo.NewMockSettingsRepository(mc), token: mocktoken.NewMockService(mc)}
	e.repo.EXPECT().LoadSettings().Return(model.Settings{}, repository.ErrNotFound)
	ctrl, err := New(Params{Repository: e.repo, Token: e.token, RootAccessToken: "root-token", Defaults: defaults()})
	require.NoError(t, err)
	e.ctrl = ctrl
	return e
}

func TestNew(t *testing.T) {
	mc := gomock.NewController(t)

	t.Run("requires a root token", func(t *testing.T) {
		_, err := New(Params{})
		assert.Error(t, err)
	})

	t.Run("saved settings win over defaults", func(t *testing.T) {
		repo := mockrepo.NewMockSettingsRepository(mc)
		saved := defaults()
		saved.Domain.AllowedOrigins = []string{"https://topup.sh"}
		repo.EXPECT().LoadSettings().Return(mappermodel.FromSettingsEntityToSettingsModel(saved), nil)
		ctrl, err := New(Params{Repository: repo, RootAccessToken: "x", Defaults: defaults()})
		require.NoError(t, err)
		assert.Equal(t, []string{"https://topup.sh"}, ctrl.AllowedOrigins())
		assert.Equal(t, saved, ctrl.Current())
	})

	t.Run("saved settings without an app URL keep the configured one", func(t *testing.T) {
		repo := mockrepo.NewMockSettingsRepository(mc)
		saved := defaults()
		saved.Domain.AppURL = ""
		saved.TLS.Port = 0
		repo.EXPECT().LoadSettings().Return(mappermodel.FromSettingsEntityToSettingsModel(saved), nil)
		ctrl, err := New(Params{Repository: repo, RootAccessToken: "x", Defaults: defaults()})
		require.NoError(t, err)
		assert.Equal(t, defaults(), ctrl.Current())
	})

	t.Run("load error", func(t *testing.T) {
		repo := mockrepo.NewMockSettingsRepository(mc)
		repo.EXPECT().LoadSettings().Return(model.Settings{}, errBoom)
		_, err := New(Params{Repository: repo, RootAccessToken: "x"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("wrong token", func(t *testing.T) {
		e := newEnv(t)
		for _, tok := range []string{"", "root", "root-token-but-longer"} {
			_, err := e.ctrl.Login(ctx, &entity.RootLoginRequest{Token: tok})
			assert.ErrorIs(t, err, ErrInvalidRootToken, tok)
		}
	})

	t.Run("ok", func(t *testing.T) {
		e := newEnv(t)
		e.token.EXPECT().CreateRootToken().Return("root-session", nil)
		rs, err := e.ctrl.Login(ctx, &entity.RootLoginRequest{Token: "root-token"})
		require.NoError(t, err)
		assert.Equal(t, "root-session", rs.SessionToken)
	})

	t.Run("session error", func(t *testing.T) {
		e := newEnv(t)
		e.token.EXPECT().CreateRootToken().Return("", errBoom)
		_, err := e.ctrl.Login(ctx, &entity.RootLoginRequest{Token: "root-token"})
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestAuthenticate(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	assert.ErrorIs(t, e.ctrl.Authenticate(ctx, &entity.AuthenticateRootRequest{}), ErrUnauthorized)

	e.token.EXPECT().ValidateRootToken("bad").Return(errBoom)
	assert.ErrorIs(t, e.ctrl.Authenticate(ctx, &entity.AuthenticateRootRequest{SessionToken: "bad"}), ErrUnauthorized)

	e.token.EXPECT().ValidateRootToken("good").Return(nil)
	assert.NoError(t, e.ctrl.Authenticate(ctx, &entity.AuthenticateRootRequest{SessionToken: "good"}))
}

func TestGetSettings_HidesSecrets(t *testing.T) {
	e := newEnv(t)
	rs := e.ctrl.GetSettings(context.Background())
	assert.Empty(t, rs.Settings.Database.Postgres.DSN)
	assert.Empty(t, rs.Settings.Auth.Google.ClientSecret)
	assert.Equal(t, entity.SecretsSet{
		PostgresDSN: true, GoogleClientSecret: true, ProviderClientSecrets: map[string]bool{},
	}, rs.SecretsSet)
	assert.Equal(t, "g", rs.Settings.Auth.Google.ClientID)
	assert.False(t, rs.RestartRequired)
}

func TestUpdateSettings_AllowedOriginsApplyLive(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	next := e.ctrl.GetSettings(ctx).Settings // secrets blank: kept
	next.Domain.AllowedOrigins = []string{" HTTPS://Topup.sh/ ", "", "https://www.topup.sh", "https://topup.sh"}

	e.repo.EXPECT().SaveSettings(gomock.Any()).DoAndReturn(func(m model.Settings) error {
		assert.Equal(t, "postgres://postgres:pgpass@localhost:5432/topupsh_tracker?sslmode=disable", m.Database.Postgres.DSN)
		assert.Equal(t, "gsecret", m.Auth.Google.ClientSecret)
		return nil
	})
	rs, err := e.ctrl.UpdateSettings(ctx, &entity.UpdateSettingsRequest{Settings: next})
	require.NoError(t, err)
	assert.False(t, rs.RestartRequired, "allowed origins apply without a restart")
	assert.Equal(t, []string{"https://topup.sh", "https://www.topup.sh"}, e.ctrl.AllowedOrigins(), "normalized, blanks and repeats dropped")
	assert.Empty(t, rs.Settings.Auth.Google.ClientSecret)
}

func TestUpdateSettings_RestartRequired(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	next := e.ctrl.GetSettings(ctx).Settings
	next.Database.Driver = " Postgres "
	next.Database.Postgres.DSN = " host=db user=app password=newpass dbname=prod "
	next.Auth.Google.ClientID = "" // disables Google, dropping its secret
	next.Auth.GitHub = entity.OAuthClientSettings{ClientID: " h ", ClientSecret: "hsecret"}

	e.repo.EXPECT().SaveSettings(gomock.Any()).Return(nil)
	rs, err := e.ctrl.UpdateSettings(ctx, &entity.UpdateSettingsRequest{Settings: next})
	require.NoError(t, err)
	assert.True(t, rs.RestartRequired)

	cur := e.ctrl.Current()
	assert.Equal(t, DriverPostgres, cur.Database.Driver)
	assert.Equal(t, "host=db user=app password=newpass dbname=prod", cur.Database.Postgres.DSN)
	assert.Equal(t, entity.OAuthClientSettings{}, cur.Auth.Google)
	assert.Equal(t, entity.OAuthClientSettings{ClientID: "h", ClientSecret: "hsecret"}, cur.Auth.GitHub)
}

func TestUpdateSettings_SaveFails(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	next := e.ctrl.GetSettings(ctx).Settings
	next.Domain.AllowedOrigins = []string{"https://topup.sh"}
	e.repo.EXPECT().SaveSettings(gomock.Any()).Return(errBoom)
	_, err := e.ctrl.UpdateSettings(ctx, &entity.UpdateSettingsRequest{Settings: next})
	assert.ErrorIs(t, err, errBoom)
	assert.Empty(t, e.ctrl.AllowedOrigins(), "unchanged on failure")
}

func TestUpdateSettings_Validation(t *testing.T) {
	cases := map[string]func(*entity.Settings){
		"unknown driver":     func(s *entity.Settings) { s.Database.Driver = "mysql" },
		"sqlite without dsn": func(s *entity.Settings) { s.Database.SQLite.DSN = " " },
		"postgres bad dsn url": func(s *entity.Settings) {
			s.Database.Driver = DriverPostgres
			s.Database.Postgres.DSN = "postgres://u:p@db:notaport/x"
		},
		"postgres bad dsn keywords": func(s *entity.Settings) {
			s.Database.Driver = DriverPostgres
			s.Database.Postgres.DSN = "host=db sslmode=sometimes"
		},
		"github id without secret": func(s *entity.Settings) { s.Auth.GitHub = entity.OAuthClientSettings{ClientID: "h"} },
		"origin with path":         func(s *entity.Settings) { s.Domain.AllowedOrigins = []string{"https://topup.sh/x"} },
		"origin without scheme":    func(s *entity.Settings) { s.Domain.AllowedOrigins = []string{"topup.sh"} },
		"origin not http":          func(s *entity.Settings) { s.Domain.AllowedOrigins = []string{"ftp://topup.sh"} },
		"origin with query":        func(s *entity.Settings) { s.Domain.AllowedOrigins = []string{"https://topup.sh?"} },
		"origin unparsable":        func(s *entity.Settings) { s.Domain.AllowedOrigins = []string{"http://[::1"} },
		"too many origins": func(s *entity.Settings) {
			for i := range _maxOrigins + 1 {
				s.Domain.AllowedOrigins = append(s.Domain.AllowedOrigins, fmt.Sprintf("https://o%d.topup.sh", i))
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			next := defaults()
			mutate(&next)
			_, err := e.ctrl.UpdateSettings(context.Background(), &entity.UpdateSettingsRequest{Settings: next})
			assert.ErrorIs(t, err, ErrInvalidSettings)
		})
	}
}

func TestUpdateSettings_ProviderIDWithoutAnySecret(t *testing.T) {
	// Nothing stored to keep, and no new secret: rejected.
	e := newEnvWith(t, func(s *entity.Settings) { s.Auth.Google = entity.OAuthClientSettings{} })
	next := defaults()
	next.Auth.Google = entity.OAuthClientSettings{ClientID: "g2"}
	_, err := e.ctrl.UpdateSettings(context.Background(), &entity.UpdateSettingsRequest{Settings: next})
	assert.ErrorIs(t, err, ErrInvalidSettings)
}

func TestUpdateSettings_PostgresDSN(t *testing.T) {
	t.Run("required when nothing is stored", func(t *testing.T) {
		e := newEnvWith(t, func(s *entity.Settings) { s.Database.Postgres.DSN = "" })
		next := defaults()
		next.Database.Driver = DriverPostgres
		next.Database.Postgres.DSN = ""
		_, err := e.ctrl.UpdateSettings(context.Background(), &entity.UpdateSettingsRequest{Settings: next})
		assert.ErrorIs(t, err, ErrInvalidSettings)
	})

	t.Run("invalid DSN is not echoed", func(t *testing.T) {
		e := newEnv(t)
		next := defaults()
		next.Database.Driver = DriverPostgres
		next.Database.Postgres.DSN = "postgres://u:hunter2@db:notaport/x"
		_, err := e.ctrl.UpdateSettings(context.Background(), &entity.UpdateSettingsRequest{Settings: next})
		assert.ErrorIs(t, err, ErrInvalidSettings)
		assert.NotContains(t, err.Error(), "hunter2")
	})
}

func newEnvWith(t *testing.T, mutate func(*entity.Settings)) *env {
	t.Helper()
	mc := gomock.NewController(t)
	e := &env{repo: mockrepo.NewMockSettingsRepository(mc), token: mocktoken.NewMockService(mc)}
	d := defaults()
	mutate(&d)
	e.repo.EXPECT().LoadSettings().Return(model.Settings{}, repository.ErrNotFound)
	ctrl, err := New(Params{Repository: e.repo, Token: e.token, RootAccessToken: "root-token", Defaults: d})
	require.NoError(t, err)
	e.ctrl = ctrl
	return e
}
