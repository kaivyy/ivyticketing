package inventory

import "testing"

func TestRemaining(t *testing.T) {
	cases := []struct {
		capacity, reserved, paid int64
		want                     int64
	}{
		{100, 0, 0, 100},
		{100, 30, 20, 50},
		{100, 100, 0, 0},
		{100, 60, 40, 0},
	}
	for _, c := range cases {
		if got := Remaining(c.capacity, c.reserved, c.paid); got != c.want {
			t.Errorf("Remaining(%d,%d,%d) = %d, want %d", c.capacity, c.reserved, c.paid, got, c.want)
		}
	}
}

func TestHasCapacity(t *testing.T) {
	if !HasCapacity(100, 50, 49) {
		t.Error("expected capacity available (1 left)")
	}
	if HasCapacity(100, 60, 40) {
		t.Error("expected no capacity (full)")
	}
}
