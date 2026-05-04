package auth

import (
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
)

// Middleware validates the Bearer token and loads the identity's permissions
// into the request context. After this middleware runs, handlers can call
// GetAuth(c) to access IdentityID, IdentityType, ClientID, EmployeeID,
// and Permissions without any extra DB or gRPC calls.
func Middleware(verifier TokenVerifier, provider PermissionProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			_ = c.Error(errors.UnauthorizedErr("missing authorization header"))
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			_ = c.Error(errors.UnauthorizedErr("authorization header must use bearer"))
			c.Abort()
			return
		}

		claims, err := verifier.VerifyToken(parts[1])
		if err != nil {
			_ = c.Error(errors.UnauthorizedErr("invalid or expired token"))
			c.Abort()
			return
		}

		permissions, err := provider.GetPermissions(c.Request.Context(), claims)
		if err != nil {
			_ = c.Error(errors.InternalErr(err))
			c.Abort()
			return
		}

		SetAuth(c, &AuthContext{
			IdentityID:   claims.IdentityID,
			IdentityType: IdentityType(claims.IdentityType),
			ClientID:     claims.ClientID,
			EmployeeID:   claims.EmployeeID,
			Permissions:  permissions,
		})

		c.Next()
	}
}

// RequirePermission checks that the authenticated identity holds all the
// given permissions. Must run after Middleware. Checks against the
// permissions already loaded into AuthContext.
func RequirePermission(permissions ...permission.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		context := GetAuth(c)
		if context == nil {
			_ = c.Error(errors.UnauthorizedErr("not authenticated"))
			c.Abort()
			return
		}

		for _, required := range permissions {
			if !hasPermission(required, context.Permissions) {
				_ = c.Error(errors.ForbiddenErr("insufficient permissions"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireIdentityType checks that the authenticated identity is one of the
// allowed types. Must run after Middleware.
func RequireIdentityType(allowed ...IdentityType) gin.HandlerFunc {
	return func(c *gin.Context) {
		ac := GetAuth(c)
		if ac == nil {
			_ = c.Error(errors.UnauthorizedErr("not authenticated"))
			c.Abort()
			return
		}

		for _, t := range allowed {
			if ac.IdentityType == t {
				c.Next()
				return
			}
		}

		_ = c.Error(errors.ForbiddenErr("access denied for this identity type"))
		c.Abort()
	}
}

func RequireClientSelf(param string, allowEmployee bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ac := GetAuth(c)
		if ac == nil {
			abortWithError(c, errors.UnauthorizedErr("not authenticated"))
			return
		}

		if allowEmployee && ac.IdentityType == IdentityEmployee {
			c.Next()
			return
		}

		if ac.IdentityType != IdentityClient || ac.ClientID == nil {
			abortWithError(c, errors.ForbiddenErr("access denied"))
			return
		}

		clientID, err := strconv.ParseUint(c.Param(param), 10, 64)
		if err != nil {
			abortWithError(c, errors.BadRequestErr("invalid client id"))
			return
		}

		if uint(clientID) != *ac.ClientID {
			abortWithError(c, errors.ForbiddenErr("client id does not match authenticated user"))
			return
		}

		c.Next()
	}
}

func abortWithError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

func hasPermission(perm permission.Permission, permissions []permission.Permission) bool {
	for _, p := range permissions {
		if p == perm {
			return true
		}
	}

	return false
}

func AnyOf(middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, m := range middlewares {
			probeRecorder := httptest.NewRecorder()
			probe, _ := gin.CreateTestContext(probeRecorder)
			probe.Request = c.Request

			for key, value := range c.Keys {
				probe.Set(key, value)
			}

			m(probe)
			if !probe.IsAborted() {
				c.Next()
				return
			}
		}
		// none passed, abort
		abortWithError(c, errors.ForbiddenErr("user does not satisfy auth requirements"))
	}
}
