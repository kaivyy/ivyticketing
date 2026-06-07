package orders

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerateOrderNumber_Format(t *testing.T) {
	num, err := generateOrderNumber(time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	re := regexp.MustCompile(`^ORD-20260607-[A-Z0-9]{6}$`)
	if !re.MatchString(num) {
		t.Errorf("order number %q does not match format", num)
	}
}

func TestGenerateOrderNumber_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		num, err := generateOrderNumber(time.Now())
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[num] {
			t.Fatalf("collision at %d: %s", i, num)
		}
		seen[num] = true
	}
}
