// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"github.com/cespare/xxhash/v2"
	"github.com/mustafaturan/monoflake"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

// OrgMappingID is the mapping ID of a service's org: the xxhash64 of the
// service's base62 ID and the org, NUL-separated.
func OrgMappingID(serviceID int64, orgID string) int64 {
	return mappingID(serviceID, orgID)
}

// UserMappingID is the mapping ID of a user of a service's org: the
// xxhash64 of the service's base62 ID, the org and the user, NUL-separated.
func UserMappingID(serviceID int64, orgID, userID string) int64 {
	return mappingID(serviceID, orgID, userID)
}

// OperationMappingID is the mapping ID of a service's operation: the
// xxhash64 of the service's base62 ID and the operation, NUL-separated.
func OperationMappingID(serviceID int64, operation string) int64 {
	return mappingID(serviceID, operation)
}

func mappingID(serviceID int64, parts ...string) int64 {
	d := xxhash.New()
	_, _ = d.WriteString(monoflake.ID(serviceID).String())
	for _, p := range parts {
		_, _ = d.WriteString("\x00")
		_, _ = d.WriteString(p)
	}
	return int64(d.Sum64()) //nolint:gosec // the hash's bits, sign included
}

// mapOrg stores the mapping of a service's org unless it is stored, and
// returns its mapping ID.
func mapOrg(tx *gorm.DB, nextID func() int64, serviceID int64, orgID string) (int64, error) {
	return mapIDs(tx, nextID, model.ExternalIDMapping{ServiceID: serviceID, OrgID: orgID, MappingID: OrgMappingID(serviceID, orgID)})
}

// mapUser stores the mappings of a user and its org unless they are
// stored, and returns their mapping IDs.
func mapUser(tx *gorm.DB, nextID func() int64, serviceID int64, orgID, userID string) (int64, int64, error) {
	org, err := mapOrg(tx, nextID, serviceID, orgID)
	if err != nil {
		return 0, 0, err
	}
	user, err := mapIDs(tx, nextID, model.ExternalIDMapping{
		ServiceID: serviceID, OrgID: orgID, UserID: userID, MappingID: UserMappingID(serviceID, orgID, userID),
	})
	return org, user, err
}

// mapIDs stores m unless its service, org and user, or its service and
// mapping ID, are stored, and returns its mapping ID.
func mapIDs(tx *gorm.DB, nextID func() int64, m model.ExternalIDMapping) (int64, error) {
	m.ID = nextID()
	return m.MappingID, tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
}

// mapOperation stores the mapping of a service's operation unless its
// mapping ID is stored, and returns the mapping ID; 0 for no operation.
// An operation that hashes like a stored one shares its mapping.
func mapOperation(tx *gorm.DB, nextID func() int64, serviceID int64, operation string) (int64, error) {
	if operation == "" {
		return 0, nil
	}
	m := model.OperationMapping{ID: nextID(), ServiceID: serviceID, Operation: operation, MappingID: OperationMappingID(serviceID, operation)}
	return m.MappingID, tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
}

// operationName returns the operation a service's mapping ID maps; "" for
// none.
func operationName(q *gorm.DB, serviceID, mappingID int64) (string, error) {
	var ops []string
	err := q.Model(&model.OperationMapping{}).Where("service_id = ? AND mapping_id = ?", serviceID, mappingID).
		Limit(1).Pluck("operation", &ops).Error
	if err != nil || len(ops) == 0 {
		return "", err
	}
	return ops[0], nil
}

// externalIDs returns the mappings of a service's mapping IDs, by mapping
// ID.
func externalIDs(q *gorm.DB, serviceID int64, ids []int64) (map[int64]model.ExternalIDMapping, error) {
	byID := make(map[int64]model.ExternalIDMapping, len(ids))
	if len(ids) == 0 {
		return byID, nil
	}
	var ms []model.ExternalIDMapping
	if err := q.Where("service_id = ? AND mapping_id IN ?", serviceID, ids).Find(&ms).Error; err != nil {
		return nil, err
	}
	for _, m := range ms {
		byID[m.MappingID] = m
	}
	return byID, nil
}
