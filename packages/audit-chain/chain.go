// Package auditchain implements an ed25519-signed, hash-chained, append-only
// JSONL audit log.
//
// Each Event references the prior event's hash so any tampering breaks the
// chain. Each Event is also signed individually with an ed25519 key so a
// reader can verify origin without trusting the chain file itself.
//
// File format: newline-delimited JSON, one Event per line.
//
// This package is intentionally dependency-free (stdlib only) so the Y2 Rust
// rewrite can match the binary format byte-for-byte.
package auditchain

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// GenesisHash is the prev_hash value of the first event in a chain.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Event is a single audit record. Signed bytes = JSON of Event with Sig
// blanked (Hash + PrevHash are included in the signed payload).
type Event struct {
	Seq      uint64          `json:"seq"`
	TS       string          `json:"ts"`
	Kind     string          `json:"kind"`
	Payload  json.RawMessage `json:"payload"`
	PrevHash string          `json:"prev_hash"`
	Hash     string          `json:"hash"`
	Sig      string          `json:"sig"`
}

// Chain is an append-only, single-writer audit log.
type Chain struct {
	mu    sync.Mutex
	path  string
	priv  ed25519.PrivateKey
	seq   uint64
	prev  string
	clock func() time.Time
}

// New opens (or creates) an audit chain at path. If the file exists, the
// chain is rewound to its last event so subsequent Append calls extend it.
func New(path string, priv ed25519.PrivateKey) (*Chain, error) {
	c := &Chain{
		path:  path,
		priv:  priv,
		seq:   0,
		prev:  GenesisHash,
		clock: time.Now,
	}
	if err := c.recover(); err != nil {
		return nil, err
	}
	return c, nil
}

// GenerateKey returns a fresh ed25519 keypair.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// SetClock overrides the time source (used by tests).
func (c *Chain) SetClock(f func() time.Time) { c.clock = f }

// Append signs and writes a new event with the given kind + payload.
func (c *Chain) Append(kind string, payload any) (Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pb, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal payload: %w", err)
	}
	ev := Event{
		Seq:      c.seq + 1,
		TS:       c.clock().UTC().Format(time.RFC3339Nano),
		Kind:     kind,
		Payload:  pb,
		PrevHash: c.prev,
	}
	ev.Hash = hashEvent(ev)
	sig := ed25519.Sign(c.priv, []byte(ev.Hash))
	ev.Sig = base64.StdEncoding.EncodeToString(sig)

	line, err := json.Marshal(ev)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event: %w", err)
	}

	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Event{}, fmt.Errorf("open chain: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return Event{}, fmt.Errorf("write event: %w", err)
	}

	c.seq = ev.Seq
	c.prev = ev.Hash
	return ev, nil
}

func hashEvent(ev Event) string {
	signing := Event{
		Seq:      ev.Seq,
		TS:       ev.TS,
		Kind:     ev.Kind,
		Payload:  ev.Payload,
		PrevHash: ev.PrevHash,
	}
	b, _ := json.Marshal(signing)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (c *Chain) recover() error {
	f, err := os.Open(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open chain: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			return fmt.Errorf("parse line: %w", err)
		}
		c.seq = ev.Seq
		c.prev = ev.Hash
	}
	return scanner.Err()
}

// Verify walks path and confirms hash links + signatures. Returns count of
// events verified.
func Verify(path string, pub ed25519.PublicKey) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	var (
		count   int
		prev    = GenesisHash
		wantSeq uint64 = 1
	)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			return count, fmt.Errorf("parse line %d: %w", wantSeq, err)
		}
		if ev.Seq != wantSeq {
			return count, fmt.Errorf("seq mismatch at line %d: got %d, want %d", wantSeq, ev.Seq, wantSeq)
		}
		if ev.PrevHash != prev {
			return count, fmt.Errorf("prev_hash mismatch at seq %d", ev.Seq)
		}
		if got := hashEvent(ev); got != ev.Hash {
			return count, fmt.Errorf("hash mismatch at seq %d", ev.Seq)
		}
		sig, err := base64.StdEncoding.DecodeString(ev.Sig)
		if err != nil {
			return count, fmt.Errorf("decode sig at seq %d: %w", ev.Seq, err)
		}
		if !ed25519.Verify(pub, []byte(ev.Hash), sig) {
			return count, fmt.Errorf("signature invalid at seq %d", ev.Seq)
		}
		prev = ev.Hash
		count++
		wantSeq++
	}
	return count, scanner.Err()
}

// SaveKey writes an ed25519 private key to path (base64-std, 0600).
func SaveKey(path string, priv ed25519.PrivateKey) error {
	return os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(priv)), 0o600)
}

// LoadKey loads an ed25519 private key written by SaveKey.
func LoadKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if l := len(raw); l != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("bad key size: %d", l)
	}
	return ed25519.PrivateKey(raw), nil
}

// SavePublicKey writes an ed25519 public key (base64-std, 0644).
func SavePublicKey(path string, pub ed25519.PublicKey) error {
	return os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(pub)), 0o644)
}

// LoadPublicKey loads a public key written by SavePublicKey.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return nil, fmt.Errorf("decode pub: %w", err)
	}
	if l := len(raw); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bad pub size: %d", l)
	}
	return ed25519.PublicKey(raw), nil
}
