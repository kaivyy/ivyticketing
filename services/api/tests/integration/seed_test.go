//go:build integration

package integration

import (
	"context"
	"testing"
)

func TestSeed_CatalogAndTemplatesPresent(t *testing.T) {
	pool := testPool(t)

	var permCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM permissions").Scan(&permCount); err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	// Baseline check, not an exact count: later phases add permissions to the
	// catalog (24 at Phase 8, 35 by Phase 21). Asserting a floor verifies the
	// seed ran without turning every RBAC addition into a test edit.
	if permCount < 24 {
		t.Errorf("permissions = %d, want >= 24 (seed catalog missing)", permCount)
	}

	var tmplCount int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM roles WHERE organization_id IS NULL").Scan(&tmplCount); err != nil {
		t.Fatalf("count templates: %v", err)
	}
	if tmplCount != 5 {
		t.Errorf("template roles = %d, want 5", tmplCount)
	}
}
