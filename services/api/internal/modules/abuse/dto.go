package abuse

import "time"

type SettingDTO struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type BlockRequest struct {
	SubjectType  string  `json:"subjectType"`
	SubjectValue string  `json:"subjectValue"`
	Reason       string  `json:"reason"`
	ExpiresAt    *string `json:"expiresAt"`
}

type UnblockRequest struct {
	SubjectType  string `json:"subjectType"`
	SubjectValue string `json:"subjectValue"`
}

type BlockedDTO struct {
	SubjectType  string     `json:"subjectType"`
	SubjectValue string     `json:"subjectValue"`
	Reason       string     `json:"reason,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

type IPRuleRequest struct {
	CIDR string `json:"cidr"`
	Rule string `json:"rule"`
	Note string `json:"note"`
}

type AbuseLogDTO struct {
	Action       string    `json:"action"`
	Category     string    `json:"category,omitempty"`
	SubjectType  string    `json:"subjectType,omitempty"`
	SubjectValue string    `json:"subjectValue,omitempty"`
	IP           string    `json:"ip,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type SecurityConfigDTO struct {
	TurnstileEnabled bool   `json:"turnstileEnabled"`
	SiteKey          string `json:"siteKey,omitempty"`
}
