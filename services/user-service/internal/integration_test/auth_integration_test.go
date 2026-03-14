//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"common/pkg/auth"

	"github.com/stretchr/testify/assert"
)

func TestAuthLoginClaims(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	identity, employee := seedEmployee(t, db, position.PositionID)

	recorder := performRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    identity.Email,
		"password": "Password12",
	}, "")

	requireStatus(t, recorder, http.StatusOK)

	response := decodeResponse[loginResponse](t, recorder)
	assert.Equal(t, employee.EmployeeID, response.User.ID)

	claims := verifyAccessToken(t, response.Token)
	assert.Equal(t, identity.ID, claims.IdentityID)
	assert.Equal(t, string(auth.IdentityEmployee), claims.IdentityType)
	if assert.NotNil(t, claims.EmployeeID) {
		assert.Equal(t, employee.EmployeeID, *claims.EmployeeID)
	}
	assert.Nil(t, claims.ClientID)
}

func TestAuthRefreshClaims(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)
	position := seedPosition(t, db)
	identity, employee := seedEmployee(t, db, position.PositionID)

	login := loginEmployee(t, router, identity.Email, "Password12")

	recorder := performRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{
		"refresh_token": login.RefreshToken,
	}, "")

	requireStatus(t, recorder, http.StatusOK)

	response := decodeResponse[loginResponse](t, recorder)
	assert.Equal(t, employee.EmployeeID, response.User.ID)

	claims := verifyAccessToken(t, response.Token)
	assert.Equal(t, identity.ID, claims.IdentityID)
	assert.Equal(t, string(auth.IdentityEmployee), claims.IdentityType)
	if assert.NotNil(t, claims.EmployeeID) {
		assert.Equal(t, employee.EmployeeID, *claims.EmployeeID)
	}
	assert.Nil(t, claims.ClientID)
}
