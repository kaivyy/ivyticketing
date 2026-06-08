package abuse

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// AuditRecorder is the narrow interface Service needs from audit.Logger.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// Service provides admin operations, queue-cap enforcement, and abuse logging.
type Service struct {
	repo        Repository
	settings    *Settings
	audit       AuditRecorder
	maxQueueCap int
	siteKey     string
}

func NewService(repo Repository, settings *Settings, recorder AuditRecorder, maxQueueCap int, siteKey string) *Service {
	return &Service{
		repo:        repo,
		settings:    settings,
		audit:       recorder,
		maxQueueCap: maxQueueCap,
		siteKey:     siteKey,
	}
}

// Log implements AbuseLogger. Called by the guard; failures are silently dropped.
func (s *Service) Log(ctx context.Context, e AbuseEvent) {
	_ = s.repo.InsertAbuseLog(ctx, db.InsertAbuseLogParams{
		SubjectType:  pgtype.Text{String: e.SubjectType, Valid: e.SubjectType != ""},
		SubjectValue: pgtype.Text{String: e.SubjectValue, Valid: e.SubjectValue != ""},
		Action:       e.Action,
		Category:     pgtype.Text{String: e.Category, Valid: e.Category != ""},
		Fingerprint:  pgtype.Text{String: e.Fingerprint, Valid: e.Fingerprint != ""},
		Ip:           pgtype.Text{String: e.IP, Valid: e.IP != ""},
		UserID:       e.UserID,
		Detail:       nil,
	})
}

// WithinQueueCap implements QueueCapChecker. Fails open on DB error.
func (s *Service) WithinQueueCap(ctx context.Context, userID uuid.UUID) (bool, error) {
	if s.maxQueueCap <= 0 {
		return true, nil
	}
	n, err := s.repo.CountActiveQueueTokensByUser(ctx, userID)
	if err != nil {
		return true, nil // fail-open
	}
	return n < int64(s.maxQueueCap), nil
}

// SecurityConfig returns the public-facing Turnstile configuration.
func (s *Service) SecurityConfig() SecurityConfigDTO {
	return SecurityConfigDTO{
		TurnstileEnabled: s.settings.IsEnabled(SettingTurnstileEnabled),
		SiteKey:          s.siteKey,
	}
}

// SetSetting upserts a platform setting and refreshes the in-memory cache.
func (s *Service) SetSetting(ctx context.Context, actor uuid.UUID, key, value string) error {
	if !validSettingKey(key) {
		return ErrInvalidSetting
	}
	_, err := s.repo.UpsertSetting(ctx, db.UpsertPlatformSettingParams{
		Key:       key,
		Value:     value,
		UpdatedBy: &actor,
	})
	if err != nil {
		return err
	}
	_ = s.settings.Refresh(ctx)
	s.record(ctx, &actor, "ABUSE_SETTING_CHANGED", "platform_setting", key, map[string]any{"value": value})
	return nil
}

// Block upserts a blocked subject entry.
func (s *Service) Block(ctx context.Context, actor uuid.UUID, req BlockRequest) error {
	if req.SubjectType != SubjectUser && req.SubjectType != SubjectIP {
		return ErrInvalidSetting
	}
	var exp pgtype.Timestamptz
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return ErrInvalidSetting
		}
		exp = pgtype.Timestamptz{Time: t, Valid: true}
	}
	_, err := s.repo.UpsertBlockedSubject(ctx, db.UpsertBlockedSubjectParams{
		SubjectType:  req.SubjectType,
		SubjectValue: req.SubjectValue,
		Reason:       pgtype.Text{String: req.Reason, Valid: req.Reason != ""},
		BlockedBy:    &actor,
		ExpiresAt:    exp,
	})
	if err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_BLOCK_SET", req.SubjectType, req.SubjectValue, map[string]any{"reason": req.Reason})
	return nil
}

