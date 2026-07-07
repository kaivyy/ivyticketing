package results

import "testing"

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{0, "0:00:00"},
		{1000, "0:00:01"},
		{61000, "0:01:01"},
		{3600000, "1:00:00"},
		{5025000, "1:23:45"},
		{5025500, "1:23:45"}, // sub-second truncated
		{36000000, "10:00:00"},
		{-500, "0:00:00"}, // negative clamped
	}
	for _, c := range cases {
		if got := formatDuration(c.ms); got != c.want {
			t.Errorf("formatDuration(%d) = %q, want %q", c.ms, got, c.want)
		}
	}
}
