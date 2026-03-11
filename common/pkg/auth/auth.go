package auth

import (
	"common/pkg/permission"
	"strings"

	"common/pkg/errors"

	"github.com/gin-gonic/gin"
)

// Middleware validates the Bearer token and loads the user's permissions
// into the request context. After this middleware runs, handlers can call
// GetAuth(c) to access UserID and Permissions without any extra DB or
// gRPC calls.
func Middleware(verifier TokenVerifier, provider PermissionProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Error(errors.UnauthorizedErr("missing authorization header"))
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Error(errors.UnauthorizedErr("authorization header must use bearer"))
			c.Abort()
			return
		}

		claims, err := verifier.VerifyToken(parts[1])
		if err != nil {
			c.Error(errors.UnauthorizedErr("invalid or expired token"))
			c.Abort()
			return
		}

		permissions, err := provider.GetPermissions(c.Request.Context(), claims.UserID)
		if err != nil {
			c.Error(errors.InternalErr(err))
			c.Abort()
			return
		}

		SetAuth(c, &AuthContext{
			UserID:      claims.UserID,
			Permissions: permissions,
		})

		c.Next()
	}
}

// RequirePermission checks that the authenticated user holds all the
// given permissions. Must run after Middleware. Checks against the
// permissions already loaded into AuthContext
func RequirePermission(permissions ...permission.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		context := GetAuth(c)
		if context == nil {
			c.Error(errors.UnauthorizedErr("not authenticated"))
			c.Abort()
			return
		}

		for _, required := range permissions {
			if !hasPermission(required, context.Permissions) {
				c.Error(errors.ForbiddenErr("insufficient permissions"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func hasPermission(perm permission.Permission, permissions []permission.Permission) bool {
	for _, p := range permissions {
		if p == perm {
			return true
		}
	}
	return false
}
