package auth

import (
	"common/pkg/permission"
	"context"

	"github.com/gin-gonic/gin"
)

// AuthContext holds the authenticated user's identity and permissions.
// Populated by Middleware, then available to RequirePermission and
// all downstream handlers via GetAuth / GetAuthFromContext.
type AuthContext struct {
	UserID      uint
	Permissions []permission.Permission
}

type authKeyType struct{}

var authKey = authKeyType{}

// SetAuth stores the authenticated user in both the Gin context and the
// request's stdlib context. The Gin context is used by middleware (GetAuth),
// and the stdlib context is used by service-layer code (GetAuthFromContext).
func SetAuth(c *gin.Context, auth *AuthContext) {
	c.Set(authKey, auth)
	ctx := context.WithValue(c.Request.Context(), authKey, auth)
	c.Request = c.Request.WithContext(ctx)
}

// GetAuth retrieves the authenticated user from the Gin context.
func GetAuth(c *gin.Context) *AuthContext {
	val, exists := c.Get(authKey)
	if !exists {
		return nil
	}

	auth, ok := val.(*AuthContext)
	if !ok {
		return nil
	}

	return auth
}

// GetAuthFromContext retrieves the authenticated user from a stdlib context.
func GetAuthFromContext(ctx context.Context) *AuthContext {
	val := ctx.Value(authKey)
	if val == nil {
		return nil
	}

	auth, ok := val.(*AuthContext)
	if !ok {
		return nil
	}

	return auth
}
