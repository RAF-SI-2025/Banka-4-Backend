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
		{
			name:     "last Thursday of December 2026 (December 31)",
			date:     time.Date(2026, time.December, 31, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "non-dividend month (January)",
			date:     time.Date(2026, time.January, 31, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			// September 30 2023 je subota — poslednji radni dan je petak 29.
			name:     "last business day when month ends on Saturday",
			date:     time.Date(2023, time.September, 29, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLastBusinessDayOfDividendQuarter(tt.date)
			require.Equal(t, tt.expected, result)
		})
	}
}
