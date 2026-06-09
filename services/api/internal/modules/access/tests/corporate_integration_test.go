//go:build integration
// +build integration

package access_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

// fakeAccessRepoWithTracking extends fakeAccessRepoWithSlots to record inserted members.
type fakeAccessRepoWithTracking struct {
	fakeAccessRepoWithSlots
	insertedEmails []string
}

func (r *fakeAccessRepoWithTracking) AddPoolMember(_ context.Context, arg db.AddPoolMemberParams) (db.AccessPoolMember, error) {
	r.insertedEmails = append(r.insertedEmails, arg.Email)
	return db.AccessPoolMember{ID: uuid.New(), PoolID: arg.PoolID, Email: arg.Email}, nil
}

func TestCorporateBulkUpload_10kRows_AllImported(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("email\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString("user" + strconv.Itoa(i) + "@corp.example.com\n")
	}

	repo := &fakeAccessRepoWithTracking{
		fakeAccessRepoWithSlots: fakeAccessRepoWithSlots{poolAvailable: 10001},
	}
	svc := access.NewCorporateService(repo)

	result, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("10k upload should succeed: %v", err)
	}
	if result.Imported != 10000 {
		t.Fatalf("want 10000 imported, got %d", result.Imported)
	}
	if result.Skipped != 0 {
		t.Fatalf("want 0 skipped, got %d", result.Skipped)
	}
	if len(repo.insertedEmails) != 10000 {
		t.Fatalf("repo has %d members, want 10000", len(repo.insertedEmails))
	}
}

func TestCorporateBulkUpload_ExceedsQuota_RejectsAll(t *testing.T) {
	csv := "email\na@x.com\nb@x.com\nc@x.com\n" // 3 rows but only 2 slots

	repo := &fakeAccessRepoWithTracking{
		fakeAccessRepoWithSlots: fakeAccessRepoWithSlots{poolAvailable: 2},
	}
	svc := access.NewCorporateService(repo)

	_, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err == nil {
		t.Fatal("should reject upload exceeding quota")
	}
	// No members should have been inserted — transactional rejection
	if len(repo.insertedEmails) != 0 {
		t.Fatalf("no members should be inserted on rejection, got %d", len(repo.insertedEmails))
	}
}

func TestCorporateBulkUpload_AllDuplicates_ZeroImported(t *testing.T) {
	csv := "email\nsame@corp.com\nsame@corp.com\nsame@corp.com\n"

	repo := &fakeAccessRepoWithTracking{
		fakeAccessRepoWithSlots: fakeAccessRepoWithSlots{poolAvailable: 100},
	}
	svc := access.NewCorporateService(repo)

	result, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err != nil {
		t.Fatalf("all-dup upload should not error: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("want 1 (deduped to single), got %d", result.Imported)
	}
	if result.Skipped != 2 {
		t.Fatalf("want 2 skipped, got %d", result.Skipped)
	}
}
