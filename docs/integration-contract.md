# Integration contract (proposal): how the process owners apply a network profile

Status: **DRAFT rev 2, 2026-10-02: advice closed 2026-10-01T16:00Z (one owner answered: ivan-tb-critical-fixes); Ivan decides (card C).** Rule-27 proposal; recommendations do not bind consumers before the decision. Repository owners retain approval of implementation in their repositories.

Normative inputs: `spec/network-profiles.md` (N1–N14) and the implemented `spec/contract-appendix.md` (digest, EnvPatch, equality, Record and refusals). **The appendix wins on bytes.** Proposed API/origin changes below do not amend v1.

Source pins checked locally on 2026-10-02; citations use these aliases (short filenames retain the stated directory):

| Alias | Repository / path | Commit |
| --- | --- | --- |
| N | this repository, N-A implementation/spec baseline | `bf6d514` |
| A | skill-agents-management | `400f5377` |
| B | skill-project-management / `tools/board-cli` | `110270d9` |
| R / H | B / `internal/spawnruntime` / `internal/sessionmanager` | same as B |
| L | curator-agent-launcher | `1ac7eafb` |
| S | curator-spec | `e41c561b` |
| C | curator | `2cb29dac` |
| W | wiki | `21d77b7f` |
| X | agent-session-host | `2c6cd39d` |

Advice item numbers below retain `critical-fixes`' numbering within §2.2 and §2.3; card C assessed all nine objections. Only that owner replied by the deadline.

## 1. What the library gives every consumer

The Go module `github.com/relux-works/curator-network-profiles` exposes `pkg/`, creates no processes, runs no daemon and imports no callers (N2). The process owner prepares a binding **on the destination host immediately before spawn or resume**:

```text
Selection → resolve/validate → bounded preflight → bind → patch.Apply(final childEnv) → spawn → record
ref+origin      N4/N3/N12           N10            N3             N5                         N8
```

| Operation | Input / result | Refusals |
| --- | --- | --- |
| resolve/validate | caller's explicit/runtime/project refs; host catalog, confirmations, allowed set, assurance and engine hosts → normalized profile and origin, or unmanaged | `network_profile_unknown`, `network_profile_denied`, `network_file_unreadable`, `network_profile_invalid`, `network_configuration_conflict`, `network_scope_unsupported` |
| preflight | endpoint, optional target, deadline → TCP/CONNECT/TLS facts | `network_proxy_unreachable`, `network_proxy_auth_failed` |
| bind / patch / record | verified adapter tuple → Binding; unset then set final env; sanitized Record | no new refusal codes |
| reattach / resume | recorded Record, freshly resolved binding, event → full equality check | `network_profile_drift`, `network_scope_unsupported`; resolve's unknown/denied comes first |

These are conceptual stages, not separate public methods: `catalog.Load` and `Catalog.Resolve` perform catalog/ledger loading and validation (N/pkg/catalog/catalog.go:28–70; pkg/resolve/resolve.go:116–157). The appendix already implements the added file/profile codes, retaining their TODO(decision) labels. **Unmanaged means no selection, an empty patch and unchanged ambient routing; it does not mean direct.**

Apply the patch **after filtering and every overlay**, updating full Env and owned literals. Its `set` half travels as `env_literals`, never `env_names` (N5; S/decisions/0013-execution-ownership-and-launch-plans.md:193–206). Later overlays must refuse proxy-family names case-insensitively (`network_configuration_conflict`) when managed. Reapply the same patch after the final overlay if cleanliness cannot be proved; idempotence does not protect against later writes or different bindings. Relevant late overlays are B/internal/spawn/launch_plan.go:479 and L/internal/composition/composition.go:91–110. A/pkg/vendorplugin/vendors/local-models/spawn.go:54 merges **before** BuildPlan (A/pkg/vendorplugin/spawn.go:249–315).

