// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package repository is the data access layer.
package repository

//go:generate mockgen -source=user.go -destination=mocks/user.go -package=mocks

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

type (
	// UserRepository persists users.
	UserRepository interface {
		FindUserByID(ctx context.Context, id int64) (model.User, error)
		FindUserByEmail(ctx context.Context, email string) (model.User, error)
		CreateUser(ctx context.Context, u model.User) (model.User, error)
		UpdateUser(ctx context.Context, u model.User) (model.User, error)
	}

	userRepository struct {
		db *gorm.DB
	}
)

// NewUserRepository returns a GORM-backed user repository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) FindUserByID(ctx context.Context, id int64) (model.User, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *userRepository) FindUserByEmail(ctx context.Context, email string) (model.User, error) {
	return r.first(ctx, "email = ?", email)
}

func (r *userRepository) CreateUser(ctx context.Context, u model.User) (model.User, error) {
	if err := r.db.WithContext(ctx).Create(&u).Error; err != nil {
		return model.User{}, err
	}
	return u, nil
}

func (r *userRepository) UpdateUser(ctx context.Context, u model.User) (model.User, error) {
	if err := r.db.WithContext(ctx).Save(&u).Error; err != nil {
		return model.User{}, err
	}
	return u, nil
}

func (r *userRepository) first(ctx context.Context, query string, arg any) (model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where(query, arg).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, err
	}
	return u, nil
}
