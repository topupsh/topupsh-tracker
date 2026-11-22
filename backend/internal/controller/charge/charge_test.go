// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package charge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/entity"
	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
	"github.com/topupsh/topupsh-tracker/backend/internal/repository"
	repomocks "github.com/topupsh/topupsh-tracker/backend/internal/repository/mocks"
	idgenmocks "github.com/topupsh/topupsh-tracker/backend/internal/service/idgen/mocks"
)

var (
	_now    = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_renews = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

func newController(t *testing.T) (Controller, *repomocks.MockChargeRepository, *idgenmocks.MockService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	charges := repomocks.NewMockChargeRepository(ctrl)
	ids := idgenmocks.NewMockService(ctrl)
	ids.EXPECT().NextID().Return(int64(42)).AnyTimes()
	return New(Params{Charges: charges, IDGen: ids, Now: func() time.Time { return _now }}), charges, ids
}

func validCharge() *entity.CreateChargeRequest {
	return &entity.CreateChargeRequest{ServiceID: 7, RequestID: "r1", OrgID: "o1", UserID: "u1", Credits: 2, Operation: "tool"}
}

func TestNew_Defaults(t *testing.T) {
	c := New(Params{}).(*controller)
	assert.WithinDuration(t, time.Now(), c.now(), time.Second)
}

func TestCreateCharge(t *testing.T) {
	ctx := context.Background()
	t.Run("charged", func(t *testing.T) {
		c, charges, _ := newController(t)
		r1 := "r1"
		charges.EXPECT().Charge(ctx, model.LedgerEntry{
			CreatedAt: _now, ServiceID: 7, RequestID: &r1, OrgID: "o1", UserID: "u1", Amount: -2, Operation: "tool",
		}, gomock.Any()).DoAndReturn(func(_ context.Context, e model.LedgerEntry, next func() int64) (repository.ChargeResult, error) {
			assert.Equal(t, int64(42), next(), "the repository gets the ID generator")
			e.EntryType, e.BalanceAfter, e.OrgRemaining = model.EntryTypeSpend, 8, 90
			return repository.ChargeResult{Limited: true, Entry: e}, nil
		})
		rs, err := c.CreateCharge(ctx, validCharge())
		require.NoError(t, err)
		assert.True(t, rs.Limited)
		assert.Equal(t, int64(2), rs.Charge.Credits)
		assert.Equal(t, "r1", rs.Charge.RequestID)
		assert.Equal(t, int64(8), rs.Charge.Balance)
		assert.Equal(t, int64(90), rs.Charge.OrgRemaining)
		assert.Equal(t, _renews, rs.RenewsAt)
	})
	t.Run("replayed refusal renews after the original month", func(t *testing.T) {
		c, charges, _ := newController(t)
		charges.EXPECT().Charge(ctx, gomock.Any(), gomock.Any()).Return(repository.ChargeResult{Limited: true, Entry: model.LedgerEntry{
			EntryType: model.EntryTypeRefusal, RefusalReason: model.RefusalReasonUserLimit, CreatedAt: time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC),
		}}, nil)
		rs, err := c.CreateCharge(ctx, validCharge())
		require.NoError(t, err)
		assert.Equal(t, entity.RefusalUserLimit, rs.Charge.Refusal)
		assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), rs.RenewsAt)
	})
	t.Run("switched off", func(t *testing.T) {
		c, charges, _ := newController(t)
		charges.EXPECT().Charge(ctx, gomock.Any(), gomock.Any()).Return(repository.ChargeResult{}, nil)
		rs, err := c.CreateCharge(ctx, validCharge())
		require.NoError(t, err)
		assert.False(t, rs.Limited)
		assert.Equal(t, entity.Charge{ServiceID: 7, RequestID: "r1", OrgID: "o1", UserID: "u1", Credits: 2, Operation: "tool"}, rs.Charge)
	})
	t.Run("invalid", func(t *testing.T) {
		c, _, _ := newController(t)
		for want, edit := range map[error]func(rq *entity.CreateChargeRequest){
			ErrInvalidRequestID: func(rq *entity.CreateChargeRequest) { rq.RequestID = strings.Repeat("r", 33) },
			ErrInvalidOrg:       func(rq *entity.CreateChargeRequest) { rq.OrgID = "" },
			ErrInvalidUser:      func(rq *entity.CreateChargeRequest) { rq.UserID = strings.Repeat("u", 65) },
			ErrInvalidCredits:   func(rq *entity.CreateChargeRequest) { rq.Credits = 0 },
			ErrInvalidOperation: func(rq *entity.CreateChargeRequest) { rq.Operation = strings.Repeat("o", 256) },
		} {
			rq := validCharge()
			edit(rq)
			_, err := c.CreateCharge(ctx, rq)
			assert.ErrorIs(t, err, want)
		}
	})
	t.Run("errors", func(t *testing.T) {
		for repoErr, want := range map[error]error{
			repository.ErrRequestConflict: ErrRequestConflict,
			repository.ErrNotFound:        ErrServiceNotFound,
			errBoom:                       errBoom,
		} {
			c, charges, _ := newController(t)
			charges.EXPECT().Charge(ctx, gomock.Any(), gomock.Any()).Return(repository.ChargeResult{}, repoErr)
			_, err := c.CreateCharge(ctx, validCharge())
			assert.ErrorIs(t, err, want)
		}
	})
}

