package email

import "log/slog"

// SenderConfig is the configuration consumed by NewSenderFromConfig to construct
// a Sender. Driver is "smtp" or "log"; any other value falls back to LogSender.
type SenderConfig struct {
	Driver      string
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	FromName    string
	FromAddress string
}

// NewSenderFromConfig returns an SMTPSender when driver=="smtp" and the required
// host/port are set. Otherwise it falls back to a LogSender and never panics.
func NewSenderFromConfig(cfg SenderConfig, log *slog.Logger) Sender {
	if cfg.Driver == "smtp" && cfg.SMTPHost != "" && cfg.SMTPPort != "" {
		log.Info("email driver: smtp", "host", cfg.SMTPHost, "port", cfg.SMTPPort)
		return NewSMTPSender(SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Pass:     cfg.SMTPPass,
			FromName: cfg.FromName,
			FromAddr: cfg.FromAddress,
		})
	}
	if cfg.Driver == "smtp" {
		log.Warn("email driver smtp but config incomplete, falling back to log")
	}
	return &LogSender{Log: log}
}