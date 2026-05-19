package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setIssuerPublicKey(t *testing.T, pub []byte) {
	t.Helper()
	overridePubKey = pub
	t.Cleanup(func() { overridePubKey = nil })
}

func mintJWT(t *testing.T, priv ed25519.PrivateKey, c Claim) string {
	t.Helper()
	headerJSON := []byte(`{"alg":"EdDSA","typ":"JWT"}`)
	payloadJSON, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal claim: %v", err)
	}
	hB := base64.RawURLEncoding.EncodeToString(headerJSON)
	pB := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signed := []byte(hB + "." + pB)
	sig := ed25519.Sign(priv, signed)
	sB := base64.RawURLEncoding.EncodeToString(sig)
	return hB + "." + pB + "." + sB
}

func validClaim() Claim {
	now := time.Now().Unix()
	return Claim{
		Iss:      "sapctl.dev",
		Sub:      "cus_TEST",
		Aud:      "sapctl-cli",
		Iat:      now,
		Nbf:      now,
		Exp:      now + 30*24*60*60,
		Tier:     "team",
		Seats:    5,
		Features: []string{"audit-export-retain-365d", "multi-cred"},
		RevURL:   "https://license.sapctl.dev/revoked.json",
	}
}

func TestVerify_HappyPath(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, pub)
	tok := mintJWT(t, priv, validClaim())

	lic, err := Verify(tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !lic.Present {
		t.Fatal("license not marked present")
	}
	if !lic.Has("audit-export-retain-365d") {
		t.Fatal("expected feature flag missing")
	}
	if lic.Has("nope") {
		t.Fatal("unexpected feature flag present")
	}
	if lic.Claim.Seats != 5 {
		t.Fatalf("seats: want 5, got %d", lic.Claim.Seats)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, otherPub)
	tok := mintJWT(t, priv, validClaim())

	_, err := Verify(tok)
	if !errors.Is(err, ErrSignature) {
		t.Fatalf("want ErrSignature, got %v", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, pub)
	c := validClaim()
	c.Exp = time.Now().Unix() - 3600
	tok := mintJWT(t, priv, c)

	_, err := Verify(tok)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

func TestVerify_AudienceMismatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, pub)
	c := validClaim()
	c.Aud = "some-other-tool"
	tok := mintJWT(t, priv, c)

	_, err := Verify(tok)
	if !errors.Is(err, ErrAudience) {
		t.Fatalf("want ErrAudience, got %v", err)
	}
}

func TestVerify_IssuerMismatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, pub)
	c := validClaim()
	c.Iss = "evil.example.com"
	tok := mintJWT(t, priv, c)

	_, err := Verify(tok)
	if !errors.Is(err, ErrIssuer) {
		t.Fatalf("want ErrIssuer, got %v", err)
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	cases := []string{"", "a", "a.b", "not.a.jwt.too.many.parts", "..."}
	for _, tok := range cases {
		t.Run(tok, func(t *testing.T) {
			_, err := Verify(tok)
			if err == nil {
				t.Fatal("expected error for malformed token")
			}
		})
	}
}

func TestLoad_Absent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "license.jwt")
	lic, err := Load(p)
	if err != nil {
		t.Fatalf("absent file should be nil err, got %v", err)
	}
	if lic.Present {
		t.Fatal("absent file should not be Present")
	}
}

func TestInstall_RoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, pub)
	tok := mintJWT(t, priv, validClaim())

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home) // macOS UserConfigDir uses HOME, not XDG_CONFIG_HOME

	lic, err := Install(tok)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !lic.Present {
		t.Fatal("install did not return present license")
	}
	p, _ := DefaultPath()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat license file: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("license mode = %o, want 600", st.Mode().Perm())
	}

	lic2, err := LoadCurrent()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !lic2.Has("multi-cred") {
		t.Fatal("reload lost features")
	}
}

func TestInstall_RejectsInvalidBeforeWriting(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	setIssuerPublicKey(t, other)
	tok := mintJWT(t, priv, validClaim())

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home) // macOS UserConfigDir uses HOME, not XDG_CONFIG_HOME

	if _, err := Install(tok); !errors.Is(err, ErrSignature) {
		t.Fatalf("want ErrSignature, got %v", err)
	}
	p, _ := DefaultPath()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("Install wrote file despite signature failure")
	}
}
