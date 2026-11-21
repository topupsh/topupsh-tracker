// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package servicekey issues, revokes and checks the API keys services call
// the API with. A key identifies one service, so a service only reaches
// its own credits.
package servicekey

//go:generate mockgen -source=servicekey.go -destination=mocks/servicekey.go -package=mocks

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	mappermodel "github.com/topupsh/topupsh-tracker/backend/internal/mapper/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/idgen"
)

const (
	// KeyPrefix starts every key, so a leaked one is easy to spot.
	KeyPrefix = "tpk_"
	// _keyBytes is how much randomness a key carries.
	_keyBytes = 32
	// _shownPrefixLen is how much of a key is kept to tell keys apart.
	_shownPrefixLen = len(KeyPrefix) + 8
)

var (
	// ErrInvalidServiceName is returned for a name that is not 1 to 64
	// lowercase letters, digits or dashes, starting with a letter or digit.
	ErrInvalidServiceName = errors.New("invalid service name")
	// ErrServiceNotFound is returned for an unknown service.
	ErrServiceNotFound = errors.New("service not found")
	// ErrKeyNotFound is returned for a key the service does not have.
	ErrKeyNotFound = errors.New("api key not found")
	// ErrInvalidKey is returned for a key that is unknown or revoked.
	ErrInvalidKey = errors.New("invalid api key")

	_serviceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

type (
	// Controller manages service API keys.
	Controller interface {
		// CreateServiceAPIKey issues a new key for a service, creating the
		// service (disabled) when it is new. The service's other keys keep
		// working, so a key is rotated by creating a new one and revoking
		// the old one once every caller has switched.
		CreateServiceAPIKey(ctx context.Context, rq *entity.CreateServiceAPIKeyRequest) (*entity.CreateServiceAPIKeyResponse, error)
		// RevokeServiceAPIKey revokes one of a service's keys. Revoking a
		// revoked key succeeds.
		RevokeServiceAPIKey(ctx context.Context, rq *entity.RevokeServiceAPIKeyRequest) (*entity.RevokeServiceAPIKeyResponse, error)
		// AuthenticateService returns the service an active key belongs to.
		AuthenticateService(ctx context.Context, rq *entity.AuthenticateServiceRequest) (*entity.AuthenticateServiceResponse, error)
	}

	// Params are the controller's dependencies.
	Params struct {
		Services repository.ServiceRepository
		IDGen    idgen.Service
		// Rand is where keys come from; nil means crypto/rand.
		Rand io.Reader
		Now  func() time.Time
	}

	controller struct {
		services repository.ServiceRepository
		ids      idgen.Service
		rand     io.Reader
		now      func() time.Time
	}
)

// New returns the service key controller.
func New(p Params) Controller {
	c := &controller{services: p.Services, ids: p.IDGen, rand: p.Rand, now: p.Now}
	if c.rand == nil {
		c.rand = rand.Reader
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

func (c *controller) CreateServiceAPIKey(ctx context.Context, rq *entity.CreateServiceAPIKeyRequest) (*entity.CreateServiceAPIKeyResponse, error) {
	if !_serviceName.MatchString(rq.ServiceName) {
		return nil, ErrInvalidServiceName
	}
	raw := make([]byte, _keyBytes)
	if _, err := io.ReadFull(c.rand, raw); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	key := KeyPrefix + base64.RawURLEncoding.EncodeToString(raw)

	s, err := c.services.FindOrCreateService(ctx, model.Service{ID: c.ids.NextID(), Name: rq.ServiceName})
	if err != nil {
		return nil, fmt.Errorf("find or create service: %w", err)
	}
	k, err := c.services.CreateServiceAPIKey(ctx, model.ServiceAPIKey{
		ID:        c.ids.NextID(),
		CreatedAt: c.now().UTC(),
		ServiceID: s.ID,
		Prefix:    key[:_shownPrefixLen],
		KeyHash:   hashKey(key),
	})
	if err != nil {
		return nil, fmt.Errorf("create key: %w", err)
	}
	return &entity.CreateServiceAPIKeyResponse{
		Service: mappermodel.FromServiceModelToServiceEntity(s),
		APIKey:  mappermodel.FromServiceAPIKeyModelToServiceAPIKeyEntity(k),
		Key:     key,
	}, nil
}

func (c *controller) RevokeServiceAPIKey(ctx context.Context, rq *entity.RevokeServiceAPIKeyRequest) (*entity.RevokeServiceAPIKeyResponse, error) {
	s, err := c.services.FindServiceByName(ctx, rq.ServiceName)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find service: %w", err)
	}
	err = c.services.RevokeServiceAPIKey(ctx, s.ID, rq.KeyID, c.now().UTC())
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("revoke key: %w", err)
	}
	return &entity.RevokeServiceAPIKeyResponse{}, nil
}

func (c *controller) AuthenticateService(ctx context.Context, rq *entity.AuthenticateServiceRequest) (*entity.AuthenticateServiceResponse, error) {
	if !strings.HasPrefix(rq.Key, KeyPrefix) {
		return nil, ErrInvalidKey
	}
	s, err := c.services.FindServiceByActiveKeyHash(ctx, hashKey(rq.Key))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrInvalidKey
	}
	if err != nil {
		return nil, fmt.Errorf("find service by key: %w", err)
	}
	return &entity.AuthenticateServiceResponse{Service: mappermodel.FromServiceModelToServiceEntity(s)}, nil
}

// hashKey is the hex SHA-256 a key is stored and looked up by. Keys are
// long and random, so a plain hash is enough; no salt or slow hash needed.
func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
