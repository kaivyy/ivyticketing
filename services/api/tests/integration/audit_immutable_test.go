//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
)

// TestAuditLogsImmutable verifies the append-only trigger from migration 00058:
// INSERT succeeds, but UPDATE and DELETE are rejected at the database level.
// This proves audit-trail immutability regardless of caller privileges.
func TestAuditLogsImmutable(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO audit_logs (action, target_type, target_id, metadata)
		VALUES ('TEST_IMMUTABLE', 'test', 'x', '{}'::jsonb)
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("insert audit row: %v", err)
	}

	// UPDATE must be blocked.
	_, err = pool.Exec(ctx, `UPDATE audit_logs SET action = 'TAMPERED' WHERE id = $1`, id)
	if err == nil {
		t.Fatal("UPDATE on audit_logs succeeded, want rejection")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE error = %v, want append-only rejection", err)
	}

	// DELETE must be blocked.
	_, err = pool.Exec(ctx, `DELETE FROM audit_logs WHERE id = $1`, id)
	if err == nil {
		t.Fatal("DELETE on audit_logs succeeded, want rejection")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE error = %v, want append-only rejection", err)
	}

	// Row must still be present and unchanged.
	var action string
	if err := pool.QueryRow(ctx, `SELECT action FROM audit_logs WHERE id = $1`, id).Scan(&action); err != nil {
		t.Fatalf("reread audit row: %v", err)
	}
	if action != "TEST_IMMUTABLE" {
		t.Fatalf("action = %q, want TEST_IMMUTABLE (unchanged)", action)
	}
}
