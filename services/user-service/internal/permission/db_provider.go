package permission

import (
	perm "common/pkg/permission"
	"context"

	"gorm.io/gorm"
)

// DBPermissionProvider loads all permissions for a user by querying the
// employee_permissions table directly. Only user-service uses this -- it
// owns the permissions data and can query without a network call.
type DBPermissionProvider struct {
	db *gorm.DB
}

func NewDBPermissionProvider(db *gorm.DB) *DBPermissionProvider {
	return &DBPermissionProvider{db: db}
}

func (p *DBPermissionProvider) GetPermissions(ctx context.Context, userID uint) ([]perm.Permission, error) {
	var permissions []perm.Permission

	err := p.db.WithContext(ctx).
		Table("employee_permissions").
		Where("employee_id = ?", userID).
		Pluck("permission", &permissions).Error

	if err != nil {
		return nil, err
	}

	return permissions, nil
}
