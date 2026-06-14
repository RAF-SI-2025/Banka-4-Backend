package dto

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func TestOTCListRequestNormalizeDefaultsNonPositiveValues(t *testing.T) {
	req := OTCListRequest{}
	req.Normalize()

	if req.Page != 1 || req.PageSize != 10 {
		t.Fatalf("Normalize() = page %d pageSize %d", req.Page, req.PageSize)
	}

	req = OTCListRequest{Page: 3, PageSize: 50}
	req.Normalize()
	if req.Page != 3 || req.PageSize != 50 {
		t.Fatalf("Normalize changed valid values: %#v", req)
	}
}

func TestToOtcExecutionLogEntryResponses(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 30, 0, 0, time.UTC)
	responses := ToOtcExecutionLogEntryResponses([]model.OtcExecutionSagaLogEntry{
		{
			Step:      "F1",
			Outcome:   model.OtcExecutionLogOutcomeErr,
			Error:     "transfer failed",
			CreatedAt: now,
		},
	})

	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d", len(responses))
	}
	if responses[0].Step != "F1" || responses[0].Outcome != model.OtcExecutionLogOutcomeErr || responses[0].Error != "transfer failed" || !responses[0].CreatedAt.Equal(now) {
		t.Fatalf("unexpected response %#v", responses[0])
	}
}
