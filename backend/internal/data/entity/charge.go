// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package entity

import "time"

type (
	// CreateChargeRequest asks to spend a user's credits for an operation.
	CreateChargeRequest struct {
		ServiceID int64
		RequestID string
		OrgID     string
		UserID    string
		Credits   int64
		Operation string
	}

	// CreateChargeResponse is the charge's outcome.
	CreateChargeResponse struct {
		// Limited is false while the service's switch is off: the charge
		// succeeded without spending or recording anything.
		Limited bool
		Charge  Charge
		// RenewsAt is when the monthly limits renew.
		RenewsAt time.Time
	}

	// CreateRefundRequest asks to refund the charge of ChargeRequestID;
	// RequestID is the refund's own request.
	CreateRefundRequest struct {
		ServiceID       int64
		RequestID       string
		ChargeRequestID string
	}

	// CreateRefundResponse is the refund.
	CreateRefundResponse struct {
		// Limited is false for an unknown charge while the service's
		// switch is off.
		Limited bool
		Refund  Refund
	}
)
