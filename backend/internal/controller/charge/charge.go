// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package charge spends and refunds credits for the services.
package charge

//go:generate mockgen -source=charge.go -destination=mocks/charge.go -package=mocks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	mappermodel "github.com/topupsh/topupsh-tracker/backend/internal/mapper/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/idgen"
)

// Field length limits, matching the columns.
const (
	_maxSubjectLen   = 64
	_maxRequestIDLen = 32
	_maxOperationLen = 128
)

var (
	// ErrInvalidRequestID is returned for a request ID that is empty or
	// longer than 32 characters.
	ErrInvalidRequestID = errors.New("invalid request id")
	// ErrInvalidChargeRequestID is returned for a refund's charge request
	// ID that is empty or longer than 32 characters.
	ErrInvalidChargeRequestID = errors.New("invalid charge request id")
	// ErrInvalidOrg is returned for an org ID that is empty or longer
	// than 64 characters.
	ErrInvalidOrg = errors.New("invalid org")
	// ErrInvalidUser is returned for a user ID that is empty or longer
	// than 64 characters.
	ErrInvalidUser = errors.New("invalid user")
	// ErrInvalidCredits is returned for a charge of less than 1 credit.
	ErrInvalidCredits = errors.New("credits must be positive")
	// ErrInvalidOperation is returned for an operation longer than 128
	// characters.
	ErrInvalidOperation = errors.New("operation is too long")
	// ErrRequestConflict is returned when a request ID is reused for
	// another request: a charge of a different org, user or number of
	// credits, or a refund of another charge.
	ErrRequestConflict = errors.New("request id was used for another request")
	// ErrChargeNotFound is returned when refunding an unknown request.
	ErrChargeNotFound = errors.New("charge not found")
	// ErrNotCharged is returned when refunding a refused charge.
	ErrNotCharged = errors.New("charge was refused")
	// ErrServiceNotFound is returned when the calling service is gone.
	ErrServiceNotFound = errors.New("service not found")
)

type (
	// Controller spends and refunds credits.
	Controller interface {
		// CreateCharge spends a user's credits, oldest bucket first, if
		// the org limit and then the user's credits allow it. A refused
		// charge spends nothing. A retry with the same request ID returns
		// the original outcome. While the service's switch is off it
		// succeeds without spending or recording anything.
		CreateCharge(ctx context.Context, rq *entity.CreateChargeRequest) (*entity.CreateChargeResponse, error)
		// CreateRefund gives a charge back to the buckets it came from,
		// once; a retry with the same request ID, or another refund of the
		// charge, returns the same refund.
		CreateRefund(ctx context.Context, rq *entity.CreateRefundRequest) (*entity.CreateRefundResponse, error)
	}

	// Params are the controller's dependencies.
	Params struct {
		Charges repository.ChargeRepository
		IDGen   idgen.Service
		Now     func() time.Time
	}

	controller struct {
		charges repository.ChargeRepository
		ids     idgen.Service
		now     func() time.Time
	}
)

// New returns the charge controller.
func New(p Params) Controller {
	c := &controller{charges: p.Charges, ids: p.IDGen, now: p.Now}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

func (c *controller) CreateCharge(ctx context.Context, rq *entity.CreateChargeRequest) (*entity.CreateChargeResponse, error) {
	switch {
	case !validLen(rq.RequestID, _maxRequestIDLen):
		return nil, ErrInvalidRequestID
	case !validLen(rq.OrgID, _maxSubjectLen):
		return nil, ErrInvalidOrg
	case !validLen(rq.UserID, _maxSubjectLen):
		return nil, ErrInvalidUser
	case rq.Credits <= 0:
		return nil, ErrInvalidCredits
	case len(rq.Operation) > _maxOperationLen:
		return nil, ErrInvalidOperation
	}
	rs, err := c.charges.Charge(ctx, model.LedgerEntry{
		CreatedAt: c.now().UTC(),
		ServiceID: rq.ServiceID,
		RequestID: &rq.RequestID,
		OrgID:     rq.OrgID,
		UserID:    rq.UserID,
		Amount:    -rq.Credits,
		Operation: rq.Operation,
	}, c.ids.NextID)
	if err != nil {
		return nil, mapError(err, "charge")
	}
	if !rs.Limited {
		return &entity.CreateChargeResponse{Charge: entity.Charge{
			ServiceID: rq.ServiceID, RequestID: rq.RequestID, OrgID: rq.OrgID,
			UserID: rq.UserID, Credits: rq.Credits, Operation: rq.Operation,
		}}, nil
	}
	ch := mappermodel.FromLedgerEntryModelToChargeEntity(rs.Entry)
	return &entity.CreateChargeResponse{
		Limited:  true,
		Charge:   ch,
		RenewsAt: repository.MonthStart(ch.CreatedAt).AddDate(0, 1, 0),
	}, nil
}

func (c *controller) CreateRefund(ctx context.Context, rq *entity.CreateRefundRequest) (*entity.CreateRefundResponse, error) {
	switch {
	case !validLen(rq.RequestID, _maxRequestIDLen):
		return nil, ErrInvalidRequestID
	case !validLen(rq.ChargeRequestID, _maxRequestIDLen):
		return nil, ErrInvalidChargeRequestID
	}
	rs, err := c.charges.Refund(ctx, rq.ServiceID, rq.RequestID, rq.ChargeRequestID, c.now().UTC(), c.ids.NextID)
	if err != nil {
		return nil, mapError(err, "refund")
	}
	if !rs.Limited {
		return &entity.CreateRefundResponse{Refund: entity.Refund{
			ServiceID: rq.ServiceID, RequestID: rq.RequestID, ChargeRequestID: rq.ChargeRequestID,
		}}, nil
	}
	return &entity.CreateRefundResponse{
		Limited: true,
		Refund:  mappermodel.FromLedgerEntryModelToRefundEntity(rs.Refund, rq.ChargeRequestID),
	}, nil
}

// mapError turns repository errors into the controller's; anything else is
// wrapped with op.
func mapError(err error, op string) error {
	switch {
	case errors.Is(err, repository.ErrRequestConflict):
		return ErrRequestConflict
	case errors.Is(err, repository.ErrNotCharged):
		return ErrNotCharged
	case errors.Is(err, repository.ErrNotFound) && op == "refund":
		return ErrChargeNotFound
	case errors.Is(err, repository.ErrNotFound):
		return ErrServiceNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func validLen(s string, maxLen int) bool {
	return s != "" && len(s) <= maxLen
}
