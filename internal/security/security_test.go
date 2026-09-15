package security

import (
	"strings"
	"testing"
)

func TestCodes(t *testing.T) {
	secret := strings.Repeat("s", 48)
	id := ID()
	code := PassCode(id, secret)
	normalized, err := NormalizeCode("CAMPUS:" + strings.ToLower(code))
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized) != 32 {
		t.Fatalf("length %d", len(normalized))
	}
	if code != PassCode(id, secret) {
		t.Fatal("unstable code")
	}
	if code == PassCode(ID(), secret) || code == PassCode(id, strings.Repeat("t", 48)) {
		t.Fatal("code not bound to id and key")
	}
	for _, bad := range []string{"", strings.Repeat("I", 32), "https://evil.example/a", strings.Repeat("A", 33)} {
		if _, err := NormalizeCode(bad); err == nil {
			t.Fatalf("accepted invalid code %q", bad)
		}
	}
}
func TestPassword(t *testing.T) {
	password := "длинная фраза для входа 2026"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(password, hash) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword("incorrect", hash) {
		t.Fatal("incorrect password accepted")
	}
	if VerifyPassword(password, strings.Replace(hash, "m=65536", "m=999999999", 1)) {
		t.Fatal("unbounded hash parameters accepted")
	}
	other, _ := HashPassword(password)
	if hash == other {
		t.Fatal("password salts are not random")
	}
}
func TestOpaqueTokens(t *testing.T) {
	one, two := Token(), Token()
	if len(one) != 43 || one == two {
		t.Fatal("token entropy/format")
	}
	if Equal(MAC("key", "one"), MAC("key", "two")) {
		t.Fatal("MAC values equal")
	}
	for _, bad := range []string{"", "short", strings.Repeat("x", 81), "request-key-that-has a-space"} {
		if ValidRequestKey(bad) {
			t.Fatal("accepted invalid request key")
		}
	}
}
