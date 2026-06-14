package dto

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
)

func TestAuditLogResponses(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entry := audit.AuditLog{
		ID:                    1,
		ActionType:            audit.ActionOrderApproved,
		PerformedByEmployeeID: 7,
		Details:               "order approved",
		CreatedAt:             now,
	}

	resp := ToAuditLogResponse(entry)
	if resp.ID != 1 || resp.ActionType != audit.ActionOrderApproved || !resp.CreatedAt.Equal(now) {
		t.Fatalf("unexpected audit response %#v", resp)
	}

	list := ToAuditLogResponseList([]audit.AuditLog{entry}, 11, 2, 5)
	if len(list.Data) != 1 || list.TotalPages != 3 || list.Page != 2 {
		t.Fatalf("unexpected audit response list %#v", list)
	}
}
