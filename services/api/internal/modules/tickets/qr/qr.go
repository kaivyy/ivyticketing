// Package qr signs and verifies ticket QR tokens using HMAC-SHA256.
// Payload carries only ticket_id, event_id, and a version — never PII.
package qr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// CurrentVersion is the QR payload schema/secret version.
const CurrentVersion = 1

// Sentinel errors returned by Verify and DecodeStructure. Callers may use
// errors.Is to distinguish rejection reasons. Note that DecodeStructure never
// returns ErrInvalidSignature because it does not check the HMAC.
var (
	// ErrMalformedToken is returned when the token does not have the expected
	// "<version>.<payload>.<signature>" structure, or the payload segment is
	// not a base64url-decodable JSON object with parseable ticket/event IDs.
	ErrMalformedToken = errors.New("qr: malformed token")
	// ErrUnsupportedVersion is returned when the token's version segment is not
	// a supported schema version.
	ErrUnsupportedVersion = errors.New("qr: unsupported version")
	// ErrInvalidSignature is returned when the HMAC signature does not match.
	ErrInvalidSignature = errors.New("qr: invalid signature")
)

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

// splitToken splits a token into its version, payload, and signature segments,
// returning ErrMalformedToken if the shape is wrong.
func splitToken(token string) (verSeg, payloadSeg, sigSeg string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", "", ErrMalformedToken
	}
	return parts[0], parts[1], parts[2], nil
}

// decodePayload validates the version segment and decodes the base64url payload
// into a TicketRef. It performs NO signature check, so it never returns
// ErrInvalidSignature. It is the shared structural core of Verify and
// DecodeStructure.
func decodePayload(verSeg, payloadSeg string) (TicketRef, error) {
	ver, err := strconv.Atoi(verSeg)
	if err != nil || ver != CurrentVersion {
		return TicketRef{}, ErrUnsupportedVersion
	}

	raw, err := enc.DecodeString(payloadSeg)
	if err != nil {
		return TicketRef{}, fmt.Errorf("%w: bad payload encoding", ErrMalformedToken)
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return TicketRef{}, fmt.Errorf("%w: bad payload json", ErrMalformedToken)
	}
	tid, err := uuid.Parse(p.TID)
	if err != nil {
		return TicketRef{}, fmt.Errorf("%w: bad ticket id", ErrMalformedToken)
	}
	eid, err := uuid.Parse(p.EID)
	if err != nil {
		return TicketRef{}, fmt.Errorf("%w: bad event id", ErrMalformedToken)
	}
	return TicketRef{TicketID: tid, EventID: eid, Version: p.V}, nil
}

// Verify checks the HMAC signature and returns the decoded TicketRef. On
// failure it returns one of the sentinel errors ErrMalformedToken,
// ErrUnsupportedVersion, or ErrInvalidSignature (wrapped), distinguishable via
// errors.Is.
func (s *Signer) Verify(token string) (TicketRef, error) {
	verSeg, payloadSeg, sigSeg, err := splitToken(token)
	if err != nil {
		return TicketRef{}, err
	}

	// Reject unsupported versions before spending work on the MAC.
	if ver, err := strconv.Atoi(verSeg); err != nil || ver != CurrentVersion {
		return TicketRef{}, ErrUnsupportedVersion
	}

	expected := s.mac(verSeg + "." + payloadSeg)
	if !hmac.Equal([]byte(expected), []byte(sigSeg)) {
		return TicketRef{}, ErrInvalidSignature
	}

	return decodePayload(verSeg, payloadSeg)
}

// DecodeStructure parses a token's segments, version, and base64url payload into
// a TicketRef WITHOUT verifying the HMAC signature. It requires no secret and is
// the authoritative reference for the client's offline structural validation
// contract: it accepts a token exactly when it is well-formed and carries a
// supported version and parseable ticket/event IDs.
//
// It returns the same structural sentinel errors as Verify (ErrMalformedToken,
// ErrUnsupportedVersion) but never ErrInvalidSignature, since no signature check
// is performed. A token that passes DecodeStructure is only provisionally valid;
// the server-side Verify (which holds the HMAC secret) remains the authoritative
// signature gate.
func DecodeStructure(token string) (TicketRef, error) {
	verSeg, payloadSeg, _, err := splitToken(token)
	if err != nil {
		return TicketRef{}, err
	}
	return decodePayload(verSeg, payloadSeg)
}
