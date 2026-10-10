package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IMP 11-clinic-reservation-slot-api-admin-settings: 診療日の境界は JST（tzdata に依存しない固定オフセット）

func TestJST_FixedOffset(t *testing.T) {
	_, off := time.Date(2026, 1, 1, 0, 0, 0, 0, JST).Zone()
	assert.Equal(t, 9*3600, off)
}

func TestStartOfDay(t *testing.T) {
	// UTC 2026-10-11 16:00 は JST 2026-10-12 01:00 → 診療日は 12 日
	got := StartOfDay(time.Date(2026, 10, 11, 16, 0, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, JST), got)

	// 境界: JST 23:59:59.999 は同日、00:00:00 は翌日
	assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, JST), StartOfDay(time.Date(2026, 10, 12, 23, 59, 59, 999, JST)))
	assert.Equal(t, time.Date(2026, 10, 13, 0, 0, 0, 0, JST), StartOfDay(time.Date(2026, 10, 13, 0, 0, 0, 0, JST)))
}

func TestParseDate(t *testing.T) {
	d, err := ParseDate("2026-10-12")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, JST), d)

	for _, bad := range []string{"", "2026-13-01", "2026-02-30", "2026/10/12", "10-12-2026", "2026-10-12T00:00:00Z", "abc"} {
		_, err := ParseDate(bad)
		assert.Error(t, err, "%q", bad)
	}
}

func TestParseClock(t *testing.T) {
	d, err := ParseClock("09:30")
	require.NoError(t, err)
	assert.Equal(t, 9*time.Hour+30*time.Minute, d)

	d, err = ParseClock("00:00")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), d)

	d, err = ParseClock("23:59")
	require.NoError(t, err)
	assert.Equal(t, 23*time.Hour+59*time.Minute, d)

	for _, bad := range []string{"", "24:00", "09:60", "9:30", "09:30:00", "ab:cd", "-1:00"} {
		_, err := ParseClock(bad)
		assert.Error(t, err, "%q", bad)
	}
}

func TestFormatClock(t *testing.T) {
	assert.Equal(t, "09:05", FormatClock(9*time.Hour+5*time.Minute))
	assert.Equal(t, "00:00", FormatClock(0))
}