func TestCreateRefund(t *testing.T) {
	ctx := context.Background()
	rq := &entity.CreateRefundRequest{ServiceID: 7, RequestID: "f1", ChargeRequestID: "r1"}
	t.Run("refunded", func(t *testing.T) {
		c, charges, _ := newController(t)
		f1 := "f1"
		charges.EXPECT().Refund(ctx, int64(7), "f1", "r1", _now, gomock.Any()).
			Return(repository.RefundResult{Limited: true, Refund: model.LedgerEntry{RequestID: &f1, EntryType: model.EntryTypeRefund, Amount: 2, BalanceAfter: 9, CreatedAt: _now}}, nil)
		rs, err := c.CreateRefund(ctx, rq)
		require.NoError(t, err)
		assert.True(t, rs.Limited)
		assert.Equal(t, entity.Refund{RequestID: "f1", ChargeRequestID: "r1", Credits: 2, Balance: 9, CreatedAt: _now}, rs.Refund)
	})
	t.Run("switched off", func(t *testing.T) {
		c, charges, _ := newController(t)
		charges.EXPECT().Refund(ctx, int64(7), "f1", "r1", _now, gomock.Any()).Return(repository.RefundResult{}, nil)
		rs, err := c.CreateRefund(ctx, rq)
		require.NoError(t, err)
		assert.False(t, rs.Limited)
		assert.Equal(t, entity.Refund{ServiceID: 7, RequestID: "f1", ChargeRequestID: "r1"}, rs.Refund)
	})
	t.Run("invalid", func(t *testing.T) {
		c, _, _ := newController(t)
		_, err := c.CreateRefund(ctx, &entity.CreateRefundRequest{ServiceID: 7, ChargeRequestID: "r1"})
		assert.ErrorIs(t, err, ErrInvalidRequestID)
		_, err = c.CreateRefund(ctx, &entity.CreateRefundRequest{ServiceID: 7, RequestID: "f1", ChargeRequestID: strings.Repeat("r", 33)})
		assert.ErrorIs(t, err, ErrInvalidChargeRequestID)
	})
	t.Run("errors", func(t *testing.T) {
		for repoErr, want := range map[error]error{
			repository.ErrNotFound:        ErrChargeNotFound,
			repository.ErrNotCharged:      ErrNotCharged,
			repository.ErrRequestConflict: ErrRequestConflict,
			errBoom:                       errBoom,
		} {
			c, charges, _ := newController(t)
			charges.EXPECT().Refund(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(repository.RefundResult{}, repoErr)
			_, err := c.CreateRefund(ctx, rq)
			assert.ErrorIs(t, err, want)
		}
	})
}
