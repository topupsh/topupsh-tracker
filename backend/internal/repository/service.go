// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

//go:generate mockgen -source=service.go -destination=mocks/service.go -package=mocks

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

type (
	// ServiceRepository persists services and their API keys.
	ServiceRepository interface {
		// FindServiceByName returns the service named name.
		FindServiceByName(ctx context.Context, name string) (model.Service, error)
		// FindOrCreateService returns the service named s.Name, creating s
		// when there is none.
		FindOrCreateService(ctx context.Context, s model.Service) (model.Service, error)
		// SetServiceEnabled switches credit limiting on or off for the
		// service named name. It returns ErrNotFound for an unknown service.
		SetServiceEnabled(ctx context.Context, name string, enabled bool) (model.Service, error)
		// CreateServiceAPIKey stores a new key.
		CreateServiceAPIKey(ctx context.Context, k model.ServiceAPIKey) (model.ServiceAPIKey, error)
		// RevokeServiceAPIKey revokes the key id of the service serviceID
		// at at. Revoking a revoked key keeps its first revocation time.
		// It returns ErrNotFound when the service has no such key.
		RevokeServiceAPIKey(ctx context.Context, serviceID, id int64, at time.Time) error
		// FindServiceByActiveKeyHash returns the service with an unrevoked
		// key whose hash is hash.
		FindServiceByActiveKeyHash(ctx context.Context, hash string) (model.Service, error)
	}

	serviceRepository struct {
		db *gorm.DB
	}
)

// NewServiceRepository returns a GORM-backed service repository.
func NewServiceRepository(db *gorm.DB) ServiceRepository {
	return &serviceRepository{db: db}
}

func (r *serviceRepository) FindServiceByName(ctx context.Context, name string) (model.Service, error) {
	return r.firstService(r.db.WithContext(ctx).Where("name = ?", name))
}

func (r *serviceRepository) FindOrCreateService(ctx context.Context, s model.Service) (model.Service, error) {
	// Doing nothing on a name conflict lets two admins create the same
	// service at once; both then read the one that won.
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).Create(&s).Error
	if err != nil {
		return model.Service{}, err
	}
	return r.FindServiceByName(ctx, s.Name)
}

func (r *serviceRepository) SetServiceEnabled(ctx context.Context, name string, enabled bool) (model.Service, error) {
	res := r.db.WithContext(ctx).Model(&model.Service{}).Where("name = ?", name).Update("enabled", enabled)
	if res.Error != nil {
		return model.Service{}, res.Error
	}
	return r.FindServiceByName(ctx, name)
}

func (r *serviceRepository) CreateServiceAPIKey(ctx context.Context, k model.ServiceAPIKey) (model.ServiceAPIKey, error) {
	if err := r.db.WithContext(ctx).Create(&k).Error; err != nil {
		return model.ServiceAPIKey{}, err
	}
	return k, nil
}

func (r *serviceRepository) RevokeServiceAPIKey(ctx context.Context, serviceID, id int64, at time.Time) error {
	db := r.db.WithContext(ctx)
	var k model.ServiceAPIKey
	err := db.Where("id = ? AND service_id = ?", id, serviceID).First(&k).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if k.RevokedAt != nil {
		return nil
	}
	return db.Model(&k).Where("revoked_at IS NULL").Update("revoked_at", at).Error
}

func (r *serviceRepository) FindServiceByActiveKeyHash(ctx context.Context, hash string) (model.Service, error) {
	return r.firstService(r.db.WithContext(ctx).
		Joins("JOIN service_api_keys ON service_api_keys.service_id = services.id").
		Where("service_api_keys.key_hash = ? AND service_api_keys.revoked_at IS NULL", hash))
}

func (r *serviceRepository) firstService(q *gorm.DB) (model.Service, error) {
	var s model.Service
	err := q.First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Service{}, ErrNotFound
	}
	if err != nil {
		return model.Service{}, err
	}
	return s, nil
}
