// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Package balance reads balances and the ledger: a service reads its own
// users', the root admin every service's, and a user their own through a
// view token the service issued.
package balance

//go:generate mockgen -source=balance.go -destination=mocks/balance.go -package=mocks

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	mappermodel "github.com/topupsh/topupsh-tracker/backend/internal/mapper/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	"github.com/topupsh/topupsh-tracker/backend/internal/service/token"
)

// Ledger page sizes.
const (
	DefaultLedgerLimit = 50
	MaxLedgerLimit     = 200
)

// _maxSubjectLen is the longest org or user ID a service may use.
const _maxSubjectLen = 64

var (
	// ErrForbidden is returned when the reader may not read what it
	// asked for, e.g. a service naming another service.
	ErrForbidden = errors.New("forbidden")
	// ErrViewerFilter is returned when a viewer names a service, org or
	// user: a view token reads only its own user's credits.
	ErrViewerFilter = errors.New("a view token reads only its own credits")
	// ErrServiceRequired is returned when the root admin reads a balance
	// without naming the service.
	ErrServiceRequired = errors.New("service is required")
	// ErrServiceNotFound is returned for an unknown service.
	ErrServiceNotFound = errors.New("service not found")
	// ErrInvalidOrg is returned for an org ID that is missing where it is
	// required, or longer than 64 characters.
	ErrInvalidOrg = errors.New("invalid org")
	// ErrInvalidUser is returned for a user ID that is missing where it is
	// required, or longer than 64 characters.
	ErrInvalidUser = errors.New("invalid user")
	// ErrInvalidLimit is returned for a ledger page size outside
	// 1..MaxLedgerLimit.
	ErrInvalidLimit = errors.New("invalid limit")
	// ErrInvalidToken is returned for a view token that is malformed,
	// expired or signed by someone else.
	ErrInvalidToken = errors.New("invalid view token")
)

type (
	// Controller reads balances and the ledger and issues view tokens.
	Controller interface {
		// GetBalance returns one user's balance at the current time.
		GetBalance(ctx context.Context, rq *entity.GetBalanceRequest) (*entity.GetBalanceResponse, error)
		// ListLedgerEntries returns a page of the ledger, newest first.
		ListLedgerEntries(ctx context.Context, rq *entity.ListLedgerEntriesRequest) (*entity.ListLedgerEntriesResponse, error)
		// CreateViewToken issues a token that lets one user of the calling
		// service read their own balance and ledger for token.ViewTTL.
		CreateViewToken(ctx context.Context, rq *entity.CreateViewTokenRequest) (*entity.CreateViewTokenResponse, error)
		// AuthenticateViewer returns the user a view token was issued for.
		AuthenticateViewer(ctx context.Context, rq *entity.AuthenticateViewerRequest) (*entity.AuthenticateViewerResponse, error)
	}

	// Params are the controller's dependencies.
	Params struct {
		Balances repository.BalanceRepository
		Services repository.ServiceRepository
		Token    token.Service
		// PageURL is the user balance page, e.g.
		// https://topup.sh/balance; a view token's URL is PageURL with
		// the token in the fragment. Empty leaves the URL out.
		PageURL string
		Now     func() time.Time
	}

	controller struct {
		balances repository.BalanceRepository
		services repository.ServiceRepository
		tokens   token.Service
		pageURL  string
		now      func() time.Time
	}

	// scope is what a read covers; a zero serviceID means every service.
	scope struct {
		serviceID int64
		orgID     string
		userID    string
	}
)

