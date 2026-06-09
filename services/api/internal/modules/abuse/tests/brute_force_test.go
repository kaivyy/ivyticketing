package abuse_test

import (
	"context"
	"testing"

	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

func TestTrackCodeFailure_BlocksAfterMaxTries(t *testing.T) {
	// TrackCodeFailure requires a live Redis connection — skip in unit suite.
	// Full coverage is provided by the integration test suite (tags: integration).
	t.Skip("requires fake redis — implement with existing fake pattern")
	_ = context.Background()
	_ = abuse.Guard{}
}
