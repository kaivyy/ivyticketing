package abuse

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// BlocklistReader is the minimal repo surface the blocklist needs.
type BlocklistReader interface {
	GetBlockedSubject(ctx context.Context, arg db.GetBlockedSubjectParams) (db.BlockedSubject, error)
	ListIPRules(ctx context.Context) ([]db.IpRule, error)
}

type Blocklist struct {
	repo BlocklistReader
}

func NewBlocklist(repo BlocklistReader) *Blocklist { return &Blocklist{repo: repo} }

// IsBlocked reports whether the user or IP is blocked. An explicit allow ip_rule
// wins over a deny rule. userID may be empty (unauthenticated).
func (b *Blocklist) IsBlocked(ctx context.Context, userID, ip string) bool {
	// ip_rules: allow wins
	rules, err := b.repo.ListIPRules(ctx)
	if err == nil {
		allowed, denied := matchIPRules(rules, ip)
		if allowed {
			return false
		}
		if denied {
			return true
		}
	}
	if userID != "" {
		if _, err := b.repo.GetBlockedSubject(ctx, db.GetBlockedSubjectParams{SubjectType: SubjectUser, SubjectValue: userID}); err == nil {
			return true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			// on DB error, fail-safe: do not block
		}
	}
	if ip != "" {
		if _, err := b.repo.GetBlockedSubject(ctx, db.GetBlockedSubjectParams{SubjectType: SubjectIP, SubjectValue: ip}); err == nil {
			return true
		}
	}
	return false
}

// matchIPRules returns (allowed, denied) for the given IP against the rules.
func matchIPRules(rules []db.IpRule, ip string) (allowed, denied bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false, false
	}
	for _, rule := range rules {
		if !cidrContains(rule.Cidr, parsed) {
			continue
		}
		switch rule.Rule {
		case "allow":
			allowed = true
		case "deny":
			denied = true
		}
	}
	return allowed, denied
}

// cidrContains handles both bare IPs and CIDR notation.
func cidrContains(cidr string, ip net.IP) bool {
	if _, network, err := net.ParseCIDR(cidr); err == nil {
		return network.Contains(ip)
	}
	if single := net.ParseIP(cidr); single != nil {
		return single.Equal(ip)
	}
	return false
}
