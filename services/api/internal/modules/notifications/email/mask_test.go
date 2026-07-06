package email

import "testing"

func TestMaskAddress(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"jane.doe@example.com", "j***@example.com"},
		{"a@example.com", "*@example.com"},
		{"bob@sub.domain.co", "b***@sub.domain.co"},
		{"", "***"},
		{"noatsign", "***"},
		{"@example.com", "***"},
	}
	for _, c := range cases {
		if got := MaskAddress(c.in); got != c.want {
			t.Errorf("MaskAddress(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
