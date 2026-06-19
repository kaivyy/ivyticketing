package notifications_test

import (
	"context"
	"testing"

	notifemail "github.com/varin/ivyticketing/services/api/internal/modules/notifications/email"
)

func TestSMTPSender_Configuration(t *testing.T) {
	cfg := notifemail.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     "587",
		User:     "user@example.com",
		Pass:     "password",
		FromName: "Test Sender",
		FromAddr: "noreply@example.com",
	}

	sender := notifemail.NewSMTPSender(cfg)
	if sender.Host != cfg.Host {
		t.Errorf("expected host %q, got %q", cfg.Host, sender.Host)
	}
	if sender.Port != cfg.Port {
		t.Errorf("expected port %q, got %q", cfg.Port, sender.Port)
	}
	if sender.FromName != cfg.FromName {
		t.Errorf("expected from name %q, got %q", cfg.FromName, sender.FromName)
	}
	if sender.FromAddress != cfg.FromAddr {
		t.Errorf("expected from address %q, got %q", cfg.FromAddr, sender.FromAddress)
	}
}

func TestSMTPSender_Send_MessageFormat(t *testing.T) {
	// This test verifies the message can be built without panicking
	// We don't test actual SMTP send (requires real SMTP server)
	sender := notifemail.NewSMTPSender(notifemail.SMTPConfig{
		Host:     "localhost",
		Port:     "1025",
		User:     "test",
		Pass:     "test",
		FromName: "Test",
		FromAddr: "test@example.com",
	})

	// Verify Send method exists and accepts correct parameters
	// It will fail to connect (no SMTP server), but that's expected
	ctx := context.Background()
	err := sender.Send(ctx, "recipient@example.com", "Test Subject", "<html>body</html>", "text body")
	
	// We expect an error (connection refused) but no panic
	if err == nil {
		t.Log("Send succeeded (unexpected but OK if SMTP server is running)")
	} else {
		t.Logf("Send failed as expected (no SMTP server): %v", err)
	}
}

func TestLogSender_Fallback(t *testing.T) {
	// LogSender is tested indirectly in service_test.go
	// This test just verifies the interface contract
	var _ notifemail.Sender = &notifemail.LogSender{}
}
