package qr

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"
)

const testSecret = "property-test-secret-key"

// signSegments produces a token "<verSeg>.<payloadSeg>.<hmac>" whose signature
// is valid for the given version and payload segments. This lets a property
// test drive a structurally-malformed payload past Verify's HMAC gate so that
// the structural decode path (and its ErrMalformedToken) is exercised.
func signSegments(s *Signer, verSeg, payloadSeg string) string {
	signingInput := verSeg + "." + payloadSeg
	return signingInput + "." + s.mac(signingInput)
}

// uuidGen draws a random UUID from 16 random bytes so rapid controls the input
// space and can shrink counterexamples.
func uuidGen() *rapid.Generator[uuid.UUID] {
	return rapid.Custom(func(t *rapid.T) uuid.UUID {
		var b [16]byte
		for i := range b {
			b[i] = rapid.Byte().Draw(t, "uuidByte")
		}
		return uuid.UUID(b)
	})
}

// Feature: scanner-pwa, Property 1: QR verification round-trip
//
// For any pair of ticket_id and event_id, verifying the token produced by
// signing them returns a TicketRef whose ticket_id and event_id equal the
// originals (and whose Version is CurrentVersion).
//
// Validates: Requirements 2.2
func TestProperty_QRVerificationRoundTrip(t *testing.T) {
	s := NewSigner(testSecret)

	rapid.Check(t, func(t *rapid.T) {
		ticketID := uuidGen().Draw(t, "ticketID")
		eventID := uuidGen().Draw(t, "eventID")

		token, err := s.Sign(ticketID, eventID)
		if err != nil {
			t.Fatalf("Sign returned error: %v", err)
		}

		ref, err := s.Verify(token)
		if err != nil {
			t.Fatalf("Verify returned error for freshly signed token: %v", err)
		}

		if ref.TicketID != ticketID {
			t.Fatalf("TicketID mismatch: got %s, want %s", ref.TicketID, ticketID)
		}
		if ref.EventID != eventID {
			t.Fatalf("EventID mismatch: got %s, want %s", ref.EventID, eventID)
		}
		if ref.Version != CurrentVersion {
			t.Fatalf("Version = %d, want %d", ref.Version, CurrentVersion)
		}
	})
}

