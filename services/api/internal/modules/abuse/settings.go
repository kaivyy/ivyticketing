package abuse

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// SettingsReader is the minimal repo surface the settings cache needs.
type SettingsReader interface {
	ListSettings(ctx context.Context) ([]db.PlatformSetting, error)
}

// failSafeDefaults: protective features ON by default; turnstile OFF (won't block
// users when no secret configured).
var failSafeDefaults = map[string]bool{
	SettingRateLimitEnabled:    true,
	SettingBlocklistEnabled:    true,
	SettingIPReputationEnabled: true,
	SettingTurnstileEnabled:    false,
	SettingCodeBruteForceBlock: true,
}

// numericDefaults: default integer values for numeric settings (returned when key is absent).
var numericDefaults = map[string]int{
	SettingCodeBruteForceWindow:   60,  // seconds
	SettingCodeBruteForceMaxTries: 3,
	SettingCodeBruteForceBlockDur: 600, // seconds
}

// Settings is an in-memory cache of platform_settings, refreshed periodically.
type Settings struct {
	repo SettingsReader
	mu   sync.RWMutex
	vals map[string]string
}

func NewSettings(repo SettingsReader) *Settings {
	return &Settings{repo: repo, vals: map[string]string{}}
}

// Refresh reloads all settings from the DB into the cache.
func (s *Settings) Refresh(ctx context.Context) error {
	rows, err := s.repo.ListSettings(ctx)
	if err != nil {
		return err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		m[row.Key] = row.Value
	}
	s.mu.Lock()
	s.vals = m
	s.mu.Unlock()
	return nil
}

// IsEnabled returns whether a boolean feature toggle is on. Missing key →
// fail-safe default (protective features ON, turnstile OFF).
func (s *Settings) IsEnabled(key string) bool {
	s.mu.RLock()
	v, ok := s.vals[key]
	s.mu.RUnlock()
	if !ok {
		return failSafeDefaults[key]
	}
	return v == "true"
}

// Get returns the raw string value (empty if unset).
func (s *Settings) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vals[key]
}

// GetInt returns the integer value for key. Falls back to numericDefaults, then 0.
func (s *Settings) GetInt(key string) int {
	s.mu.RLock()
	v, ok := s.vals[key]
	s.mu.RUnlock()
	if ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	if d, ok := numericDefaults[key]; ok {
		return d
	}
	return 0
}

// StartRefresh launches a background ticker refreshing the cache until ctx is done.
func (s *Settings) StartRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = s.Refresh(ctx)
			}
		}
	}()
}
