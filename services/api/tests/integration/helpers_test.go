//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/app"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// truncate clears all tenant data but keeps the seeded catalog/templates.
func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		DELETE FROM form_fields;
		DELETE FROM form_schemas;
		DELETE FROM event_categories;
		DELETE FROM events;
		DELETE FROM member_roles;
		DELETE FROM organization_members;
		DELETE FROM audit_logs;
		DELETE FROM refresh_tokens;
		DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE organization_id IS NOT NULL);
		DELETE FROM roles WHERE organization_id IS NOT NULL;
		DELETE FROM organizations;
		DELETE FROM users;
	`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

type stubChecker struct{}

func (stubChecker) Ping(context.Context) error { return nil }

func newNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestServer(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()
	mediaDir, err := os.MkdirTemp("", "ivyticketing-test-media-*")
	if err != nil {
		t.Fatalf("create temp media dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(mediaDir) })
	cfg := app.Config{
		AppEnv:                "test",
		APIPort:               "0",
		WebOrigin:             "http://localhost:4321",
		JWTSecret:             "integration-secret",
		AccessTokenTTL:        15 * time.Minute,
		RefreshTokenTTL:       168 * time.Hour,
		StorageDriver:         "local",
		StorageLocalPath:      mediaDir,
		StoragePublicBaseURL:  "http://localhost:8080",
		StorageUploadMaxBytes: 5242880,
	}
	h, err := app.NewRouter(cfg, newNopLogger(), pool, stubChecker{}, stubChecker{})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func refreshCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}

// createEvent makes a draft event and returns its ID. Requires event.create.
func createEvent(t *testing.T, client *http.Client, baseURL, token, orgID, name string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": name, "eventType": "marathon"}, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create event = %d, want 201", resp.StatusCode)
	}
	var ev struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&ev)
	return ev.ID
}

// bytesReader wraps a byte slice in a *bytes.Reader for use as an HTTP body.
func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}