// New returns the balance controller.
func New(p Params) Controller {
	c := &controller{balances: p.Balances, services: p.Services, tokens: p.Token, pageURL: p.PageURL, now: p.Now}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

func (c *controller) GetBalance(ctx context.Context, rq *entity.GetBalanceRequest) (*entity.GetBalanceResponse, error) {
	sc, err := c.scope(ctx, rq.Reader, rq.ServiceName, rq.OrgID, rq.UserID, true)
	if err != nil {
		return nil, err
	}
	if !validSubject(sc.orgID) {
		return nil, ErrInvalidOrg
	}
	if !validSubject(sc.userID) {
		return nil, ErrInvalidUser
	}
	now := c.now().UTC()
	b, err := c.balances.GetBalance(ctx, sc.serviceID, sc.orgID, sc.userID, now)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	rs := entity.Balance{
		Service:  mappermodel.FromServiceModelToServiceEntity(b.Service),
		OrgID:    sc.orgID,
		UserID:   sc.userID,
		Grants:   []entity.CreditBucket{},
		RenewsAt: repository.MonthStart(now).AddDate(0, 1, 0),
		Org: entity.OrgCredits{
			Limit:     b.OrgLimit.MonthlyLimit,
			UserLimit: b.OrgLimit.UserLimit,
			Used:      b.OrgUsed,
			Remaining: max(b.OrgLimit.MonthlyLimit-b.OrgUsed, 0),
		},
	}
	// Until the user's first charge of the month creates the monthly
	// bucket, nothing of the allowance is used. A charge resizes the
	// bucket to the current limits, never below what was used.
	allowance := repository.MonthlyAllowance(b.OrgLimit)
	var grants int64
	for _, bk := range b.Buckets {
		rs.Credits += bk.Remaining
		if bk.BucketType == model.BucketTypeMonthly {
			rs.Monthly.Used = bk.Amount - bk.Remaining
			continue
		}
		grants += bk.Remaining
		rs.Grants = append(rs.Grants, mappermodel.FromCreditBucketModelToCreditBucketEntity(bk))
	}
	rs.Monthly.Limit = max(allowance, rs.Monthly.Used)
	rs.Monthly.Remaining = rs.Monthly.Limit - rs.Monthly.Used
	rs.Available = grants + min(rs.Monthly.Remaining, rs.Org.Remaining)
	return &entity.GetBalanceResponse{Balance: rs}, nil
}

func (c *controller) ListLedgerEntries(ctx context.Context, rq *entity.ListLedgerEntriesRequest) (*entity.ListLedgerEntriesResponse, error) {
	sc, err := c.scope(ctx, rq.Reader, rq.ServiceName, rq.OrgID, rq.UserID, false)
	if err != nil {
		return nil, err
	}
	switch {
	case len(sc.orgID) > _maxSubjectLen:
		return nil, ErrInvalidOrg
	case len(sc.userID) > _maxSubjectLen, sc.userID != "" && sc.orgID == "":
		// User IDs are only unique within an org.
		return nil, ErrInvalidUser
	case rq.Limit < 0 || rq.Limit > MaxLedgerLimit:
		return nil, ErrInvalidLimit
	}
	limit := rq.Limit
	if limit == 0 {
		limit = DefaultLedgerLimit
	}
	// One more row than the page tells whether another page follows.
	rows, err := c.balances.ListLedgerEntries(ctx, repository.LedgerFilter{
		ServiceID: sc.serviceID, OrgID: sc.orgID, UserID: sc.userID, BeforeID: rq.Cursor, Limit: limit + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	rs := &entity.ListLedgerEntriesResponse{Entries: make([]entity.LedgerEntry, 0, min(len(rows), limit))}
	if len(rows) > limit {
		rows = rows[:limit]
		rs.NextCursor = rows[limit-1].ID
	}
	for _, r := range rows {
		e := mappermodel.FromLedgerEntryModelToLedgerEntryEntity(r.LedgerEntry)
		e.ServiceName = r.ServiceName
		rs.Entries = append(rs.Entries, e)
	}
	return rs, nil
}

func (c *controller) CreateViewToken(_ context.Context, rq *entity.CreateViewTokenRequest) (*entity.CreateViewTokenResponse, error) {
	if !validSubject(rq.OrgID) {
		return nil, ErrInvalidOrg
	}
	if !validSubject(rq.UserID) {
		return nil, ErrInvalidUser
	}
	tok, exp, err := c.tokens.CreateViewToken(token.Viewer{ServiceID: rq.ServiceID, OrgID: rq.OrgID, UserID: rq.UserID})
	if err != nil {
		return nil, fmt.Errorf("create view token: %w", err)
	}
	rs := &entity.CreateViewTokenResponse{Token: tok, ExpiresAt: exp}
	if c.pageURL != "" {
		// The fragment keeps the token out of server and proxy logs.
		rs.URL = c.pageURL + "#token=" + url.QueryEscape(tok)
	}
	return rs, nil
}

func (c *controller) AuthenticateViewer(_ context.Context, rq *entity.AuthenticateViewerRequest) (*entity.AuthenticateViewerResponse, error) {
	v, err := c.tokens.ValidateViewToken(rq.Token)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &entity.AuthenticateViewerResponse{Viewer: entity.Viewer{ServiceID: v.ServiceID, OrgID: v.OrgID, UserID: v.UserID}}, nil
}

// scope returns what the reader's request covers. A viewer reads its own
// user and names nothing; a service reads only its own service; the root
// admin reads any service, and must name one when needService is set.
func (c *controller) scope(ctx context.Context, rd entity.Reader, serviceName, orgID, userID string, needService bool) (scope, error) {
	switch {
	case rd.Viewer != nil:
		if serviceName != "" || orgID != "" || userID != "" {
			return scope{}, ErrViewerFilter
		}
		return scope{serviceID: rd.Viewer.ServiceID, orgID: rd.Viewer.OrgID, userID: rd.Viewer.UserID}, nil
	case rd.ServiceID != 0:
		if serviceName != "" {
			s, err := c.findService(ctx, serviceName)
			if errors.Is(err, ErrServiceNotFound) || (err == nil && s.ID != rd.ServiceID) {
				return scope{}, ErrForbidden
			}
			if err != nil {
				return scope{}, err
			}
		}
		return scope{serviceID: rd.ServiceID, orgID: orgID, userID: userID}, nil
	case rd.Root:
		if serviceName == "" {
			if needService {
				return scope{}, ErrServiceRequired
			}
			return scope{orgID: orgID, userID: userID}, nil
		}
		s, err := c.findService(ctx, serviceName)
		if err != nil {
			return scope{}, err
		}
		return scope{serviceID: s.ID, orgID: orgID, userID: userID}, nil
	}
	return scope{}, ErrForbidden
}

func (c *controller) findService(ctx context.Context, name string) (entity.Service, error) {
	s, err := c.services.FindServiceByName(ctx, name)
	if errors.Is(err, repository.ErrNotFound) {
		return entity.Service{}, ErrServiceNotFound
	}
	if err != nil {
		return entity.Service{}, fmt.Errorf("find service: %w", err)
	}
	return mappermodel.FromServiceModelToServiceEntity(s), nil
}

func validSubject(s string) bool {
	return s != "" && len(s) <= _maxSubjectLen
}
