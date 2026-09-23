package results

import "testing"

func TestFormatDuration_WholeSecondCeiling(t *testing.T) {
	// World Athletics TR 19.24.5 whole-second ceiling rounding for road races
	cases := []struct {
		ms   int64
		want string
	}{
		{0, "0:00:00"},
		{1, "0:00:01"},
		{999, "0:00:01"},
		{1000, "0:00:01"},
		{1001, "0:00:02"},
		// Minute boundaries
		{59999, "0:01:00"},
		{60000, "0:01:00"},
		{60001, "0:01:01"},
		{119999, "0:02:00"},
		{120000, "0:02:00"},
		{120001, "0:02:01"},
		// Hour boundaries
		{3599999, "1:00:00"},
		{3600000, "1:00:00"},
		{3600001, "1:00:01"},
		{7199999, "2:00:00"},
		{7200000, "2:00:00"},
		{7200001, "2:00:01"},
		// Arbitrary race times
		{5025000, "1:23:45"},
		{5025100, "1:23:46"},
		{5025500, "1:23:46"},
		{36000000, "10:00:00"},
		{-500, "0:00:00"}, // negative clamped to zero
	}
	for _, c := range cases {
		if got := formatDuration(c.ms); got != c.want {
			t.Errorf("formatDuration(%d) = %q, want %q", c.ms, got, c.want)
		}
	}
}

func TestFormatDurationWithPrecision(t *testing.T) {
	// Standard floor seconds ("s")
	if got := FormatDurationWithPrecision(1500, "s"); got != "0:00:01" {
		t.Errorf("expected 0:00:01, got %s", got)
	}

	// Centiseconds ("cs")
	if got := FormatDurationWithPrecision(1542, "cs"); got != "0:00:01.55" {
		t.Errorf("expected 0:00:01.55, got %s", got)
	}
	if got := FormatDurationWithPrecision(1000, "cs"); got != "0:00:01.00" {
		t.Errorf("expected 0:00:01.00, got %s", got)
	}

	// Milliseconds ("ms")
	if got := FormatDurationWithPrecision(1542, "ms"); got != "0:00:01.542" {
		t.Errorf("expected 0:00:01.542, got %s", got)
	}
}
