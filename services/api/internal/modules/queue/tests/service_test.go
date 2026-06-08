package queue_test

import (
	"testing"
	"time"

	"github.com/varin/ivyticketing/services/api/internal/modules/queue"
)

func TestFifoScore_UsedInJoin(t *testing.T) {
	// Verify FifoScore is monotonically increasing across calls spaced in time
	s1 := queue.FifoScore(time.Now())
	time.Sleep(time.Millisecond)
	s2 := queue.FifoScore(time.Now())
	if s2 <= s1 {
		t.Fatalf("scores not monotonic: %d >= %d", s2, s1)
	}
}
