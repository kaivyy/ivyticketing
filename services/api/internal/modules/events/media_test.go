package events

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateObjectKey(t *testing.T) {
	orgID := uuid.New()
	eventID := uuid.New()
	good := mediaKeyPrefix(orgID, eventID, "banner") + "abc.png"
	if err := validateObjectKey(good, orgID, eventID, "banner"); err != nil {
		t.Errorf("good key rejected: %v", err)
	}
	// wrong event
	otherEvent := uuid.New()
	bad := mediaKeyPrefix(orgID, otherEvent, "banner") + "abc.png"
	if err := validateObjectKey(bad, orgID, eventID, "banner"); err == nil {
		t.Error("key for another event should be rejected")
	}
	// traversal attempt
	if err := validateObjectKey("../../etc/passwd", orgID, eventID, "banner"); err == nil {
		t.Error("traversal key should be rejected")
	}
}

func TestValidExtension(t *testing.T) {
	if !validImageContentType("image/png") || !validImageContentType("image/jpeg") || !validImageContentType("image/webp") {
		t.Error("standard image types should be allowed")
	}
	if validImageContentType("application/pdf") {
		t.Error("pdf should be rejected")
	}
}
