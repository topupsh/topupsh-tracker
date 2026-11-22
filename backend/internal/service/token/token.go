// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package token issues and validates the signed JWTs used for browser
// sessions, root (setup) sessions, OAuth state and credit view tokens. All
// of them are stateless; each kind has its own audience, so one can never
// pass for another.
package token

//go:generate mockgen -source=token.go -destination=mocks/token.go -package=mocks

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// SessionTTL is how long a browser session lasts.
	SessionTTL = 7 * 24 * time.Hour
	// StateTTL is how long a user has to finish an OAuth round trip.
	StateTTL = 5 * time.Minute
	// RootTTL is how long a root (setup) session lasts.
	RootTTL = 12 * time.Hour
	// ViewTTL is how long a credit view token lasts.
	ViewTTL = time.Hour

	_sessionAudience = "session:human"
	_rootAudience    = "session:root"
	_viewAudience    = "view:credits"
	_rootSubject     = "root"
	_stateSubject    = "oauth_state"
)

// ErrInvalidToken is returned for any token that fails validation.
var ErrInvalidToken = errors.New("invalid token")

type (
	// Service creates and validates tokens.
	Service interface {
		// CreateSessionToken signs a session for the given user ID.
		CreateSessionToken(userID string) (string, error)
		// ValidateSessionToken returns the user ID a session belongs to.
		ValidateSessionToken(token string) (string, error)
		// CreateStateToken signs an OAuth state bound to a provider and the
		// in-app path to return to.
		CreateStateToken(provider, redirectURL string) (string, error)
		// ValidateStateToken returns the redirect path carried by a state
		// issued for provider.
		ValidateStateToken(token, provider string) (string, error)
		// CreateRootToken signs a root (setup) session.
		CreateRootToken() (string, error)
		// ValidateRootToken checks a root session.
		ValidateRootToken(token string) error
		// CreateViewToken signs a token that lets one user of a service
		// read their own credits, and returns it with its expiry.
		CreateViewToken(v Viewer) (string, time.Time, error)
		// ValidateViewToken returns the user a view token was issued for.
		ValidateViewToken(tok string) (Viewer, error)
	}

	// Viewer is the user of a service a view token was issued for.
	Viewer struct {
		ServiceID int64
		OrgID     string
		UserID    string
	}

	// Config holds the token signing settings.
	Config struct {
		Secret string
		Now    func() time.Time
	}

	service struct {
		secret []byte
		now    func() time.Time
	}

	stateClaims struct {
		jwt.RegisteredClaims
		RedirectURL string `json:"rurl,omitempty"`
	}

	viewClaims struct {
		jwt.RegisteredClaims
		ServiceID int64  `json:"svc"`
		OrgID     string `json:"org"`
	}
)

// New returns a token service. The secret must not be empty.
func New(cfg Config) (Service, error) {
	if cfg.Secret == "" {
		return nil, errors.New("token: secret is required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &service{secret: []byte(cfg.Secret), now: now}, nil
}

func (s *service) CreateSessionToken(userID string) (string, error) {
	now := s.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Audience:  jwt.ClaimStrings{_sessionAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(SessionTTL)),
	}
	return s.sign(claims)
}

func (s *service) ValidateSessionToken(token string) (string, error) {
	var claims jwt.RegisteredClaims
	if err := s.parse(token, &claims, _sessionAudience); err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

func (s *service) CreateStateToken(provider, redirectURL string) (string, error) {
	now := s.now()
	claims := stateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   _stateSubject,
			Audience:  jwt.ClaimStrings{provider},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(StateTTL)),
		},
		RedirectURL: redirectURL,
	}
	return s.sign(claims)
}

func (s *service) ValidateStateToken(token, provider string) (string, error) {
	var claims stateClaims
	if err := s.parse(token, &claims, provider); err != nil {
		return "", err
	}
	if claims.Subject != _stateSubject {
		return "", ErrInvalidToken
	}
	return claims.RedirectURL, nil
}

func (s *service) CreateRootToken() (string, error) {
	now := s.now()
	return s.sign(jwt.RegisteredClaims{
		Subject:   _rootSubject,
		Audience:  jwt.ClaimStrings{_rootAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(RootTTL)),
	})
}

func (s *service) ValidateRootToken(token string) error {
	var claims jwt.RegisteredClaims
	if err := s.parse(token, &claims, _rootAudience); err != nil {
		return err
	}
	if claims.Subject != _rootSubject {
		return ErrInvalidToken
	}
	return nil
}

func (s *service) CreateViewToken(v Viewer) (string, time.Time, error) {
	now := s.now()
	exp := now.Add(ViewTTL)
	tok, err := s.sign(viewClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   v.UserID,
			Audience:  jwt.ClaimStrings{_viewAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		ServiceID: v.ServiceID,
		OrgID:     v.OrgID,
	})
	return tok, exp, err
}

func (s *service) ValidateViewToken(token string) (Viewer, error) {
	var claims viewClaims
	if err := s.parse(token, &claims, _viewAudience); err != nil {
		return Viewer{}, err
	}
	if claims.Subject == "" || claims.OrgID == "" || claims.ServiceID == 0 {
		return Viewer{}, ErrInvalidToken
	}
	return Viewer{ServiceID: claims.ServiceID, OrgID: claims.OrgID, UserID: claims.Subject}, nil
}

func (s *service) sign(claims jwt.Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *service) parse(token string, claims jwt.Claims, audience string) error {
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return nil
}
