// Package qr signs and verifies ticket QR tokens using HMAC-SHA256.
// Payload carries only ticket_id, event_id, and a version — never PII.
package qr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// CurrentVersion is the QR payload schema/secret version.
const CurrentVersion = 1

// TicketRef is the data carried in a QR token.
type TicketRef struct {
	TicketID uuid.UUID
	EventID  uuid.UUID
	Version  int
}

type payload struct {
	TID string `json:"tid"`
	EID string `json:"eid"`
	V   int    `json:"v"`
}

// Signer signs and verifies QR tokens with a single HMAC secret.
type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

var enc = base64.RawURLEncoding

func (s *Signer) mac(versionAndPayload string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(versionAndPayload))
	return enc.EncodeToString(m.Sum(nil))
}

// Sign returns "<version>.<base64url(payload)>.<base64url(hmac)>".
func (s *Signer) Sign(ticketID, eventID uuid.UUID) (string, error) {
	body, err := json.Marshal(payload{TID: ticketID.String(), EID: eventID.String(), V: CurrentVersion})
	if err != nil {
		return "", err
	}
	verSeg := strconv.Itoa(CurrentVersion)
	payloadSeg := enc.EncodeToString(body)
	signingInput := verSeg + "." + payloadSeg
	return signingInput + "." + s.mac(signingInput), nil
}

// Verify checks the signature and returns the decoded TicketRef.
func (s *Signer) Verify(token string) (TicketRef, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TicketRef{}, fmt.Errorf("qr: malformed token")
	}
	verSeg, payloadSeg, sigSeg := parts[0], parts[1], parts[2]

	ver, err := strconv.Atoi(verSeg)
	if err != nil || ver != CurrentVersion {
		return TicketRef{}, fmt.Errorf("qr: unsupported version")
	}

	expected := s.mac(verSeg + "." + payloadSeg)
	if !hmac.Equal([]byte(expected), []byte(sigSeg)) {
		return TicketRef{}, fmt.Errorf("qr: invalid signature")
	}

	raw, err := enc.DecodeString(payloadSeg)
	if err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad payload encoding")
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad payload json")
	}
	tid, err := uuid.Parse(p.TID)
	if err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad ticket id")
	}
	eid, err := uuid.Parse(p.EID)
	if err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad event id")
	}
	return TicketRef{TicketID: tid, EventID: eid, Version: p.V}, nil
}
