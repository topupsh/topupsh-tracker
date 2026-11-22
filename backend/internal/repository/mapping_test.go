// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/topupsh/topupsh-tracker/backend/internal/data/model"
)

func TestMappingIDs(t *testing.T) {
	assert.Equal(t, OrgMappingID(1, "o1"), OrgMappingID(1, "o1"), "the same IDs map the same")
	assert.Equal(t, UserMappingID(1, "o1", "u1"), UserMappingID(1, "o1", "u1"))
	assert.NotEqual(t, OrgMappingID(1, "o1"), OrgMappingID(2, "o1"), "each service has its own")
	assert.NotEqual(t, UserMappingID(1, "ab", "c"), UserMappingID(1, "a", "bc"), "the parts are separated")
	assert.NotEqual(t, OrgMappingID(1, "o1"), UserMappingID(1, "o1", ""), "an org is not its user with no ID")
}

func TestMapOperation(t *testing.T) {
	db, _ := newCreditDB(t)
	next := ids()
	none, err := mapOperation(db, next, 1, "")
	require.NoError(t, err)
	assert.Zero(t, none, "no operation, no mapping")

	id, err := mapOperation(db, next, 1, "tool")
	require.NoError(t, err)
	assert.Equal(t, OperationMappingID(1, "tool"), id)
	again, err := mapOperation(db, next, 1, "tool")
	require.NoError(t, err)
	assert.Equal(t, id, again)
	assert.NotEqual(t, id, OperationMappingID(2, "tool"), "each service has its own")

	var n int64
	require.NoError(t, db.Model(&model.OperationMapping{}).Count(&n).Error)
	assert.Equal(t, int64(1), n, "stored once")

	op, err := operationName(db, 1, id)
	require.NoError(t, err)
	assert.Equal(t, "tool", op)
	op, err = operationName(db, 2, id)
	require.NoError(t, err)
	assert.Empty(t, op, "another service's mapping")
}

func TestMapUser(t *testing.T) {
	db, _ := newCreditDB(t)
	next := ids()
	org, user, err := mapUser(db, next, 1, "o1", "u1")
	require.NoError(t, err)
	assert.Equal(t, OrgMappingID(1, "o1"), org)
	assert.Equal(t, UserMappingID(1, "o1", "u1"), user)

	again, _, err := mapUser(db, next, 1, "o1", "u2")
	require.NoError(t, err)
	assert.Equal(t, org, again, "the org is stored once")

	var ms []model.ExternalIDMapping
	require.NoError(t, db.Order("id").Find(&ms).Error)
	require.Len(t, ms, 3)
	assert.Equal(t, model.ExternalIDMapping{ID: 1001, CreatedAt: ms[0].CreatedAt, ServiceID: 1, OrgID: "o1", MappingID: org}, ms[0])
	assert.Equal(t, [4]string{"o1", "u1", "o1", "u2"}, [4]string{ms[1].OrgID, ms[1].UserID, ms[2].OrgID, ms[2].UserID})

	byID, err := externalIDs(db, 1, []int64{user, org, 42})
	require.NoError(t, err)
	assert.Len(t, byID, 2, "unknown mapping IDs are left out")
	other, err := externalIDs(db, 2, []int64{user, org})
	require.NoError(t, err)
	assert.Empty(t, other, "another service's mappings")
	assert.Equal(t, "u1", byID[user].UserID)
}