Advice §2.2(5): “The `set` half can carry proxy userinfo.” Raw structs can, but normalized profiles refuse it (N/pkg/netprofile/profile.go:202–203). Only normalized, confirmed resolution may produce a patch; serialize **Record only**, never Binding/Patch, endpoint or env. Owned literals are recorded/replayed and must be non-secret (0013:232–235,304–313). V1 permits credential mode `none`; future secrets stay in a gateway, behind a loopback endpoint (N6/N11).

Plans/dry runs resolve, validate, bind and patch, skip network preflight, and record `skipped` (N10). Real launches probe the destination; a targetless probe proves TCP only.

### Record and equality (N3, N7, N8)

```json
{"network":{"schema":"relux-network-binding-record-v1","profile_ref":"egress-a","origin":"explicit","profile_digest":"sha256:…","adapter_identity":{"adapter":"generic-env-v1","harness":"codex","build":"0.x.y","entrypoint":"exec"},"assurance":"cooperative","probe":{"tcp":"ok","connect":"skipped","tls":"skipped","checked_at":"RFC3339"}}}
```

Absent key means unmanaged. The illustrative Record omits endpoint, env and credentials. Equality compares **digest + adapter identity + assurance**, not name. Resume re-resolves the recorded ref on that host; unknown/denied precedes comparison (N/spec/contract-appendix.md:757).

| Event | Outcome |
| --- | --- |
| fresh process | new launch with its own binding |
| existing-run resume/restore, full equality | reattach |
| existing-run resume/restore, same ref but changed digest | `network_profile_drift` |
| existing-run resume/restore, other inequality | `network_scope_unsupported` |
| new assignment on a live host, equal binding | attach |
| new assignment, unequal binding, no per-session route | `network_scope_unsupported` |
| new assignment, unequal binding, verified per-session route | N7 permits set-per-session; deferred for first-release live hosts (§2.3) |

A changed profile requires restoration or a genuinely new launch, never silent rerouting of the existing run. Retry has the separate proposed policy in §2.2.

## 2. Per consumer: where, who, what

### 2.1 Launch plane: `skill-agents-management`

**Lands in:** optional N-B hook migration / N-C1. **Q1 — recommended; Ivan decides:** typed `Network{Patch,Record}` request data on `agentic.LaunchRequest` and `vendorplugin.SpawnRequest`, rather than arbitrary callbacks. Apply purely after `ChildEnv` (A/pkg/agentic/plan.go:459–463), including the separately filtered owned snapshot (:482–489; type :267–273). `LaunchRequest.Env` is the parent env (A/pkg/agentic/system.go:560); preserve existing engine provenance when adding network provenance. The module still owns no exec.

Decision 0019 is **adopted** (S/decisions/0019-fragment-consumers-and-one-construction-site.md:5), but :117–120,172–175 leave environment composition with consumers pending a separate decision. Compose-first N-B remains viable. Keep owner-side final application until every later overlay rejects proxy names and final Env/OwnedEnv parity is proved.

**Q7 — recommended; Ivan decides:** support only verified exact harness/build/entrypoint/adapter tuples. Generic env support does not certify every claude/codex/qwen/pi/pinative/gemini/agy release. Muse returns `network_scope_unsupported` until its tuple is verified; its interactive/permission support (L/README.md:12–20) is not proxy evidence. Leave its closed allowlist unchanged (A/pkg/agentic/systems/muse/env.go:16–42; env_test.go:283–290,483): a post-filter patch bypasses it. Verified harness-specific websocket/native overrides belong to that adapter, not the generic one (N5).

### 2.2 task-board spawn and runner: `skill-project-management`

**Lands in:** N-C1 explicit selection / N-C2 defaults. `--network <name>` selects an id with explicit origin; later `runtimes.toml network` and `spawn.network.default` supply defaults. The detached runner inherits the orchestrator env (R/runtime.go:4764–4766); it is the destination host responsible for child unsets.

