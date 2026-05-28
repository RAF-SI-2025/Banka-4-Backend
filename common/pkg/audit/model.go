package audit

import "time"

const (
	ActionActuaryLimitChanged        = "ACTUARY_LIMIT_CHANGED"
	ActionActuaryLimitReset          = "ACTUARY_LIMIT_RESET"
	ActionOrderApproved              = "ORDER_APPROVED"
	ActionOrderDeclined              = "ORDER_DECLINED"
	ActionEmployeePermissionsChanged = "EMPLOYEE_PERMISSIONS_CHANGED"
	ActionTaxCollectionTriggered     = "TAX_COLLECTION_TRIGGERED"
)

type AuditLog struct {
	ID                    uint      `gorm:"primaryKey;autoIncrement"`
	ActionType            string    `gorm:"not null;size:50;index"`
	PerformedByEmployeeID uint      `gorm:"not null;index"`
	Details               string    `gorm:"type:text"`
	CreatedAt             time.Time `gorm:"index"`
}
