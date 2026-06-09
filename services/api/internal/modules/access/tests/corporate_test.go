package access_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/access"
)

// fakeAccessRepoWithSlots extends fakeAccessRepo to control pool available slots.
type fakeAccessRepoWithSlots struct {
	fakeAccessRepo
	poolAvailable int32
}

func (r *fakeAccessRepoWithSlots) GetAccessPool(_ context.Context, _ uuid.UUID) (db.AccessPool, error) {
	return db.AccessPool{
		ID:            uuid.New(),
		TotalSlots:    r.poolAvailable,
		ReservedSlots: 0,
		UsedSlots:     0,
	}, nil
}

func TestCorporate_BulkUpload_ParsesCSV(t *testing.T) {
	csv := "email,name\nfoo@example.com,Foo\nbar@example.com,Bar\n"
	repo := &fakeAccessRepoWithSlots{poolAvailable: 100}
	svc := access.NewCorporateService(repo)
	result, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 {
		t.Fatalf("want 2 imported, got %d", result.Imported)
	}
}

func TestCorporate_BulkUpload_SkipsDuplicateEmails(t *testing.T) {
	csv := "email\nfoo@example.com\nfoo@example.com\nbar@example.com\n"
	repo := &fakeAccessRepoWithSlots{poolAvailable: 100}
	svc := access.NewCorporateService(repo)
	result, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 {
		t.Fatalf("want 2 (deduped), got %d", result.Imported)
	}
	if result.Skipped != 1 {
		t.Fatalf("want 1 skipped duplicate, got %d", result.Skipped)
	}
}

func TestCorporate_BulkUpload_RejectsIfExceedsQuota(t *testing.T) {
	csv := "email\na@x.com\nb@x.com\nc@x.com\n"
	// pool only has 2 available slots
	repo := &fakeAccessRepoWithSlots{poolAvailable: 2}
	svc := access.NewCorporateService(repo)
	_, err := svc.BulkUploadMembers(context.Background(), uuid.New(), uuid.New(), strings.NewReader(csv))
	if err == nil {
		t.Fatal("should reject upload that exceeds pool quota")
	}
}