- **Preparation/application — advice §2.2(1), partly accepted:** “also serves the dry run” is correct; “does no I/O” is not (B/internal/spawn/launch_plan.go:202 calls catalog-reading `localModelsPeek`). Resolve/validate after slot claim (R/runtime.go:1905), then prepare, persist Record and refresh patch before every attempt at :2024. Carry only an ephemeral Config `EnvPatch` (`json:"-"`) to the composer. Apply last after `RuntimeControlEnvironment`, before B/internal/spawn/launch_plan.go:488, updating owned literals too. Real `planCommand` (spawn.go:1063–1071) and dry-run `BuildArgs` (:1414, call :1418) share it; plans skip probes. The launch-plane owner lands this final application with M2; other runner/cmd changes stay outside that zone. Earlier `/goal` probes (B/cmd/spawn.go:2343 → cmd/spawn_runtime.go:850–870) need resolution/patching **before their own spawn**, then freshness checks after the slot. A future BuildPlan hook replaces the final call only under §1's overlay proof.
- **Manifest/rollback — advice §2.2(4):** “old binaries drop unknown keys silently” (R/runtime.go:4870–4875). Add Record beside `runtime_provenance` (:337; manifest :325–436), copy through `newSpawnRunManifest` (:803,840), `spawnConfigFromManifest` (:3205–3259), and `cloneSpawnRunSuccessor` (R/directives.go:743,892–979). `NetworkAccess` (:403) remains the codex sandbox flag. Require implementation round-trip coverage across all three paths and prohibit rollback of profiled runs to unaware binaries: they lose managed provenance and inherit ambient routing; a schema field cannot protect an old decoder.
- **Suites — advice §2.2(3):** “Make both paths build the env the same way.” Do not apply harness patches to validation suites; share suite-environment construction. R/changerequest.go:271 → B/internal/changerequest/validation.go:189 strips selectors; B/internal/integration/validation.go:423 does not. Neither strips proxies, so different egress is possible, not inevitable. Inherited-route evidence policy is TODO(decision), recommended; Ivan decides. Keep raw-env integrate issue #419 within its existing scope.
- **Control plane — advice §2.2(2), subsidiary claim REJECTED:** “The runner's own HTTP to board-server also picks up that proxy.” B/internal/remote/client.go:87–95 installs an explicit `http.Transport` with nil `Proxy`; that client does not consult inherited proxy variables. Preserve this separation and explicitly configured helper transports (N11).
- **Managed goal launches** (B/cmd/codex_goal_spawn.go:152) carry a selector envelope to tb-sessiond, which resolves and applies locally (§2.3); never send an endpoint or source-host patch.

#### Inherited selection: Q2

**Recommended; Ivan decides; TODO(decision), N3/N4 amendment before N-C1.** Advice §2.2(2), “an inherited binding is a mandatory default,” identifies real leakage: an empty patch under a profiled parent retains ambient proxies without provenance. Use durable reference inheritance, not a new ambient `TASK_BOARD_NETWORK` contract or inference from proxy values. Advice Q2, “Use the existing plumbing,” needs qualification: existing goal lookup (B/cmd/spawn_goal_launch.go:70–90) loads state/goal, is queued-only (cmd/spawn.go:1679–1685), and returns nil for non-goal parents (:129–131).

Persist **parent identity plus resolved reference** during reservation (B/cmd/spawn.go:2804); restore through `QueuedPreparation` (:3089–3127), and copy through R/runtime.go:3205, R/limit_retry.go:413 and R/directives.go:870–979. Review fan-out re-enters the pipeline (B/cmd/spawn_review_fanout.go:180). Retries bypass parent lookup (R/runtime.go:2021–2063): rebuild managed retry envelopes (R/limit_retry.go:447,524,575) and rebind successor envelopes (R/directives.go:875). Validate manifest/control identity and allowed sets; mutable `TASK_BOARD_RUN_ID` alone is not authorization (A/pkg/agentic/runcontext.go:31–44 is vendored at v0.5.30, B/go.mod:10).

Primaries use session markers, not RUN_ID (H/codex_host.go:293–311; claude_host.go:2461–2483): require authenticated session-record lookup or explicit child selection. Non-board profiled parents need an explicit carrier contract too. Propose `Request.InheritedRef` and `OriginInherited`, with precedence **explicit → inherited → runtime → project → operator**; update N3/N4 and Record's closed origin vocabulary before use. Origin and transport are independent. A parent on A can explicitly select B for a child, subject to host authorization.

