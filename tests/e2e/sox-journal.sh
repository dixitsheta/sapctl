#!/usr/bin/env bash
# E2E test: sox-journal recipe end-to-end against a fake SAP server.
#
# Flow:
#   1. Start python http.server fake S/4HANA on :17891
#   2. Configure sapctl with a basic-auth cred pointing at it
#   3. Run `sapctl s4 audit-export --use-case sox-journal --from --to --out`
#   4. Verify the bundle: extract -> sapctl audit verify
#   5. Assert manifest.json + chain.jsonl + ed25519.pub + rows.jsonl exist
#
# Exit 0 on success, non-zero on failure. Designed for CI + local.
#
# Usage:  bash tests/e2e/sox-journal.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SAPCTL_BIN="${SAPCTL_BIN:-$REPO_ROOT/apps/cli/bin/sapctl}"
PORT="${PORT:-17891}"
WORKDIR="$(mktemp -d -t sapctl-e2e-XXXXXX)"
FAKE_PID=""

cleanup() {
  local rc=$?
  [[ -n "$FAKE_PID" ]] && kill "$FAKE_PID" 2>/dev/null || true
  rm -rf "$WORKDIR"
  exit "$rc"
}
trap cleanup EXIT INT TERM

step() { printf '\033[1;34m==> %s\033[0m\n' "$*"; }
fail() { printf '\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; exit 1; }
pass() { printf '\033[1;32mOK:   %s\033[0m\n' "$*"; }

# -------- 1. build binary if missing --------
step "Resolve sapctl binary"
if [[ ! -x "$SAPCTL_BIN" ]]; then
  step "Building $SAPCTL_BIN"
  (cd "$REPO_ROOT/apps/cli" && go build -o bin/sapctl .)
fi
[[ -x "$SAPCTL_BIN" ]] || fail "sapctl binary not found at $SAPCTL_BIN"
pass "sapctl found"

# -------- 2. start fake S/4 OData server --------
step "Start fake S/4HANA OData server on :$PORT"
FAKE_ROOT="$WORKDIR/fake-s4"
mkdir -p "$FAKE_ROOT/sap/opu/odata/sap/API_JOURNALENTRYITEMBASIC_SRV"

cat > "$FAKE_ROOT/sap/opu/odata/sap/API_JOURNALENTRYITEMBASIC_SRV/A_JournalEntryItemBasic" <<'JSON'
{
  "d": {
    "results": [
      {
        "CompanyCode": "1010",
        "AccountingDocument": "100000001",
        "AccountingDocumentItem": "001",
        "PostingDate": "/Date(1735689600000)/",
        "GLAccount": "0000400000",
        "AmountInCompanyCodeCurrency": "1234.56",
        "DebitCreditCode": "S"
      },
      {
        "CompanyCode": "1010",
        "AccountingDocument": "100000002",
        "AccountingDocumentItem": "001",
        "PostingDate": "/Date(1735776000000)/",
        "GLAccount": "0000200000",
        "AmountInCompanyCodeCurrency": "-987.65",
        "DebitCreditCode": "H"
      }
    ]
  }
}
JSON

if lsof -i ":$PORT" -t >/dev/null 2>&1; then
  fail "port $PORT already in use (stale process from prior run?)"
fi
(cd "$FAKE_ROOT" && python3 -m http.server "$PORT" >/dev/null 2>&1) &
FAKE_PID=$!
for i in 1 2 3 4 5; do
  curl -sf "http://127.0.0.1:$PORT/sap/opu/odata/sap/API_JOURNALENTRYITEMBASIC_SRV/A_JournalEntryItemBasic" >/dev/null && break
  sleep 1
done
curl -sf "http://127.0.0.1:$PORT/sap/opu/odata/sap/API_JOURNALENTRYITEMBASIC_SRV/A_JournalEntryItemBasic" >/dev/null \
  || fail "fake server not responding"
pass "fake server up (pid $FAKE_PID)"

# -------- 3. sapctl auth login (basic) --------
step "Configure sapctl basic-auth credential 'e2e'"
export XDG_CONFIG_HOME="$WORKDIR/xdg"
mkdir -p "$XDG_CONFIG_HOME/sapctl"

"$SAPCTL_BIN" auth login \
  --flow basic \
  --label e2e \
  --username "dummy" \
  --password "dummy" >/dev/null 2>"$WORKDIR/auth.err" \
  || { cat "$WORKDIR/auth.err" >&2; fail "auth login failed"; }
pass "credential saved"

# -------- 4. audit init --------
step "Initialize audit chain"
export SAPCTL_AUDIT_DIR="$WORKDIR/audit"
"$SAPCTL_BIN" audit init >/dev/null 2>"$WORKDIR/audit.err" \
  || { cat "$WORKDIR/audit.err" >&2; fail "audit init failed"; }
pass "audit chain initialized"

# -------- 5. audit-export sox-journal --------
step "Run sox-journal audit-export"
OUT="$WORKDIR/out"
mkdir -p "$OUT"

if ! SAPCTL_AUDIT_DIR="$WORKDIR/audit" \
     "$SAPCTL_BIN" s4 audit-export \
       --cred e2e \
       --base-url "http://127.0.0.1:$PORT" \
       --use-case sox-journal \
       --from 2025-01-01 \
       --to   2025-01-31 \
       --out  "$OUT" 2>"$WORKDIR/export.err"; then
  cat "$WORKDIR/export.err" >&2
  fail "audit-export crashed"
fi
pass "audit-export ran"

# -------- 6. locate + extract bundle --------
step "Locate + extract bundle"
BUNDLE="$(ls -1 "$OUT"/sapctl-evidence-sox-journal-*.tar.gz 2>/dev/null | head -1 || true)"
[[ -n "$BUNDLE" ]] || fail "no bundle .tar.gz produced under $OUT"

EXTRACT="$WORKDIR/extract"
mkdir -p "$EXTRACT"
tar -xzf "$BUNDLE" -C "$EXTRACT"
# bundle tar.gz wraps a top-level dir; descend into it
INNER="$(find "$EXTRACT" -maxdepth 1 -mindepth 1 -type d | head -1)"
[[ -n "$INNER" ]] || fail "extracted bundle missing inner directory"
pass "bundle extracted: $(basename "$BUNDLE")"

# -------- 7. assert artifacts --------
step "Assert bundle contents"
for f in rows.jsonl chain.jsonl ed25519.pub manifest.json; do
  [[ -s "$INNER/$f" ]] || fail "missing or empty: $f"
done
pass "all 4 artifacts present"

# -------- 8. verify chain externally --------
step "Verify chain with sapctl audit verify"
if ! "$SAPCTL_BIN" audit verify \
       --chain "$INNER/chain.jsonl" \
       --pub   "$INNER/ed25519.pub" >"$WORKDIR/verify.out" 2>&1; then
  cat "$WORKDIR/verify.out" >&2
  fail "audit verify rejected the chain"
fi
pass "chain verified ed25519 + hash linkage"

# -------- 9. summary --------
echo
echo "=========================================="
echo "  sox-journal E2E: PASS"
echo "  bundle: $BUNDLE"
echo "  rows:   $(wc -l < "$INNER/rows.jsonl" | tr -d ' ')"
echo "  chain:  $(wc -l < "$INNER/chain.jsonl" | tr -d ' ')"
echo "=========================================="
