package payments

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerateMerchantReference_Format(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	ref, err := generateMerchantReference(now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	re := regexp.MustCompile(`^PAY-20260607-[0-9A-Z]{6}$`)
	if !re.MatchString(ref) {
		t.Errorf("ref %q does not match expected format", ref)
	}
}

func TestGenerateMerchantReference_Unique(t *testing.T) {
	now := time.Now()
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		ref, err := generateMerchantReference(now)
		if err != nil {
			t.Fatal(err)
		}
		if seen[ref] {
			t.Fatalf("collision on %s", ref)
		}
		seen[ref] = true
	}
}
