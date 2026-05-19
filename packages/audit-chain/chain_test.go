package auditchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tmpFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func fakeClock() time.Time { return time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC) }

func TestAppendVerifyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	path := tmpFile(t, "audit.jsonl")

	c, err := New(path, priv)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.SetClock(fakeClock)

	for i := 0; i < 5; i++ {
		ev, err := c.Append("test.kind", map[string]any{"i": i})
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		if ev.Seq != uint64(i+1) {
			t.Fatalf("seq=%d, want %d", ev.Seq, i+1)
		}
		if ev.PrevHash == ev.Hash {
			t.Fatalf("hash should not equal prev")
		}
	}

	n, err := Verify(path, pub)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if n != 5 {
		t.Fatalf("verified %d, want 5", n)
	}
}

func TestRecoverContinues(t *testing.T) {
	_, priv, _ := GenerateKey()
	path := tmpFile(t, "audit.jsonl")

	c, _ := New(path, priv)
	_, _ = c.Append("k1", nil)
	_, _ = c.Append("k2", nil)

	c2, err := New(path, priv)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	ev, err := c2.Append("k3", nil)
	if err != nil {
		t.Fatalf("Append after recover: %v", err)
	}
	if ev.Seq != 3 {
		t.Fatalf("seq=%d, want 3 (recover failed)", ev.Seq)
	}
}

func TestTamperBreaksVerify(t *testing.T) {
	pub, priv, _ := GenerateKey()
	path := tmpFile(t, "audit.jsonl")
	c, _ := New(path, priv)
	for i := 0; i < 3; i++ {
		_, _ = c.Append("k", map[string]int{"i": i})
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte{}, b...)
	idx := len(tampered) / 2
	tampered[idx] ^= 0x01
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Verify(path, pub); err == nil {
		t.Fatalf("Verify should fail on tamper")
	}
}

func TestWrongKeyFailsVerify(t *testing.T) {
	_, priv, _ := GenerateKey()
	otherPub, _, _ := GenerateKey()

	path := tmpFile(t, "audit.jsonl")
	c, _ := New(path, priv)
	_, _ = c.Append("k", nil)

	if _, err := Verify(path, otherPub); err == nil {
		t.Fatalf("Verify should fail with wrong public key")
	}
}

func TestSaveLoadKey(t *testing.T) {
	pub, priv, _ := GenerateKey()
	kp := tmpFile(t, "ed25519.key")
	pp := tmpFile(t, "ed25519.pub")

	if err := SaveKey(kp, priv); err != nil {
		t.Fatalf("SaveKey: %v", err)
	}
	if err := SavePublicKey(pp, pub); err != nil {
		t.Fatalf("SavePublicKey: %v", err)
	}

	priv2, err := LoadKey(kp)
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}
	if !equalBytes(priv, priv2) {
		t.Fatalf("private key round-trip mismatch")
	}
	pub2, err := LoadPublicKey(pp)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if !equalBytes(pub, pub2) {
		t.Fatalf("public key round-trip mismatch")
	}
}

func TestEventJSONStructure(t *testing.T) {
	_, priv, _ := GenerateKey()
	path := tmpFile(t, "audit.jsonl")
	c, _ := New(path, priv)
	c.SetClock(fakeClock)
	if _, err := c.Append("test", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ev Event
	if err := json.Unmarshal(b[:len(b)-1], &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.PrevHash != GenesisHash {
		t.Fatalf("first prev_hash=%q want genesis", ev.PrevHash)
	}
	if ev.TS != "2026-05-19T10:00:00Z" {
		t.Fatalf("ts=%q", ev.TS)
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
