package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP (RFC 6238) with the settings every authenticator app supports:
// SHA-1, 6 digits, 30-second steps.
const (
	totpPeriod = 30
	totpDigits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random 160-bit secret, base32 encoded.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// TOTPStep is the 30-second step that t falls in.
func TOTPStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// TOTPCode returns the code for a step.
func TOTPCode(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, n%1_000_000), nil
}

// VerifyTOTP checks a code, allowing one step of clock drift either way. It
// returns the step the code matched, which must be later than lastStep so an
// accepted code can't be used again.
func VerifyTOTP(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return 0, false
	}
	cur := TOTPStep(now)
	for _, step := range []int64{cur, cur - 1, cur + 1} {
		if step <= lastStep {
			continue
		}
		want, err := TOTPCode(secret, step)
		if err == nil && hmac.Equal([]byte(want), []byte(code)) {
			return step, true
		}
	}
	return 0, false
}

// TOTPURI is the otpauth:// link authenticator apps read from the QR code.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Recovery codes avoid look-alike characters (0/o, 1/l/i).
const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// NewRecoveryCodes returns n one-time codes like "k7m2p-xq4ra".
func NewRecoveryCodes(n int) ([]string, error) {
	codes := make([]string, n)
	buf := make([]byte, 10)
	for i := range codes {
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		var sb strings.Builder
		for j, b := range buf {
			if j == 5 {
				sb.WriteByte('-')
			}
			// 256 % 31 leaves a slight bias; irrelevant at 50 bits per code.
			sb.WriteByte(recoveryAlphabet[int(b)%len(recoveryAlphabet)])
		}
		codes[i] = sb.String()
	}
	return codes, nil
}

// HashRecoveryCode normalises a typed recovery code (case, spaces, dashes)
// and hashes it for storage or lookup.
func HashRecoveryCode(code string) string {
	c := strings.ToLower(code)
	c = strings.NewReplacer("-", "", " ", "").Replace(c)
	return HashToken(c)
}
