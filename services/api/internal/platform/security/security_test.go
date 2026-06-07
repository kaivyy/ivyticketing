package security

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPassword_HashVerify(t *testing.T) {
	hash, err := HashPassword("s3cret-pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "s3cret-pw" {
		t.Fatal("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "s3cret-pw") {
		t.Error("correct password should verify")
	}
	if VerifyPassword(hash, "wrong-pw") {
		t.Error("wrong password should not verify")
	}
}

func TestJWT_SignVerify(t *testing.T) {
	signer := NewJWTSigner("test-secret", time.Minute)
	uid := uuid.New()

	tok, err := signer.Sign(uid, true)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := signer.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("UserID = %v, want %v", claims.UserID, uid)
	}
	if !claims.IsPlatformAdmin {
		t.Error("IsPlatformAdmin = false, want true")
	}
}

func TestJWT_RejectExpired(t *testing.T) {
	signer := NewJWTSigner("test-secret", -time.Minute) // already expired
	tok, err := signer.Sign(uuid.New(), false)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := signer.Verify(tok); err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestJWT_RejectWrongSecret(t *testing.T) {
	tok, err := NewJWTSigner("secret-a", time.Minute).Sign(uuid.New(), false)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewJWTSigner("secret-b", time.Minute).Verify(tok); err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestRefreshToken_RawDiffersFromHash(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if raw == hash {
		t.Fatal("raw token must differ from its hash")
	}
	if HashToken(raw) != hash {
		t.Error("HashToken(raw) should equal returned hash")
	}
	if HashToken("other") == hash {
		t.Error("different input should not match hash")
	}
}
