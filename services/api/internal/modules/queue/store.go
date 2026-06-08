package queue

import (
	"context"

	pq "github.com/varin/ivyticketing/services/api/internal/platform/queue"
)

type Store struct {
	a *pq.Adapter
}

func NewStore(a *pq.Adapter) *Store { return &Store{a: a} }

func (s *Store) AddWaiting(ctx context.Context, eventID, participantID string, score int64) error {
	return s.a.AddWaiting(ctx, eventID, participantID, score)
}
func (s *Store) Rank(ctx context.Context, eventID, participantID string) (int64, error) {
	return s.a.WaitingRank(ctx, eventID, participantID)
}
func (s *Store) RangeN(ctx context.Context, eventID string, n int64) ([]string, error) {
	return s.a.WaitingRangeN(ctx, eventID, n)
}
func (s *Store) MoveToAllowed(ctx context.Context, eventID, participantID string, expiresUnix int64) error {
	return s.a.MoveToAllowed(ctx, eventID, participantID, expiresUnix)
}
func (s *Store) MoveToWaiting(ctx context.Context, eventID, participantID string, score int64) error {
	return s.a.MoveToWaiting(ctx, eventID, participantID, score)
}
func (s *Store) RemoveAllowed(ctx context.Context, eventID, participantID string) error {
	return s.a.RemoveAllowed(ctx, eventID, participantID)
}
func (s *Store) WaitingCount(ctx context.Context, eventID string) (int64, error) {
	return s.a.WaitingCount(ctx, eventID)
}
func (s *Store) AllowedCount(ctx context.Context, eventID string) (int64, error) {
	return s.a.AllowedCount(ctx, eventID)
}