#### Retry and recovery: Q3

**Recommended; Ivan decides:** advice Q3, “Do not return it as a launch error.” Resolve unknown/denied first; changed digest is terminal `network_profile_drift` before each attempt, queue release, successor launch and managed retry preflight. Ordinary launch errors enter recovery (R/runtime.go:2086–2099), so drift must bypass it. Preserve reference/digest/assurance across provider reselection; deliberately changed adapters get new attempt Records only with unchanged digest. Add a retry-specific check: ordinary `CheckReattach` rejects adapter changes (N/pkg/binding/binding.go:165–174). Recovery/resume still uses full event-appropriate equality.

#### Explicit direct

**TODO(decision); recommended; Ivan decides:** defer opt-out for N-B; prefer a named `kind=direct` profile later, through a versioned contract change. V1 permits proxy profiles only (N/spec/contract-appendix.md:27,418–420,674–687); absence cannot be called direct.

| Option | Required change |
| --- | --- |
| Defer now | Keep N-B Selection/Patch/Record unchanged; no empty-patch opt-out. Inherited origin needs its separate amendment. |
| Named direct profile later | Version `kind=direct`, canonical digest/confirmation rules; retain actual named ref/origin. Unset-only patch; Record keeps ref, digest, cooperative assurance, skipped probe. |
| Built-in selector | Add Selection mode, reserve syntax against names, version Record for absent/sentinel ref and direct-policy digest; explicit origin, unset-only patch, never omitted unmanaged Record. |

Either future opt-out requires host authorization and does not guarantee OS-level directness. Do not insert new kinds/origins into normative v1.

### 2.3 tb-sessiond: `skill-project-management`

**Lands in:** N-C1. Hosts build from daemon `os.Environ()` (H/codex_host.go:161–187,261–268; claude_host.go:700–727,2427–2450). Apply one shared helper **last** to `command.Env`, after TMPDIR at codex :178–179 and claude :710–711.

- **Carrier — advice §2.3(2):** “Send the ref, the origin and the expected digest.” Use that credential-free selector envelope in `OwnerRunContext` (H/launch_plan.go:123–136) and `PrimarySessionLaunchPlan` / `CodexSessionLaunch` (:74–86,138). The daemon re-resolves against its own catalog, confirmations, allowed set and verified tuple, checks the expected digest and probes locally. Strict decoders (:172,189,441; claude_host.go:426) fail closed. Bump `controlProtocolVersion` from 8 (H/types.go:31) and use client.go:71–100 restart diagnostics.
- **Record/reuse — advice §2.3(1):** the live-host check was already proposed; add startup reconciliation too. Store Record in `ProviderHostRecord.Opaque` snapshots (H/claude_host.go:840; codex_host.go:336). Check before all `ensureHostLocked` paths (H/manager.go:1584): live fast path :1589–1604, Restore :1625, Launch :1678–1690; also before `restoreAndReconcileLocked` calls Restore at :1981 (:1964–1981). Existing unmanaged hosts refuse profiled assignments. Stop-only Restore paths (:918,2035) remain usable.
- **Q4 — recommended; Ivan decides:** advice Q4, “tb-sessiond starts one app-server per session.” Fresh sessions get independent bindings (H/manager.go:1589,1751; codex_host.go:341); the legacy B/cmd/codex.go:189–252 shares an app-server. First-release live-host attach/restore requires equal binding or refusal; existing-run Restore is `EventReattach`, not automatically `EventNewAssignment`, so drift and full equality apply.
- **Daemon startup — advice §2.3(3):** refuse startup from an identified profiled run **before stripping its identity**. B/cmd/session.go:591–592,653–673 strips run ids, not proxies; H/detached_launch.go:72–76 retains the starter env. Blanket stripping ordinary operator proxies would break unmanaged compatibility. Each managed session still patches last.
- **Auxiliary harnesses — advice §2.3(4):** “must use the session's patch.” Pass the same resolved session context to Claude preflight (H/claude_host.go:2416–2420) and assay (claude_context_assay.go:580–582); refusals stop those subprocesses too. Audit codex context (codex_context.go:1376–1382), legacy codex (B/cmd/codex.go:189,238–252), visible clients (:431–436; cmd/codex_manager.go:270–272), Claude goal probe (B/internal/spawn/claude_goal.go:94–123) and Antigravity preflight (agy_preflight.go:90). Every exec must apply the session context or have evidence it is outside harness routing.

