# Harness network verification

Verification is specific to **(harness, build, entrypoint)**. A launcher using curl
does not certify the harness's model client, MCP servers, hooks, or tool children.
Record each reachable transport separately. Missing evidence leaves that scope
`network_scope_unsupported`; a blocked row is neither a pass nor a failure.

## Safety and detector

Never change the host network: no networksetup, scutil, route, ifconfig, pfctl,
DNS/hosts edits, sudo, system proxy, TUN, VPN, WireGuard, or sing-box operations.
No external probes, forwarding proxies, real credentials, login, or keychain
access. The **sole deliberate external connect attempt** is the detector
calibration below; the sandbox must kill it before it can leave the host.
Stop an authentication-gated row as **needs login (Ivan's approval)**. Do not
work around authentication. Do not push, post, or spawn agents.

Use a new temporary HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, all XDG directories,
TMPDIR, and working directory for every row. Construct the environment from an
allowlist; do not inherit credential variables, SSH sockets, or runtime settings.
Never read/write the operator's `.claude`, `.codex`, `.curator`, `.config`, or
other configuration. An executable located inside those trees is also refused;
obtain an existing executable outside them. File rules deny access to the
operator HOME except the selected executable and, if needed, Python's code
installation. Keychain files and securityd access are denied too.

The sink is `internal/testproxy/cmd/loopback-proxy`, built offline into the
temporary runtime directory. Start with `--id S --addr-file <temp>/sink.addr`,
**without any `--target`**. It binds `127.0.0.1:0`, records method/target on
stderr, returns 502 for CONNECT and ordinary upstream requests, and answers
absolute-URI `.invalid` GETs locally. With no target map it never forwards,
including to a fake upstream. Run it under the sandbox as an extra safeguard.

The complete profile template used by the helper follows. Substitute the three
file paths; omit the Python allowance when Python is outside the operator HOME.
The helper archives the exact rendered profile for every attempt.

```scheme
(version 1)
(allow default)
(deny network-outbound (with send-signal SIGKILL))
(allow network-outbound (remote ip "localhost:*"))
(allow network-outbound (remote unix-socket))
(deny network-outbound (literal "/private/var/run/mDNSResponder") (with send-signal SIGKILL))
(deny network-outbound (literal "/var/run/mDNSResponder") (with send-signal SIGKILL))
(deny mach-lookup (global-name "com.apple.securityd"))
(deny file-read* file-write* (subpath "<operator HOME>"))
(allow file-read* (literal "<resolved harness executable>"))
(allow file-read* (subpath "<Python code installation>"))
(deny file-read* file-write* (subpath "/Library/Keychains"))
```

First activate the profile with `sandbox-exec -f <profile> /usr/bin/true`.
Then, in the private environment, calibrate:

```sh
sandbox-exec -f <profile> /usr/bin/nc -n -z -w 2 1.1.1.1 443
# Required: SIGKILL, shell exit 137 (Python subprocess returncode -9).
```

Any other result stops verification before a harness is launched. Unified-log
denials were invisible on this host; their absence is not evidence. The new
detector's signal is the killed process, not a denial count. Distinguish sandbox
SIGKILL from a supervisor killing an overlong row. Check child-death messages as
well as the root exit: a parent can survive a killed child. Socket sampling can
miss short-lived children; no observed socket alone never certifies confinement.

## Patch and rows

After constructing the clean environment, seed hostile ambient values, including
`HTTPS_PROXY=http://127.0.0.1:9`, `NO_PROXY=*`, and mixed-case HTTP/FTP/ALL_PROXY
spellings. Apply generic-env-v1 **after** this seed:

1. Remove every name equal, case-insensitively, to HTTP_PROXY, HTTPS_PROXY,
   ALL_PROXY, FTP_PROXY, or NO_PROXY.
2. Set HTTP_PROXY, HTTPS_PROXY, http_proxy, https_proxy to the sink URL.
3. Set NO_PROXY and no_proxy to `127.0.0.1,::1,localhost`.

ALL_PROXY and FTP_PROXY are not set. Compare the six values literally in the
parent and child dumps; reject additional reserved spellings or surviving
hostile values. Keep the proxy pointed at the sink for every harness launch.

| Row | Setup | Pass criteria / interpretation |
|---|---|---|
| R0 | Offline/no-provider control, patched, sandboxed | Control completes with no observed SIGKILL or non-loopback sockets and normally zero sink requests. `--version` checks startup only; it does not replace an offline turn control. |
| R1 | A reachable own-client operation; live sink | Own-client request appears at sink, no observed SIGKILL/non-loopback sockets. Record handling of 502. Auth refusal or no exercised client means unverified. |
| R2 | Same operation as successful R1; stop the sink but retain its URL | Client reports connection refusal, with no observed SIGKILL/non-loopback sockets: no observed direct fallback on this path. If R1 did not exercise the client, R2 cannot certify fallback behavior. |
| R3 | Informative variable coverage: lowercase-only, uppercase-only, ALL_PROXY-only | After scrubbing hostile values, retain only the variant's proxy variables, all pointing at the same sink, plus loopback-only NO_PROXY/no_proxy. Record which variants reach the sink. A client requiring ALL_PROXY needs an adapter beyond generic-env-v1. |
| R4 | MCP stdio env-dumping stub; inherited environment | Stub starts and receives all six exact values, no extra reserved spellings/hostile values. No dump is blocked/unverified, not an inheritance failure. |
| R4b | Same stub with all six values in its server `env` block | Explicit values arrive; report this mitigation separately from R4. Do not treat it as evidence of automatic inheritance. |
| R5 | lsof samples of harness and descendants, about every 0.5 s | No observed non-loopback sockets; record sampling gaps/errors. Combine with calibrated sandbox and signal evidence. |
| R7 | Hooks and tool children where reachable without login | Separate env dump and transport evidence for each child type; MCP results do not cover them. |

## Helper and evidence

`scripts/verify-harness.sh` runs one command per invocation, sequentially. It
calibrates, builds the sink through the supplied shared-lock wrapper with
`go build -p 1` and offline Go settings, applies the patch, creates MCP fixtures,
and launches the command under sandbox-exec. The wrapper uses the operator HOME
solely for the brief-prescribed shared build lock; Go and every harness receive
the private HOME/config environment. Never bypass a busy lock or its host-health
gate. `--build-timeout` bounds the lock wait/build (default 180 s).

Examples (use a **new** evidence directory each time):

```sh
scripts/verify-harness.sh --lock-wrapper "$LOCK_WRAPPER" \
  --output-dir "$EVIDENCE/claude-R4" --mcp-mode inherit -- \
  /safe/path/claude -p --mcp-config '{mcp_json}' --strict-mcp-config \
  'List the available MCP tools.'

scripts/verify-harness.sh --lock-wrapper "$LOCK_WRAPPER" \
  --output-dir "$EVIDENCE/codex-R4b" --mcp-mode explicit -- \
  /safe/path/codex exec --skip-git-repo-check 'List the available MCP tools.'
```

`{mcp_json}` is replaced with the private Claude configuration path. The helper
writes Codex's private `CODEX_HOME/config.toml`. The tiny stdio MCP script writes
its full environment before answering initialize/tools/list; its dump filename
is embedded in the script, so an env allowlist cannot hide the destination.
`--mcp-mode explicit` adds all six patch values to the server entry. R2 uses
`--proxy-state unreachable`, which terminates the sink without changing its URL.
R3 uses `--proxy-vars lowercase`, `uppercase`, or `all`; the default is `generic`.
Coverage variants cannot be combined with MCP inheritance rows.
`--timeout` bounds the row (default 30 s); auth messages stop it immediately.
`VERIFY_PYTHON=/safe/path/python3` selects an existing interpreter if the system
Python launcher is unavailable. The shell does not source operator profiles.

Signal handlers and `finally` cleanup terminate supervised processes and remove
the sink binary, private HOME/configs, and runtime tree. The evidence directory
is retained with mode 0700; all env dumps originate from the clean allowlist.
The helper's nonzero exit may be a **helper/prerequisite error**, not a harness
exit. Check `summary.json` before interpreting it.

Record:

- UTC time, resolved binary/build (`--version` under the same protection), full
  argv/entrypoint, adapter, and exact sandbox/calibration result.
- Seed and applied patch, parent env, MCP configs, full stub env and verdict.
- Harness stdout/stderr and exit; helper errors and auth/timeout stops separately.
- Sink method/target lines and counts, root/child SIGKILL evidence and any
  supervisor-generated SIGKILL separately.
- Sampled descendant PIDs, lsof lines, non-loopback observations, and sampling
  gaps; use **not observed** only after a row actually ran.
- One-line behavior and pass/fail/blocked status for every row, including rows
  that need Ivan's login approval. No row below authorizes a real egress check.

## Results, 2026-10-02

### This run: Claude Code 2.1.287 / `claude -p`

The installation path identifies the requested build; CLI version and client
behavior are **not confirmed**, because the sink build never passed its gate.

| Row | Harness exit (helper exit) | Sink requests | Sandbox SIGKILL | Non-loopback sockets | Env verdict | Behavior |
|---|---|---|---|---|---|---|
| R0 startup `--version` | — (1) | Not started | Not run | Not sampled | N/A | Interrupted a waiting build wrapper; no harness launched. |
| R1 | — | Not started | Not run | Not sampled | N/A | Blocked by sink-build prerequisite; own client not exercised. |
| R2 | — | Not started | Not run | Not sampled | N/A | Blocked; no successful R1 to compare. |
| R4 | — (1) | Not started | Not run | Not sampled | No dump | Calibration 137; shared build gate timed out at 30 s before launch. |
| R4b | — (1) | Not started | Not run | Not sampled | No dump | Calibration 137; shared build gate timed out at 30 s before launch. |
| R3 / R5 / R7 | — | Not started | Not run | Not sampled | Unverified | No harness launched; no coverage claim. |

The wrapper reported `syspolicyd state=spawn; waiting` and an existing shared
lock (`ivan-network-profiles pid=96967`, acquired 05:39:02Z). Neither daemon nor
lock was changed. All three calibration attempts returned 137. This does **not**
certify Claude's proxy handling or child inheritance. Authentication was not
reached, so these rows are not classified as needs-login.

### This run: codex-cli 0.159.0 / `codex exec`

The sole installed launcher resolves into the operator's real `.codex/packages`
tree. It was not executed or copied. No safe alternative path was supplied;
requested build is unconfirmed by a CLI run.

| Row | Harness exit | Sink requests | Sandbox SIGKILL | Non-loopback sockets | Env verdict | Behavior |
|---|---|---|---|---|---|---|
| R0 / R1 / R2 | — | Not started | Not run | Not sampled | N/A | Safety-blocked executable path; client not exercised. |
| R4 / R4b | — | Not started | Not run | Not sampled | No dump | Safety-blocked executable path; inheritance/explicit env unverified. |
| R3 / R5 / R7 | — | Not started | Not run | Not sampled | Unverified | No harness launched. |

### Imported: Muse 1.4.1-R4503.1 / launcher + Echo `muse exec`

Credit: **tb-muse (ivan-tb-muse)**, D8-muse-acceptance.md and D8-muse-results.md,
2026-10-02 05:2x–05:3xZ. These are historical results, not reruns under the new
protocol. Their sandbox denied outbound connects without SIGKILL, unified-log
denials were invisible, HOME was unchanged, and XDG was isolated. Consequently
SIGKILL was **not measured**. Preserve the reported observations, not a claim
that the current safety/detector method was used.

| Row | Exit | Sink requests | SIGKILL | Non-loopback sockets reported | Env verdict | Behavior |
|---|---|---|---|---|---|---|
| R0 Echo | 0 | 0 | Not measured | 0 | N/A | Offline turn completed. |
| R1 launcher update | 1 | 2 × CONNECT api.meta.ai:443 | Not measured | 0 | N/A | Launcher curl used sink; synchronous 502 stopped update. |
| R2 closed loopback proxy | 1 | 0 | Not measured | 0 | N/A | curl connection refusal twice; no fallback observed. |
| R3a / R3b / R3c | 1 each | 2 each | Not measured | 0 | N/A | Lowercase-only, uppercase-only, ALL_PROXY-only reached sink (historical protocol). |
| R4 MCP | 0 | 0 | Not measured | 0 | FAIL | Fixed child allowlist omits all six values; hostile values absent too. |
| R4b explicit MCP env | 0 | 0 | Not measured | 0 | Partial mitigation | Explicit HTTPS_PROXY and NO_PROXY arrived; all-six-value fallback was not tested. |
| R5 across tested rows | — | — | Not measured | 0 | N/A | No non-loopback sockets reported. |
| R7 tools/hooks | — | — | Not measured | Unverified | Unverified | Echo provider called no tools; hooks not reported. |

### Imported: Muse 1.4.2-R4684.1 / launcher + Echo `muse exec`

Same source and detector/isolation limitations; do not infer unreported rows.

| Row | Exit | Sink requests | SIGKILL | Non-loopback sockets reported | Env verdict | Behavior |
|---|---|---|---|---|---|---|
| R0 / R1 / R2 | 0 / 1 / 1 | 0 / 2 / 0 | Not measured | 0 / 0 / 0 | N/A | D8 reports behavior identical to 1.4.1. |
| R4 MCP | 0 | 0 | Not measured | 0 | FAIL | Same fixed allowlist; none of six proxy values inherited. |
| R4b / R3 | — | — | Not measured | Unverified | Unverified | Not separately reported for this build. |
| R5 across tested rows | — | — | Not measured | 0 | N/A | No non-loopback sockets reported. |
| R7 tools/hooks | — | — | Not measured | Unverified | Unverified | Not exercised/reported. |

D8's R6 model-provider row was not run for either Muse build; it awaits Ivan's
approval. Launcher results do not verify the Muse binary's own HTTPS client.
All scopes lacking current evidence remain `network_scope_unsupported`.
