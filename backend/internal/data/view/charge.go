// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package view

type (
	// CreateChargeRequest is the body of POST /api/v1/charges.
	CreateChargeRequest struct {
		RequestID string `json:"requestId"`
		Org       string `json:"org"`
		User      string `json:"user"`
		Credits   int64  `json:"credits"`
		Operation string `json:"operation"`
	}

	// Charge is a charge's outcome.
	Charge struct {
		RequestID string `json:"requestId"`
		Charged   bool   `json:"charged"`
		// Limited is false while the service's credit limiting is off.
		Limited bool `json:"limited"`
		// Reason is why the charge was refused: "org_limit" or
		// "user_limit".
		Reason string `json:"reason,omitempty"`
		// RenewsAt is when a refused charge's limits renew.
		RenewsAt string `json:"renewsAt,omitempty"`
		// Balance is the user's unexpired credits after the charge.
		Balance *int64 `json:"balance,omitempty"`
		// OrgRemaining is what is left of the org limit this month.
		OrgRemaining *int64 `json:"orgRemaining,omitempty"`
	}

	// CreateRefundRequest is the body of POST /api/v1/refunds.
	CreateRefundRequest struct {
		RequestID string `json:"requestId"`
	}

	// Refund is a refunded charge.
	Refund struct {
		RequestID string `json:"requestId"`
		Limited   bool   `json:"limited"`
		// Credits is what was given back; credits of expired buckets are
		// not.
		Credits    int64  `json:"credits"`
		RefundedAt string `json:"refundedAt,omitempty"`
		Balance    *int64 `json:"balance,omitempty"`
	}
)
