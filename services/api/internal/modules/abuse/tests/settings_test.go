package abuse_test

import (
	"context"
	"testing"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/abuse"
)

type fakeSettingsRepo struct {
	rows []db.PlatformSetting
}

func (f *fakeSettingsRepo) ListSettings(ctx context.Context) ([]db.PlatformSetting, error) {
	return f.rows, nil
}

func TestSettings_IsEnabled(t *testing.T) {
	repo := &fakeSettingsRepo{rows: []db.PlatformSetting{
		{Key: "rate_limit_enabled", Value: "true"},
		{Key: "turnstile_enabled", Value: "false"},
	}}
	s := abuse.NewSettings(repo)
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !s.IsEnabled("rate_limit_enabled") {
		t.Error("rate_limit_enabled should be true")
	}
	if s.IsEnabled("turnstile_enabled") {
		t.Error("turnstile_enabled should be false")
	}
}

func TestSettings_FailSafeDefaults(t *testing.T) {
	s := abuse.NewSettings(&fakeSettingsRepo{})
	if !s.IsEnabled("rate_limit_enabled") {
		t.Error("rate_limit default should be ON (fail-safe)")
	}
	if !s.IsEnabled("blocklist_enabled") {
		t.Error("blocklist default should be ON (fail-safe)")
	}
	if !s.IsEnabled("ip_reputation_enabled") {
		t.Error("ip_reputation default should be ON (fail-safe)")
	}
	if s.IsEnabled("turnstile_enabled") {
		t.Error("turnstile default should be OFF")
	}
}
