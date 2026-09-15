package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func MAC(secret, value string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func Equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var codeEncoding = base32.NewEncoding(alphabet).WithPadding(base32.NoPadding)

func PassCode(id, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte("campus-pass:v1:" + id))
	raw := codeEncoding.EncodeToString(h.Sum(nil)[:20])
	groups := make([]string, 0, 8)
	for i := 0; i < len(raw); i += 4 {
		groups = append(groups, raw[i:i+4])
	}
	return strings.Join(groups, "-")
}
func NormalizeCode(input string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(input))
	s = strings.TrimPrefix(s, "CAMPUS:")
	s = strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(s)
	if len(s) != 32 {
		return "", fmt.Errorf("код должен содержать 32 буквы и цифры")
	}
	if _, err := codeEncoding.DecodeString(s); err != nil {
		return "", fmt.Errorf("недопустимые символы в коде")
	}
	return s, nil
}
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}
func VerifyPassword(password, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[1] != "argon2id" || p[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallel uint8
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallel); err != nil {
		return false
	}
	// Bound stored parameters so a corrupted hash cannot exhaust server memory.
	if memory < 8192 || memory > 65536 || iterations < 1 || iterations > 4 || parallel < 1 || parallel > 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(p[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(p[5])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallel, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func ValidRequestKey(s string) bool {
	if len(s) < 16 || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
func LimitInt(s string, fallback, max int) int {
	n, e := strconv.Atoi(s)
	if e != nil || n < 1 {
		return fallback
	}
	if n > max {
		return max
	}
	return n
}
