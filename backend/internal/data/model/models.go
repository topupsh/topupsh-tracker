// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package model

// Models are the models the database is migrated to.
func Models() []any {
	return []any{
		&User{}, &Service{}, &ServiceAPIKey{}, &ExternalIDMapping{}, &OperationMapping{}, &OrgLimit{},
		&CreditBucket{}, &ChargeSpend{}, &LedgerEntry{},
	}
}
