package sms

import (
	"sync"
	"time"
)

// IdempotencyStore defines key lookup and storage to prevent duplicate message dispatches.
type IdempotencyStore interface {
	Get(key string) (DeliveryResult, bool)
	Set(key string, res DeliveryResult, ttl time.Duration)
}

type memoryIdempotencyItem struct {
	result    DeliveryResult
	expiresAt time.Time
}

// MemoryIdempotencyStore provides an in-memory thread-safe implementation of IdempotencyStore.
type MemoryIdempotencyStore struct {
	mu    sync.RWMutex
	items map[string]memoryIdempotencyItem
}

// NewMemoryIdempotencyStore initializes an in-memory idempotency cache.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	store := &MemoryIdempotencyStore{
		items: make(map[string]memoryIdempotencyItem),
	}
	go store.cleanupLoop()
	return store
}

func (s *MemoryIdempotencyStore) Get(key string) (DeliveryResult, bool) {
	if key == "" {
		return DeliveryResult{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.items[key]
	if !exists {
		return DeliveryResult{}, false
	}
	if time.Now().After(item.expiresAt) {
		return DeliveryResult{}, false
	}
	return item.result, true
}

func (s *MemoryIdempotencyStore) Set(key string, res DeliveryResult, ttl time.Duration) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = memoryIdempotencyItem{
		result:    res,
		expiresAt: time.Now().Add(ttl),
	}
}

func (s *MemoryIdempotencyStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.items {
			if now.After(v.expiresAt) {
				delete(s.items, k)
			}
		}
		s.mu.Unlock()
	}
}
