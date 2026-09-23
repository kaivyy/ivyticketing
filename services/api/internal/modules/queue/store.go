package queue

import (
	"context"
	"time"

	pq "github.com/varin/ivyticketing/services/api/internal/platform/queue"
)

type Store struct {
	a *pq.Adapter
}

func NewStore(a *pq.Adapter) *Store { return &Store{a: a} }

func (s *Store) AddWaiting(ctx context.Context, eventID, participantID string, score int64) error {
	if s == nil || s.a == nil {
		return nil
	}
	return s.a.AddWaiting(ctx, eventID, participantID, score)
}
func (s *Store) Rank(ctx context.Context, eventID, participantID string) (int64, error) {
	if s == nil || s.a == nil {
		return 0, nil
	}
	return s.a.WaitingRank(ctx, eventID, participantID)
}
func (s *Store) RangeN(ctx context.Context, eventID string, n int64) ([]string, error) {
	if s == nil || s.a == nil {
		return nil, nil
	}
	return s.a.WaitingRangeN(ctx, eventID, n)
}
func (s *Store) MoveToAllowed(ctx context.Context, eventID, participantID string, expiresUnix int64) error {
	if s == nil || s.a == nil {
		return nil
	}
	return s.a.MoveToAllowed(ctx, eventID, participantID, expiresUnix)
}
func (s *Store) MoveToWaiting(ctx context.Context, eventID, participantID string, score int64) error {
	if s == nil || s.a == nil {
		return nil
	}
	return s.a.MoveToWaiting(ctx, eventID, participantID, score)
}
func (s *Store) RemoveAllowed(ctx context.Context, eventID, participantID string) error {
	if s == nil || s.a == nil {
		return nil
	}
	return s.a.RemoveAllowed(ctx, eventID, participantID)
}
func (s *Store) WaitingCount(ctx context.Context, eventID string) (int64, error) {
	if s == nil || s.a == nil {
		return 0, nil
	}
	return s.a.WaitingCount(ctx, eventID)
}
func (s *Store) AllowedCount(ctx context.Context, eventID string) (int64, error) {
	if s == nil || s.a == nil {
		return 0, nil
	}
	return s.a.AllowedCount(ctx, eventID)
}
func (s *Store) GetCachedStatus(ctx context.Context, eventID, participantID string) (string, error) {
	if s == nil || s.a == nil {
		return "", nil
	}
	return s.a.GetCachedStatus(ctx, eventID, participantID)
}
func (s *Store) SetCachedStatus(ctx context.Context, eventID, participantID string, val string, ttl time.Duration) error {
	if s == nil || s.a == nil {
		return nil
	}
	return s.a.SetCachedStatus(ctx, eventID, participantID, val, ttl)
}
func (s *Store) InvalidateCachedStatus(ctx context.Context, eventID, participantID string) error {
	if s == nil || s.a == nil {
		return nil
	}
	return s.a.InvalidateCachedStatus(ctx, eventID, participantID)
}
