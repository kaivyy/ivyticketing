//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
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
		DELETE FROM inventory_reservations;
		DELETE FROM payment_webhooks;
		DELETE FROM payments;
		DELETE FROM orders;
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

// registerAndLogin registers a new user and returns their access token.
func registerAndLogin(t *testing.T, client *http.Client, baseURL, email string) string {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/auth/register",
		map[string]string{"email": email, "password": "pw123456", "fullName": email}, "").Body.Close()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "pw123456"}, "")
	var login struct {
		AccessToken string `json:"accessToken"`
	}
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()
	return login.AccessToken
}

// publishEventWithCategory creates an event, adds a category (capacity), and publishes it.
// Returns (eventID, categoryID). Requires event.create/edit/publish + category.manage.
func publishEventWithCategory(t *testing.T, client *http.Client, baseURL, token, orgID string, capacity, maxOrder int) (string, string) {
	t.Helper()
	eventID := createEvent(t, client, baseURL, token, orgID, "Marathon "+orgID[:8])

	resp := postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories",
		map[string]any{
			"name": "42K", "price": 100000, "capacity": capacity,
			"registrationOpensAt":  time.Now().Add(-time.Hour).Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      maxOrder,
		}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category = %d, want 201", resp.StatusCode)
	}
	var cat struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&cat)
	resp.Body.Close()

	resp = postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/publish", map[string]any{}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("publish = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	return eventID, cat.ID
}

// seedPublishedCategory inserts an org, a published event, and a category with the
// given capacity directly via SQL, returning their IDs. For concurrency tests.
func seedPublishedCategory(t *testing.T, pool *pgxpool.Pool, capacity, maxOrder int32) (orgID, eventID, categoryID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	orgID = uuid.New()
	eventID = uuid.New()
	categoryID = uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`,
		orgID, "Conc Org "+orgID.String()[:8], "conc-"+orgID.String()[:8])
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	_, err = pool.Exec(ctx,
		fmt.Sprintf(`INSERT INTO events (id, organization_id, name, slug, event_type, status)
		VALUES ($1,$2,'E','e-%s','marathon','published')`, eventID.String()[:8]),
		eventID, orgID)
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO event_categories
		(id, organization_id, event_id, name, price, capacity, registration_opens_at, registration_closes_at, max_order_per_user)
		VALUES ($1,$2,$3,'42K',100000,$4, now()-interval '1 hour', now()+interval '1 hour', $5)`,
		categoryID, orgID, eventID, capacity, maxOrder)
	if err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return orgID, eventID, categoryID
}

// seedUsers bulk-inserts n users into the users table and returns their UUIDs.
func seedUsers(t *testing.T, pool *pgxpool.Pool, n int) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, 'x', 'Test User')`,
			ids[i], fmt.Sprintf("user-%s@test.com", ids[i].String()[:8]))
		if err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	return ids
}
