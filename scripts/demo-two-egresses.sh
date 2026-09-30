#!/usr/bin/env bash
# Demo of track step N-A, "two manual launches use two egresses".
#
# Loopback only: three proxies on 127.0.0.1 (A, B and a WRONG one that
# stands for hostile ambient proxy variables), a temporary HOME, no
# operator file touched, no DNS name resolved (the client asks the proxy
# for http://egress.invalid/ and the proxy answers it itself).
#
# The confirmation ledger is seeded as a FIXTURE: `curator network
# confirm` refuses to run inside an agent session or without a terminal
# by design, so a script cannot confirm; the operator does that once from
# a terminal. The fixture stands in for that step.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/curator-network-demo.XXXXXX")"
PIDS=()
cleanup() {
  local pid
  for pid in "${PIDS[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

BIN="$TMP/bin"
DEMO_HOME="$TMP/home"
mkdir -p "$BIN" "$DEMO_HOME"

echo "== build (into $BIN)"
(cd "$ROOT" && go build -o "$BIN/curator-network" ./cmd/curator-network \
  && go build -o "$BIN/egress-client" ./internal/testproxy/cmd/egress-client \
  && go build -o "$BIN/loopback-proxy" ./internal/testproxy/cmd/loopback-proxy)

start_proxy() { # <id> -> prints the proxy URL
  local id="$1" i=0
  "$BIN/loopback-proxy" --id "$id" --addr-file "$TMP/proxy-$id.addr" >/dev/null 2>"$TMP/proxy-$id.log" &
  PIDS+=("$!")
  until [ -s "$TMP/proxy-$id.addr" ]; do
    i=$((i + 1))
    if [ "$i" -gt 200 ]; then echo "proxy $id did not start" >&2; exit 1; fi
    sleep 0.05
  done
  tr -d '\n' <"$TMP/proxy-$id.addr"
}

echo "== start three loopback proxies"
A_URL="$(start_proxy A)"
B_URL="$(start_proxy B)"
WRONG_URL="$(start_proxy WRONG)"
echo "proxy A:     $A_URL"
echo "proxy B:     $B_URL"
echo "proxy WRONG: $WRONG_URL (ambient, must never be used)"

# From here on every command runs with the temporary HOME.
export HOME="$DEMO_HOME"
export PATH="$BIN:$PATH"

echo
echo "== curator-network --version"
curator-network --version

echo
echo "== add two profiles (comment-preserving edits of \$HOME/.curator/network.toml)"
curator-network add egress-a --endpoint "$A_URL"
curator-network add egress-b --endpoint "$B_URL"

echo
echo "== the operator file"
cat "$HOME/.curator/network.toml"

echo
echo "== list (unconfirmed: nothing takes effect yet)"
curator-network list

echo
echo "== exec before confirmation is refused"
set +e
curator-network exec egress-a -- egress-client
echo "exit status: $?"
set -e

echo
echo "== seed the confirmation ledger (FIXTURE standing in for 'curator network confirm' at an operator terminal)"
digest_of() { curator-network show "$1" --json | grep -o '"digest":"sha256:[0-9a-f]*"' | head -1 | cut -d'"' -f4; }
DA="$(digest_of egress-a)"
DB="$(digest_of egress-b)"
NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat >"$HOME/.curator/network.confirmations.json" <<LEDGER
{"schema":"relux-network-confirmations-v1","confirmed":{"egress-a":{"digest":"$DA","confirmed_at":"$NOW"},"egress-b":{"digest":"$DB","confirmed_at":"$NOW"}}}
LEDGER
chmod 600 "$HOME/.curator/network.confirmations.json"
curator-network list

echo
echo "== check --all --probe (TCP only: the profiles carry no probe_target)"
curator-network check --all --probe

echo
echo "== hostile ambient environment for the launches"
export HTTPS_PROXY="$WRONG_URL" http_proxy="$WRONG_URL" all_proxy="$WRONG_URL" NO_PROXY='*'
env | grep -i proxy | sort

echo
echo "== two launches at the same time"
curator-network exec egress-a -- egress-client >"$TMP/out-a" 2>"$TMP/err-a" &
PA=$!
curator-network exec egress-b -- egress-client >"$TMP/out-b" 2>"$TMP/err-b" &
PB=$!
wait "$PA"; wait "$PB"
echo "launch A printed: $(cat "$TMP/out-a")   (stderr: $(cat "$TMP/err-a"))"
echo "launch B printed: $(cat "$TMP/out-b")   (stderr: $(cat "$TMP/err-b"))"

echo
echo "== a refusal before exec: unknown profile"
set +e
curator-network exec egress-zzz -- egress-client
echo "exit status: $?"
set -e

echo
echo "== request logs of each proxy"
for id in A B WRONG; do
  echo "-- proxy $id:"
  if [ -s "$TMP/proxy-$id.log" ]; then cat "$TMP/proxy-$id.log"; else echo "(no requests)"; fi
done

echo
echo "== the parent environment is untouched"
env | grep -i proxy | sort
echo "done"