Also audit launcher release probes (L/cmd/curator-run/main.go:334,353 → A/internal/toolprobe), engine-status subprocess (A/pkg/localruntime/client.go:54) and the daemon intermediate above. Version probes need evidence of no network work or their resolved patch. Preparation/composition subprocesses (B/cmd/claude_manager.go:230, codex_manager.go:215; L/internal/fragment/resolve.go:98) remain outside harness routing (N5); `curator network exec` follows the same final application rule.

### 2.4 curator-run: `curator-agent-launcher`

**Lands in:** N-B. The Curator and launcher owners coordinate N-B; the launcher owner leads its implementation; consumer representatives advise. Ownership is unchanged (W/decisions.md:458–460).

- Add `--network <name>` to L/internal/cli/cli.go `Invocation` (:162–195), `valueFlags` (:223–231), and SPEC §3; accept identifiers, not URLs.
- Resolve/validate after admitted `plan.Build` (L/cmd/curator-run/main.go:255), before `ComposeAdmittedPlan` (:288), using the original operator env. Probe only a real supported direct launch. Refusals stop launch without fallback; tracked managed selection refuses before a source-host probe.
- In `Compose`, after all overlays (:91–110), before materializing Env (L/internal/composition/composition.go:113), delete unset names case-insensitively from both env and literals, add set to both, reject managed proxy EnvNames/overlays, and recheck disjointness. Direct exec uses the patched env (L/internal/execution/execution.go:154); emit sanitized Record provenance.
- Tracked mode carries only set literals and a proposed `works.relux.curator.network` Record extension (execution.go:52–58). **Refuse every managed tracked selection, including defaults, with `network_scope_unsupported` until §2.5's destination capability exists** (N9). No fallback or silent loss of tracking. Curator preparation (env resolve/install/repair) remains outside this patch (N5).

### 2.5 ax launch plan and session host

**Q5 — recommended; Ivan decides:** keep the Record extension (S/decisions/0013-execution-ownership-and-launch-plans.md:208–213), but require a negotiated, versioned network capability at the destination. Ax implementation owners enforce it; session-host owners own any bridge. X/README.md:5–7 is specification-only; ax proper (§5.1/§14.1) is absent from the supplied clones. Extension preservation alone proves no enforcement.

The destination resolves/authorizes locally, compares expected digest, verifies its adapter tuple, preflights, applies unset then fresh set to its final env, and persists Record. Audit start/resume/fork/re-adoption; reject unsupported schemas or unaware hosts. Suppress conflicting old proxy literals before fresh application: 0013:304–313 otherwise replays them verbatim. Resume drift refuses `network_profile_drift` (environment-drift precedent :635–645); attach/resume uses full equality.

Set literals are non-secret and disjoint from `env_names` (0013:193–206). Canonical proxy spellings are reserved (C/internal/contextpkg/contextpkg.go:427; S/schemas/v1/agent-mcp-v1.schema.json:59); consumers must reject **mixed-case** proxy names too. N9 stays closed until destination conformance proves these rules.

### 2.6 Curator umbrella and trust

`curator network …` dispatches to `curator-network` (C/cmd/curator/umbrella.go:120; S/protocol/environments.md §11, :3377). Distinguish revision A, PATH selection with an outside-roots warning, from revision B, roots-only selection (:3414–3420). No provider-selection flags are required; N-A implements `--json` and a real `--version`. Provider installs follow curator-spec#100 / curator#96.

Presence-key signing for `confirm` (curator-trust §10) is not available. N-A already implements a local digest-bound confirmations ledger and agent-session refusal; its interim policy remains TODO(decision) in the implemented appendix/API. When curator-trust lands, replace the ledger with the trust store.