// Feature: scanner-pwa, Property 2: Tampered, malformed, or unsupported tokens are rejected without side effects
//
// For any validly signed token, any mutation of its payload or signature
// segment, any structurally malformed string, and any unsupported version SHALL
// cause verification to return an error; and no rejected token SHALL ever yield
// a valid (nil-error) TicketRef. Verify must never panic and must never return
// a nil error together with a populated TicketRef for a rejected token.
//
// Validates: Requirements 2.3, 2.4, 2.6
func TestProperty_TamperedMalformedUnsupportedRejected(t *testing.T) {
	s := NewSigner(testSecret)

	rapid.Check(t, func(t *rapid.T) {
		// Build a valid token to serve as the mutation base.
		ticketID := uuidGen().Draw(t, "ticketID")
		eventID := uuidGen().Draw(t, "eventID")
		token, err := s.Sign(ticketID, eventID)
		if err != nil {
			t.Fatalf("Sign returned error: %v", err)
		}
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatalf("unexpected token shape: %q", token)
		}

		// Choose which class of invalid input to exercise this iteration.
		mode := rapid.SampledFrom([]string{
			"tamperPayload",
			"tamperSignature",
			"malformedSegments",
			"nonBase64Payload",
			"badJSONPayload",
			"unparseableUUIDPayload",
			"unsupportedVersion",
		}).Draw(t, "mode")

		var bad string
		// expectUnsupportedVersion / expectMalformed drive which sentinel we
		// assert on for the structural (non-signature) cases.
		expectUnsupportedVersion := false
		expectMalformed := false

		switch mode {
		case "tamperPayload":
			// Flip one byte in the payload segment. HMAC no longer matches.
			b := []byte(parts[1])
			idx := rapid.IntRange(0, len(b)-1).Draw(t, "payloadIdx")
			b[idx] ^= byte(rapid.IntRange(1, 255).Draw(t, "payloadXor"))
			bad = parts[0] + "." + string(b) + "." + parts[2]

		case "tamperSignature":
			// Flip one byte in the signature segment.
			b := []byte(parts[2])
			idx := rapid.IntRange(0, len(b)-1).Draw(t, "sigIdx")
			b[idx] ^= byte(rapid.IntRange(1, 255).Draw(t, "sigXor"))
			bad = parts[0] + "." + parts[1] + "." + string(b)

		case "malformedSegments":
			// Wrong number of dot-separated segments (not exactly 3).
			expectMalformed = true
			segCount := rapid.SampledFrom([]int{0, 1, 2, 4, 5}).Draw(t, "segCount")
			segs := make([]string, segCount)
			for i := range segs {
				segs[i] = rapid.StringMatching(`[A-Za-z0-9_-]*`).Draw(t, "seg")
			}
			bad = strings.Join(segs, ".")

		case "nonBase64Payload":
			// Payload segment contains characters outside the base64url
			// alphabet (and no '.'), so base64 decode fails. It is validly
			// signed so it passes the HMAC gate and reaches structural decode.
			expectMalformed = true
			verSeg := strconv.Itoa(CurrentVersion)
			// '*' is not part of the base64url alphabet and is not '.'.
			payloadSeg := parts[1] + strings.Repeat("*", rapid.IntRange(1, 4).Draw(t, "stars"))
			bad = signSegments(s, verSeg, payloadSeg)

		case "badJSONPayload":
			// Payload decodes from base64url but is not a valid JSON object.
			// Validly signed so the HMAC gate passes and decode is reached.
			expectMalformed = true
			raw := rapid.SliceOfN(rapid.Byte(), 1, 12).Draw(t, "rawBytes")
			// Prepend a byte that cannot begin a JSON object/value we accept.
			junk := append([]byte{'x'}, raw...)
			verSeg := strconv.Itoa(CurrentVersion)
			payloadSeg := enc.EncodeToString(junk)
			bad = signSegments(s, verSeg, payloadSeg)

		case "unparseableUUIDPayload":
			// Well-formed JSON payload but ticket/event IDs are not UUIDs.
			// Validly signed so the HMAC gate passes and decode is reached.
			expectMalformed = true
			tid := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "badTID")
			eid := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "badEID")
			body := []byte(`{"tid":"` + tid + `","eid":"` + eid + `","v":` + strconv.Itoa(CurrentVersion) + `}`)
			verSeg := strconv.Itoa(CurrentVersion)
			payloadSeg := enc.EncodeToString(body)
			bad = signSegments(s, verSeg, payloadSeg)

		case "unsupportedVersion":
			// A version segment that is not the supported CurrentVersion.
			expectUnsupportedVersion = true
			ver := rapid.OneOf(
				rapid.Map(rapid.IntRange(2, 1000), strconv.Itoa),
				rapid.Map(rapid.IntRange(-1000, -1), strconv.Itoa),
				rapid.StringMatching(`[A-Za-z]{1,5}`),
			).Draw(t, "badVersion")
			bad = ver + "." + parts[1] + "." + parts[2]
		}

		// Verify must never panic. Any panic will fail the test naturally.
		ref, verr := s.Verify(bad)

		// Core invariant: a rejected token must never produce a nil error.
		if verr == nil {
			t.Fatalf("Verify accepted an invalid token (mode=%q): input=%q ref=%+v", mode, bad, ref)
		}

		// Sanity: on error the returned ref must be the zero value (no populated
		// TicketRef leaks alongside an error).
		if ref != (TicketRef{}) {
			t.Fatalf("Verify returned error but a populated TicketRef (mode=%q): ref=%+v", mode, ref)
		}

		// For the structural cases, assert the specific sentinel error contract.
		switch {
		case expectUnsupportedVersion:
			if !errors.Is(verr, ErrUnsupportedVersion) {
				t.Fatalf("mode=%q: expected ErrUnsupportedVersion, got %v", mode, verr)
			}
		case expectMalformed:
			if !errors.Is(verr, ErrMalformedToken) {
				t.Fatalf("mode=%q: expected ErrMalformedToken, got %v", mode, verr)
			}
		}
		// For tamperPayload / tamperSignature the error will be
		// ErrInvalidSignature (or a structural error if the mutation also broke
		// structure); either way a non-nil error is sufficient and already
		// asserted above.
	})
}
