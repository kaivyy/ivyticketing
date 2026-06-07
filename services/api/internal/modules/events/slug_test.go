package events

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Jakarta Marathon 2026": "jakarta-marathon-2026",
		"  Trail   Run ":        "trail-run",
		"Bali!!! Run":           "bali-run",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
