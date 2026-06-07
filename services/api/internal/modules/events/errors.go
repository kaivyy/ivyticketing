package events

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrNotFound          = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrSlugTaken         = apperr.New(http.StatusConflict, "SLUG_TAKEN", "an event with this name already exists")
	ErrNoCategories      = apperr.New(http.StatusConflict, "EVENT_NO_CATEGORIES", "cannot publish an event with no categories")
	ErrInvalidTransition = apperr.New(http.StatusConflict, "INVALID_STATUS_TRANSITION", "invalid status transition")
	ErrInvalidEventType  = apperr.New(http.StatusBadRequest, "INVALID_EVENT_TYPE", "unknown event type")
	ErrInvalidObjectKey  = apperr.New(http.StatusBadRequest, "INVALID_OBJECT_KEY", "object key does not belong to this event")
	ErrInvalidContent    = apperr.New(http.StatusBadRequest, "INVALID_CONTENT_TYPE", "unsupported content type")
	ErrFileTooLarge      = apperr.New(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file exceeds the allowed size")
	ErrInvalidMediaKind  = apperr.New(http.StatusBadRequest, "INVALID_MEDIA_KIND", "media kind must be banner or logo")
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

var validEventTypes = map[string]bool{
	"marathon": true, "trail": true, "cycling": true, "triathlon": true,
	"funrun": true, "expo": true, "seminar": true, "concert": true, "other": true,
}
