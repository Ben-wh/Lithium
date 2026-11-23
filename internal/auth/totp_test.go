package auth

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

func TestTOTPRFCVectors(t *testing.T) {
	// RFC 6238 appendix B (SHA-1), truncated to 6 digits.
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := TOTPCode(secret, ts/30)
		if err != nil || got != want {
			t.Errorf("t=%d: got %q, want %q (%v)", ts, got, want, err)
		}
	}
}

func TestVerifyTOTP(t *testing.T) {
	secret, _ := NewTOTPSecret()
	now := time.Unix(1_800_000_000, 0)
	step := TOTPStep(now)
	code, _ := TOTPCode(secret, step)
	if got, ok := VerifyTOTP(secret, code, now, 0); !ok || got != step {
		t.Fatal("current code rejected")
	}
	if _, ok := VerifyTOTP(secret, code, now, step); ok {
		t.Fatal("replayed code accepted")
	}
	prev, _ := TOTPCode(secret, step-1)
	if _, ok := VerifyTOTP(secret, prev, now, 0); !ok {
		t.Fatal("previous step (clock drift) rejected")
	}
	old, _ := TOTPCode(secret, step-3)
	if _, ok := VerifyTOTP(secret, old, now, 0); ok {
		t.Fatal("stale code accepted")
	}
	if _, ok := VerifyTOTP(secret, "12345", now, 0); ok {
		t.Fatal("short code accepted")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, err := NewRecoveryCodes(10)
	if err != nil || len(codes) != 10 {
		t.Fatal(err)
	}
	c := codes[0]
	if len(c) != 11 || c[5] != '-' {
		t.Fatalf("bad format %q", c)
	}
	if HashRecoveryCode(strings.ToUpper(strings.ReplaceAll(c, "-", " "))) != HashRecoveryCode(c) {
		t.Fatal("normalisation differs")
	}
}
