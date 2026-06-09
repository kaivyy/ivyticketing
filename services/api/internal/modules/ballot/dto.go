package ballot

import "time"

type CreateDrawRequest struct {
	Quota               int32     `json:"quota"`
	WaitlistSize        int32     `json:"waitlistSize"`
	PaymentWindowHours  int32     `json:"paymentWindowHours"`
	ApplicationOpensAt  time.Time `json:"applicationOpensAt"`
	ApplicationClosesAt time.Time `json:"applicationClosesAt"`
}

type BallotDrawDTO struct {
	ID                  string     `json:"id"`
	Status              string     `json:"status"`
	Quota               int32      `json:"quota"`
	WaitlistSize        int32      `json:"waitlistSize"`
	ApplicationOpensAt  time.Time  `json:"applicationOpensAt"`
	ApplicationClosesAt time.Time  `json:"applicationClosesAt"`
	DrawAt              *time.Time `json:"drawAt,omitempty"`
	AnnouncedAt         *time.Time `json:"announcedAt,omitempty"`
	Seed                *string    `json:"seed,omitempty"`
}

type DrawResultDTO struct {
	Rank          int    `json:"rank"`
	Outcome       string `json:"outcome"`
	ParticipantID string `json:"participantId"`
	ResultHash    string `json:"resultHash"`
}

type BallotEntryDTO struct {
	ID              string     `json:"id"`
	Status          string     `json:"status"`
	AppliedAt       time.Time  `json:"appliedAt"`
	PaymentDeadline *time.Time `json:"paymentDeadline,omitempty"`
}

// ApplyRequest is the body for POST .../ballot/apply.
type ApplyRequest struct {
	DrawID string `json:"draw_id"`
}

// BallotEntryResponse is returned to the participant for all entry reads.
type BallotEntryResponse struct {
	ID              string     `json:"id"`
	DrawID          string     `json:"draw_id"`
	Status          string     `json:"status"`
	WaitlistRank    *int32     `json:"waitlist_rank,omitempty"`
	PaymentDeadline *time.Time `json:"payment_deadline,omitempty"`
	ConvertedAt     *time.Time `json:"converted_at,omitempty"`
	// AccessGrantID is the admission token a WINNER must pass as X-Queue-Token
	// when creating a checkout order. Nil for non-winners.
	AccessGrantID *string `json:"access_grant_id,omitempty"`
}
