package notifications_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	notifmod "github.com/varin/ivyticketing/services/api/internal/modules/notifications"
)

// fakeUserLookup implements ParticipantLookup for testing
type fakeUserLookup struct {
	users map[uuid.UUID]fakeUser
}

type fakeUser struct {
	email string
	name  string
}

func (f *fakeUserLookup) GetByID(_ context.Context, id uuid.UUID) (string, string, error) {
	user, ok := f.users[id]
	if !ok {
		return "", "", nil
	}
	return user.email, user.name, nil
}

func TestParticipantLookup_Adapter(t *testing.T) {
	lookup := &fakeUserLookup{
		users: map[uuid.UUID]fakeUser{
			uuid.New(): {email: "user@example.com", name: "Test User"},
		},
	}

	// Verify the interface is satisfied
	var _ notifmod.ParticipantLookup = lookup

	ctx := context.Background()
	for id, expected := range lookup.users {
		email, name, err := lookup.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID returned error: %v", err)
		}
		if email != expected.email {
			t.Errorf("expected email %q, got %q", expected.email, email)
		}
		if name != expected.name {
			t.Errorf("expected name %q, got %q", expected.name, name)
		}
	}
}

func TestParticipantLookup_NotFound(t *testing.T) {
	lookup := &fakeUserLookup{
		users: map[uuid.UUID]fakeUser{},
	}

	ctx := context.Background()
	email, name, err := lookup.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetByID returned error for non-existent user: %v", err)
	}
	if email != "" || name != "" {
		t.Errorf("expected empty email and name, got %q, %q", email, name)
	}
}
