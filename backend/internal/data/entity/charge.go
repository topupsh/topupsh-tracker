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

	// CreateRefundRequest asks to refund the charge of a request ID.
	CreateRefundRequest struct {
		ServiceID int64
		RequestID string
	}

	// CreateRefundResponse is the refunded charge and the user's balance.
	CreateRefundResponse struct {
		// Limited is false for an unknown request while the service's
		// switch is off.
		Limited bool
		Charge  Charge
		Balance int64
	}
)
