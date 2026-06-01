package job

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsLastBusinessDayOfDividendQuarter(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected bool
	}{
		{
			name:     "last Tuesday of March 2026 (March 31)",
			date:     time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Friday March 27 — not last business day",
			date:     time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "last Monday of June 2026 (June 30)",
			date:     time.Date(2026, time.June, 30, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Friday June 26 — not last business day",
			date:     time.Date(2026, time.June, 26, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "last Wednesday of September 2026 (September 30)",
			date:     time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Friday September 25 — not last business day",
			date:     time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLastBusinessDayOfDividendQuarter(tt.date)
			require.Equal(t, tt.expected, result)
		})
	}
}
