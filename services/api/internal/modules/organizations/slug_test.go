package organizations

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Jakarta Marathon":    "jakarta-marathon",
		"  Trail   Run 2026 ": "trail-run-2026",
		"Bali!!! Run":         "bali-run",
		"ALL CAPS":            "all-caps",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