## 3. Ownership and order

| Piece | Owner role / repository | Milestone | Depends on |
| --- | --- | --- | --- |
| library, provider, appendix | library owner / this repo | N-A, landed PR #12 | none |
| typed launch-plane hook (§2.1) | launch-plane owner / skill-agents-management | optional N-B migration / N-C1 | N-A tag; environment decision under adopted Decision 0019 |
| direct `curator run --network` | launcher owner / curator-agent-launcher | N-B | N-A tag; Compose-first allowed; remove final Compose patch only after overlay/parity proof |
| tracked network | launcher + ax implementation owners; session-host bridge owner | after N-B | destination unset, resolution, drift and negotiated capability |
| task-board runner | spawn-runtime owner / skill-project-management | N-C1 | M2; inherited N3/N4 decision before use |
| tb-sessiond hosts | session-plane owner / skill-project-management | N-C1 | M3; LP Phase 0 for its own plans |
| runtime/project/operator defaults | selection + launch-plane owners | N-C2 | M4 operator layer and bindings catalog |
| trust-store confirm | trust owner / curator-trust | after N-A | curator-trust §10 |
| managed gateway | library/gateway owner / this repo | N-D | N-A; no N-B/C dependency |

Consume the library **by tag**, never `replace` or `go.work`. **Q6:** ownership stays unchanged; the Curator and launcher owners coordinate N-B; the launcher owner leads its implementation; consumer representatives advise (W/decisions.md:458–460).

## 4. Invariants every consumer keeps

1. Apply last on the process-creating host, to final child env; never `os.Setenv`, shell profiles, shared tmux env, user settings or system proxy.
2. Set values are owned non-secret literals; proxy names never travel as env_names, including mixed case.
3. Selection carries an authorized id, never a proxy URL; resolve locally and never infer a ref from ambient proxy values.
4. Refusals stop workload spawn/preflight. No hidden fallback to direct, another profile/account or automatic recovery on drift; mid-run drops go to the run owner.
5. Manifests/logs keep Record only, never Binding/Patch, env, credentials, endpoints or request bodies.
6. Control-plane helpers (stdio MCP, Apiary bridge, board-server client) configure transport explicitly, outside inherited harness proxies (N11).
7. No selection preserves today's unmanaged behavior exactly. It does not guarantee direct egress or authorize opting out of inherited selection.

## 5. Decision card after advice

All recommendations remain DRAFT pending Ivan's card-C decision: **Q1** typed data/final-overlay proof (§2.1); **Q2** durable parent identity/ref and N3/N4 inherited amendment (§2.2); **Q3** terminal retry drift with adapter-specific attempt records (§2.2); **Q4** independent fresh sessions, equal-or-refuse existing hosts (§2.3); **Q5** negotiated destination enforcement and Record extension (§2.5); **Q6** unchanged role ownership (§3); **Q7** verified tuples, Muse unsupported (§2.1). Q1–Q5 and Q7 are **recommended; Ivan decides**. Explicit direct and suite inherited-route evidence remain TODO(decision).

### Changes in rev 2

- Corrected/pinned drifted anchors and added exec audits; Decision 0019 is adopted, with environment ownership still open (§1/§2.1–§2.6).
- Moved preparation/persistence to the runner, covered early probes, durable inheritance and terminal retries; qualified existing plumbing and rejected the board-client HTTP proxy claim with nil-Proxy evidence (advice §2.2(1–2), Q2–Q3).
- Added suite separation/evidence decision and rollback/round-trip requirements; retained normalized credential refusal and Record-only serialization (advice §2.2(3–5)).
- Added reconciliation, local selector resolution/protocol bump, profiled-startup refusal and session-patched preflight/assay; corrected the fresh-session premise (advice §2.3(1–4), Q4).
- Replaced unanswered questions with card-C recommendations, deferred explicit direct, corrected mixed-case reservation, trust dispatch, N-A status, overlay migration and N-D dependency; replaced the ownership-transfer assumption with unchanged role ownership (Q1, Q5–Q7).
