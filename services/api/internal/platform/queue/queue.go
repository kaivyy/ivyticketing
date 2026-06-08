// Package queue provides Redis sorted-set primitives for the waiting room.
// Postgres is the durable source of truth; these structures are rebuildable.
package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Adapter struct {
	c *redis.Client
}

func New(c *redis.Client) *Adapter { return &Adapter{c: c} }

func waitingKey(eventID string) string { return fmt.Sprintf("queue:%s:waiting", eventID) }
func allowedKey(eventID string) string { return fmt.Sprintf("queue:%s:allowed", eventID) }

func (a *Adapter) AddWaiting(ctx context.Context, eventID, member string, score int64) error {
	return a.c.ZAdd(ctx, waitingKey(eventID), redis.Z{Score: float64(score), Member: member}).Err()
}

func (a *Adapter) WaitingRank(ctx context.Context, eventID, member string) (int64, error) {
	return a.c.ZRank(ctx, waitingKey(eventID), member).Result()
}

func (a *Adapter) WaitingRangeN(ctx context.Context, eventID string, n int64) ([]string, error) {
	return a.c.ZRange(ctx, waitingKey(eventID), 0, n-1).Result()
}

func (a *Adapter) WaitingCount(ctx context.Context, eventID string) (int64, error) {
	return a.c.ZCard(ctx, waitingKey(eventID)).Result()
}

func (a *Adapter) AllowedCount(ctx context.Context, eventID string) (int64, error) {
	return a.c.ZCard(ctx, allowedKey(eventID)).Result()
}

func (a *Adapter) MoveToAllowed(ctx context.Context, eventID, member string, expiresAtUnix int64) error {
	pipe := a.c.TxPipeline()
	pipe.ZRem(ctx, waitingKey(eventID), member)
	pipe.ZAdd(ctx, allowedKey(eventID), redis.Z{Score: float64(expiresAtUnix), Member: member})
	_, err := pipe.Exec(ctx)
	return err
}

func (a *Adapter) MoveToWaiting(ctx context.Context, eventID, member string, score int64) error {
	pipe := a.c.TxPipeline()
	pipe.ZRem(ctx, allowedKey(eventID), member)
	pipe.ZAdd(ctx, waitingKey(eventID), redis.Z{Score: float64(score), Member: member})
	_, err := pipe.Exec(ctx)
	return err
}

func (a *Adapter) RemoveAllowed(ctx context.Context, eventID, member string) error {
	return a.c.ZRem(ctx, allowedKey(eventID), member).Err()
}
