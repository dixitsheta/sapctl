// Package license implements offline ed25519-JWT license verification per
// ADR 0005. The package is intentionally small and dependency-light: any
// `sapctl` command that gates on a paid feature loads the license once via
// LoadCurrent(), checks .Has(feature), and proceeds.
//
// Free-tier semantics: if no license file exists, LoadCurrent returns a
// zero-value License with no features. Gated features check License.Has()
// and surface a structured error if missing; non-gated code paths see no
// difference.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Claim is the parsed JWT payload per ADR 0005.
type Claim struct {
	Iss      string   `json:"iss"`
	Sub      string   `json:"sub"`
	Aud      string   `json:"aud"`
	Iat      int64    `json:"iat"`
	Exp      int64    `json:"exp"`
	Nbf      int64    `json:"nbf"`
	Tier     string   `json:"tier"`
	Seats    int      `json:"seats"`
	Features []string `json:"features"`
	RevURL   string   `json:"rev_url"`
}

// License is the in-process license state after verification.
type License struct {
	Present bool
	Claim   Claim
	feats   map[string]struct{}
}

// Has reports whether the license entitles the named feature.
func (l License) Has(feature string) bool {
	if !l.Present {
		return false
	}
	_, ok := l.feats[feature]
	return ok
}

// ExpiresIn reports time-until-expiry. Zero or negative means expired.
func (l License) ExpiresIn() time.Duration {
	if !l.Present {
		return 0
	}
	return time.Until(time.Unix(l.Claim.Exp, 0))
}

const (
	expectedIss = "sapctl.dev"
	expectedAud = "sapctl-cli"
	skewSec     = 60
)

var (
	ErrAbsent    = errors.New("license file absent")
	ErrSignature = errors.New("license signature invalid")
	ErrExpired   = errors.New("license expired")
	ErrAudience  = errors.New("license audience mismatch")
	ErrIssuer    = errors.New("license issuer mismatch")
	ErrMalformed = errors.New("license malformed")
)

// DefaultPath returns the canonical license-file path.
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sapctl", "license.jwt"), nil
}

// LoadCurrent reads + verifies the on-disk license. Absence returns a
// zero License with Present=false and nil error -- gated features then
// check .Has() and surface their own structured error.
//
// Signature failure, audience/issuer mismatch, expiry: returns the
// matching sentinel error AND a zero License (caller decides how loud
// to be).
func LoadCurrent() (License, error) {
	p, err := DefaultPath()
	if err != nil {
		return License{}, err
	}
	return Load(p)
}

// Load reads + verifies a license JWT from an explicit path.
func Load(path string) (License, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return License{}, nil // absent = free tier, NOT an error
		}
		return License{}, fmt.Errorf("read license: %w", err)
	}
	return Verify(strings.TrimSpace(string(raw)))
}

// Verify parses + validates a license JWT string against the embedded
// public key. Pure function; no filesystem.
func Verify(token string) (License, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return License{}, ErrMalformed
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return License{}, fmt.Errorf("%w: sig base64", ErrMalformed)
	}
	signed := []byte(headerB64 + "." + payloadB64)

	pub := IssuerPublicKey()
	if len(pub) != ed25519.PublicKeySize {
		return License{}, fmt.Errorf("%w: embedded pubkey wrong size %d", ErrSignature, len(pub))
	}
	if !ed25519.Verify(pub, signed, sig) {
		return License{}, ErrSignature
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return License{}, fmt.Errorf("%w: payload base64", ErrMalformed)
	}
	var c Claim
	if err := json.Unmarshal(payloadJSON, &c); err != nil {
		return License{}, fmt.Errorf("%w: payload json: %v", ErrMalformed, err)
	}

	now := time.Now().Unix()
	if c.Iss != expectedIss {
		return License{}, ErrIssuer
	}
	if c.Aud != expectedAud {
		return License{}, ErrAudience
	}
	if c.Exp != 0 && now > c.Exp+skewSec {
		return License{}, ErrExpired
	}

	feats := make(map[string]struct{}, len(c.Features))
	for _, f := range c.Features {
		feats[f] = struct{}{}
	}
	return License{Present: true, Claim: c, feats: feats}, nil
}

// Install writes the JWT to the default path with chmod 0600. Performs
// a verify-roundtrip first; if verify fails, nothing is written.
func Install(token string) (License, error) {
	lic, err := Verify(token)
	if err != nil {
		return License{}, err
	}
	p, err := DefaultPath()
	if err != nil {
		return License{}, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return License{}, fmt.Errorf("mkdir license dir: %w", err)
	}
	if err := os.WriteFile(p, []byte(token), 0o600); err != nil {
		return License{}, fmt.Errorf("write license: %w", err)
	}
	return lic, nil
}
