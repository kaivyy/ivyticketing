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
	if permCount != 21 {
		t.Errorf("permissions = %d, want 21", permCount)
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
