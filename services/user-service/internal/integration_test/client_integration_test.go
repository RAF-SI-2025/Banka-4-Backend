//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"common/pkg/auth"
	"user-service/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientRegister(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	email := fmt.Sprintf("client-%d@example.com", uniqueCounter.Add(1))
	username := fmt.Sprintf("client-%d", uniqueCounter.Add(1))

	recorder := performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Mika",
		"last_name":     "Client",
		"date_of_birth": time.Now().UTC().AddDate(-28, 0, 0).Format(time.RFC3339),
		"gender":        "male",
		"email":         email,
		"username":      username,
		"phone_number":  "0601111111",
		"address":       "Client Street 1",
	}, "")

	requireStatus(t, recorder, http.StatusCreated)

	var identity model.Identity
	require.NoError(t, db.Where("email = ?", email).First(&identity).Error)
	assert.Equal(t, auth.IdentityClient, identity.Type)
	assert.False(t, identity.Active)

	var client model.Client
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&client).Error)
	assert.Equal(t, "Mika", client.FirstName)
	assert.Equal(t, identity.ID, client.IdentityID)

	var activationToken model.ActivationToken
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&activationToken).Error)
	assert.NotEmpty(t, activationToken.Token)
}

func TestClientRegisterActivateAndLogin(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	router := setupTestRouter(t, db)

	email := fmt.Sprintf("clientauth-%d@example.com", uniqueCounter.Add(1))
	username := fmt.Sprintf("clientauth-%d", uniqueCounter.Add(1))

	register := performRequest(t, router, http.MethodPost, "/api/clients/register", map[string]any{
		"first_name":    "Jana",
		"last_name":     "Client",
		"date_of_birth": time.Now().UTC().AddDate(-31, 0, 0).Format(time.RFC3339),
		"gender":        "female",
		"email":         email,
		"username":      username,
		"phone_number":  "0602222222",
		"address":       "Client Street 2",
	}, "")
	requireStatus(t, register, http.StatusCreated)

	var identity model.Identity
	require.NoError(t, db.Where("email = ?", email).First(&identity).Error)

	var client model.Client
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&client).Error)

	var activationToken model.ActivationToken
	require.NoError(t, db.Where("identity_id = ?", identity.ID).First(&activationToken).Error)

	activate := performRequest(t, router, http.MethodPost, "/api/auth/activate", map[string]any{
		"token":    activationToken.Token,
		"password": "Password12",
	}, "")
	requireStatus(t, activate, http.StatusOK)

	login := performRequest(t, router, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    email,
		"password": "Password12",
	}, "")
	requireStatus(t, login, http.StatusOK)

	response := decodeResponse[loginResponse](t, login)
	assert.Equal(t, client.ClientID, response.User.ID)

	claims := verifyAccessToken(t, response.Token)
	assert.Equal(t, identity.ID, claims.IdentityID)
	assert.Equal(t, string(auth.IdentityClient), claims.IdentityType)
	if assert.NotNil(t, claims.ClientID) {
		assert.Equal(t, client.ClientID, *claims.ClientID)
	}
	assert.Nil(t, claims.EmployeeID)
}