// Unblock removes a blocked subject entry.
func (s *Service) Unblock(ctx context.Context, actor uuid.UUID, req UnblockRequest) error {
	if err := s.repo.DeleteBlockedSubject(ctx, db.DeleteBlockedSubjectParams{
		SubjectType:  req.SubjectType,
		SubjectValue: req.SubjectValue,
	}); err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_UNBLOCK", req.SubjectType, req.SubjectValue, nil)
	return nil
}

// ListSettings returns all platform settings as DTOs.
func (s *Service) ListSettings(ctx context.Context) ([]SettingDTO, error) {
	rows, err := s.repo.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SettingDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, SettingDTO{Key: r.Key, Value: r.Value})
	}
	return out, nil
}

// ListBlocked returns paginated blocked subjects.
func (s *Service) ListBlocked(ctx context.Context, limit, offset int32) ([]BlockedDTO, error) {
	rows, err := s.repo.ListBlockedSubjects(ctx, db.ListBlockedSubjectsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]BlockedDTO, 0, len(rows))
	for _, r := range rows {
		d := BlockedDTO{
			SubjectType:  r.SubjectType,
			SubjectValue: r.SubjectValue,
			CreatedAt:    r.CreatedAt.Time,
		}
		if r.Reason.Valid {
			d.Reason = r.Reason.String
		}
		if r.ExpiresAt.Valid {
			e := r.ExpiresAt.Time
			d.ExpiresAt = &e
		}
		out = append(out, d)
	}
	return out, nil
}

// ListAbuseLog returns paginated abuse log entries.
func (s *Service) ListAbuseLog(ctx context.Context, limit, offset int32) ([]AbuseLogDTO, error) {
	rows, err := s.repo.ListAbuseLog(ctx, db.ListAbuseLogParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]AbuseLogDTO, 0, len(rows))
	for _, r := range rows {
		d := AbuseLogDTO{
			Action:    r.Action,
			CreatedAt: r.CreatedAt.Time,
		}
		if r.Category.Valid {
			d.Category = r.Category.String
		}
		if r.SubjectType.Valid {
			d.SubjectType = r.SubjectType.String
		}
		if r.SubjectValue.Valid {
			d.SubjectValue = r.SubjectValue.String
		}
		if r.Ip.Valid {
			d.IP = r.Ip.String
		}
		out = append(out, d)
	}
	return out, nil
}

// ListIPRules returns all IP rules.
func (s *Service) ListIPRules(ctx context.Context) ([]db.IpRule, error) {
	return s.repo.ListIPRules(ctx)
}

// AddIPRule inserts a new CIDR rule.
func (s *Service) AddIPRule(ctx context.Context, actor uuid.UUID, req IPRuleRequest) error {
	if req.Rule != "allow" && req.Rule != "deny" {
		return ErrInvalidSetting
	}
	_, err := s.repo.CreateIPRule(ctx, db.CreateIPRuleParams{
		Cidr:      req.CIDR,
		Rule:      req.Rule,
		Note:      pgtype.Text{String: req.Note, Valid: req.Note != ""},
		CreatedBy: &actor,
	})
	if err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_IP_RULE_ADDED", "ip_rule", req.CIDR, map[string]any{"rule": req.Rule})
	return nil
}

// DeleteIPRule removes an IP rule by ID.
func (s *Service) DeleteIPRule(ctx context.Context, actor, id uuid.UUID) error {
	if err := s.repo.DeleteIPRule(ctx, id); err != nil {
		return err
	}
	s.record(ctx, &actor, "ABUSE_IP_RULE_REMOVED", "ip_rule", id.String(), nil)
	return nil
}

// record writes an audit entry; silently skips if no recorder is configured.
func (s *Service) record(ctx context.Context, actor *uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: nil,
		ActorUserID:    actor,
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		Metadata:       meta,
	})
}

// validSettingKey reports whether the key is a known platform setting.
func validSettingKey(key string) bool {
	switch key {
	case SettingTurnstileEnabled, SettingRateLimitEnabled, SettingIPReputationEnabled, SettingBlocklistEnabled:
		return true
	}
	return false
}
