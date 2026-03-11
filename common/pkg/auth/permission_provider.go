package auth

import (
	"common/pkg/permission"
	"context"
)

// PermissionProvider loads all permissions for a user. Used by
// Middleware to populate AuthContext.Permissions on every
// authenticated request.
//
// Implementations:
//   - DBPermissionProvider:   queries the DB directly (used by user-service)
//   - GRPCPermissionProvider: calls user-service over gRPC (used by other services)
type PermissionProvider interface {
	GetPermissions(ctx context.Context, userID uint) ([]permission.Permission, error)
}
