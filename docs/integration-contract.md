# Integration contract (proposal): how the process owners apply a network profile

Status: **PROPOSAL for advice** (rule 27), 2026-09-30. Author: ivan-network-profiles. Decider: Ivan (accountable), through ivan-tb-keeper and ivan-desk. The owners named below advise; they do not decide, and they keep the final say over changes inside their own repositories when the integration lands there (N-B, N-C1, N-C2). Nothing in this document edits another repository.

Normative inputs: `spec/network-profiles.md` (N1–N14) and `spec/contract-appendix.md` (digest, EnvPatch, equality; written with N-A). Where this document and the appendix disagree on a byte, the appendix wins.

---

## 1. What the library gives every consumer

The Go module `github.com/relux-works/curator-network-profiles` (public packages under `pkg/`). It creates no processes, runs no daemon and imports none of its callers (N2). Every consumer runs the same five steps, **on the host that creates the process, immediately before spawn or resume**:

```text
Selection ──► resolve ──► validate ──► preflight (bounded) ──► bind ──► patch.Apply(childEnv) ──► spawn ──► record
 (ref+origin)   N4          N3/N12        N10                   N3        N5 (unset, then set)            Record (N8)
```

| Step | Input | Output | Refusal codes |
| --- | --- | --- | --- |
| resolve | explicit / runtime-default / project-default refs from the caller; the operator default and the confirmations come from the machine's own `~/.curator/network.toml` | the profile, its origin, or `unmanaged` | `network_profile_unknown`, `network_profile_denied`, `network_file_unreadable`* |
| validate | profile, the caller's loopback engine hosts (N12), the required assurance | ok | `network_profile_invalid`*, `network_configuration_conflict`, `network_scope_unsupported` |
| preflight | endpoint, optional probe target, deadline | the probe facts (tcp, connect, tls) | `network_proxy_unreachable`, `network_proxy_auth_failed` |
| bind | profile + the adapter identity the caller supplies (harness, build, entrypoint, adapter) | `Binding` | none |
| patch | the child environment the owner has **already composed and filtered** | the new env: `unset` (case-insensitive proxy family) then `set` | none |
| record | binding + probe | a `Record` for the run manifest (never env, never credentials) | none |
| reattach / resume | the recorded `Record`, a fresh binding, the event kind | ok / attach | `network_profile_drift`, `network_scope_unsupported`, `network_profile_unknown`, `network_profile_denied` |

\* marked TODO(decision) in the N-A PR: codes the spec does not name.

`unmanaged` (no selection anywhere) returns an empty patch: today's behaviour, byte for byte (§3 acceptance, "compatibility with existing runs").

**The one rule that decides every insertion point below:** the patch is applied to the **final** child environment, after the harness plugin's own filtering and after every overlay; the `set` half then travels as **owned literals** (`env_literals`), never as `env_names` (N5, 0013 D6.3). Applying it earlier breaks two plugins in opposite ways: muse's closed allowlist would drop the `set` values, and the pass-through plugins (pi, gemini, agy) would keep an ambient proxy the `unset` never saw.

### The record (what manifests keep)

```json
{
  "network": {
    "schema": "relux-network-binding-record-v1",
    "profile_ref": "egress-a",
    "origin": "explicit",
    "profile_digest": "sha256:…",
    "adapter_identity": {"adapter": "generic-env-v1", "harness": "codex", "build": "0.x.y", "entrypoint": "exec"},
    "assurance": "cooperative",
    "probe": {"tcp": true, "connect": "ok|skipped|failed", "tls": "ok|skipped|failed", "checked_at": "RFC3339"}
  }
}
```

Absent key = unmanaged. The endpoint is not recorded (the digest binds it); no env, no `Proxy-Authorization`, no secret.

### Equality and resume (N3, N7)

Two bindings are equal when `profile_digest`, `adapter_identity` and `assurance` are equal. The name proves nothing. On reattach or resume the host re-resolves the recorded `profile_ref` **fresh on that host** and calls the library's reattach check:

| Event | Result |
| --- | --- |
| resume/reattach, fresh digest = recorded | proceed |
| resume/reattach, same name, different digest | refuse `network_profile_drift`; no per-session re-routing; the operator restores the profile or starts a new launch |
| resume/reattach, profile gone / not allowed | `network_profile_unknown` / `network_profile_denied` |
| new assignment on a running host, binding equal | attach |
| new assignment on a running host, binding differs, no per-session route | refuse `network_scope_unsupported` |

---

## 2. Per consumer: where, who, what

### 2.1 Launch plane: `skill-agents-management` (`pkg/agentic`, `pkg/vendorplugin`)
**Advice from:** ivan-tb-agents-infra-exit. **Lands in:** N-B (the launcher consumes it) and N-C1.

- Today: `LaunchRequest.Env` (system.go:560) is the parent env; `plan.go:449` `sys.ChildEnv(req.Env, req)` produces the full child env; `PlanWithEnvironment.OwnedEnv` (plan.go:469-476) is the owned snapshot. No post-filter hook exists.
- Proposal (under the proposed Decision 0019, "the patch reaches argv and env through the agents-management module"):
  1. `LaunchRequest` (and `vendorplugin.SpawnRequest`) gain an optional `Network *binding.Binding` (or just its `EnvPatch` plus `Record`).
  2. `BuildPlan` applies `patch.Apply` **right after** `ChildEnv` (plan.go:449-452), and adds the `set` half to `OwnedEnv` (:469-476), so the proxy values become tracked `env_literals`.
  3. Each system plugin declares its **adapter**: the generic adapter for claude, codex, qwen, pi, pinative, gemini, agy; a plugin whose env contract cannot carry the patch returns `network_scope_unsupported` until verified. **muse** (a closed 21-name allowlist that drops every proxy name, `muse/env_test.go:205`) is `network_scope_unsupported` in the first release.
  4. Verified per-harness additions (`ALL_PROXY`, websocket variables, native settings overrides) live in the plugin's adapter, never in the generic one (N5).
  5. `Plan.Provenance` carries the `Record`.
- The module keeps owning no exec; the owners below still create the process.

### 2.2 task-board `internal/spawn` and the spawn runner: `skill-project-management`
**Advice from:** ivan-tb-critical-fixes. **Lands in:** N-C1 (explicit `--network`), N-C2 (defaults).

- **Where the host is.** The detached spawn runner (spawnruntime/runtime.go:4764-4766) inherits the orchestrator's env, and every child is created from it. **The runner is the host that runs `unset`**, not the orchestrator process.
- **Insertion point.** `launch_plan.go:488`: after `RuntimeControlEnvironment`, before `return plan`. It is the single choke point for the real launch (`planCommand`, spawn.go:1063-1071) and the dry run (`BuildArgs`, spawn.go:1414). Once 2.1 lands, the call moves into `BuildPlan` and this site only passes the binding.
- **Selection.** `task-board spawn … --network <name>` sets `spawn.Config.Network = {ref, origin: explicit}`; N-C2 adds `runtimes.toml` `network = "<name>"` (runtime-default) and `spawn.network.default` (project-default). The orchestrator passes a profile **id**, never a URL (§4).
- **Manifest.** `spawnRunManifest` (runtime.go:325-440) gains a `network` Record beside `runtime_provenance` (:337). Copy it in `newSpawnRunManifest`, `spawnConfigFromManifest` (:3205-3259) and `cloneSpawnRunSuccessor` (directives.go:895-968). Do not reuse `NetworkAccess bool` (:403): it is the codex sandbox flag.
- **Retry** (`retryLaunchConfig`, limit_retry.go:408): a retry may switch the agent, which changes `adapter_identity`, so it re-resolves and records a new binding; the same ref with a different digest is `network_profile_drift` (a retry is a new attempt of the same run, not a new launch; question Q3).
- **Successor / recovery:** reattach check against the recorded Record before `buildLaunchPlan`.
- **Recursive children.** A child's own `task-board spawn` must inherit **by reference plus re-resolution**, never by the patched env (N8). Proposal: the runner exports `TASK_BOARD_NETWORK=<ref>` (and `TASK_BOARD_NETWORK_ORIGIN`) as run-context variables beside `TASK_BOARD_*` (runcontext.go:31-44); the child's resolve treats it as the inherited default; a parent on A may still spawn a child on B with an explicit `--network`. Question Q2.
- **Managed goal-bound runs** hand off to tb-sessiond (`cfg.ManagedSessionLaunch`, codex_goal_spawn.go:152): the binding travels in the launch request, and tb-sessiond applies it (2.3).

### 2.3 tb-sessiond: `skill-project-management` (`internal/sessionmanager`, `cmd/tb-sessiond`)
**Advice from:** ivan-tb-critical-fixes. **Lands in:** N-C1.

- **Today** the hosts build env from the **daemon's** `os.Environ()`, not the agentic plan: codex app-server (codex_host.go:160-185, `codexHostEnvironment` :261-268), claude PTY (claude_host.go:700-727, `claudeHostEnvironment` :2427-2450). The daemon re-execs itself with `os.Environ()` (detached_launch.go:72-76), so it keeps the env of the shell that first started it: the same hazard as Claude's shared `--bg` supervisor (N7). The `unset` half is what makes that safe.
- **Insertion points.** One shared helper applied **last** to `command.Env`: codex_host.go ~:178 (after `TMPDIR`) and claude_host.go ~:711 (after `applyManagedSessionEnvironment`). Audit list (every exec site must apply it or be proven harness-free): claude preflight (claude_host.go:2416-2420), assay (claude_context_assay.go:580-582), codex context (codex_context.go:1376-1382), legacy codex (cmd/codex.go:189, 238-252), visible clients (cmd/codex.go:431-436, cmd/codex_manager.go:270-272).
- **Carrier.** `OwnerRunContext` (launch_plan.go:123-136) for owner runs; `PrimarySessionLaunchPlan` (:74-86) / `CodexSessionLaunch` (:138) for primaries.
- **Record.** In the host snapshots (`claudeHostSnapshot` ~:839, `codexHostSnapshot` ~:336) stored in `ProviderHostRecord.Opaque`.
- **Resume / reattach.** The N7 check runs in `ensureHostLocked` (manager.go:1584) **before** both `Restore` (:1625) and `access.Launch` (:1678-1690). The codex app-server `Restore` reattaches to a live process without spawning: that is exactly the "new assignment on a running host" row, so it attaches only on an equal binding.

### 2.4 curator-run: `curator-agent-launcher`
**Advice from:** ivan-tb-agents-infra-exit (today's owner) and m1max_a2a-swarm_orch (Alexey's side, which takes Curator and the launcher from 2026-10-01). **Lands in:** N-B.

- **Flag.** `--network <name>` in `valueFlags` (internal/cli/cli.go:223-231), `Invocation` (:162-195) and the SPEC §3 table.
- **Resolve + preflight** after `plan.Build` (cmd/curator-run/main.go:253), before `ComposeAdmittedPlan` (:283): a refusal stops the launch with the typed code; no fallback to direct.
- **Patch** in `Compose` after the last overlay (composition.go ~:121): delete the unset names from both `env` and `literals`, add the set values to both. Direct exec (execution.go:154) then gets the patched env.
- **Tracked mode (ax).** Only the `set` half can travel (N9). The launcher adds an extension key (proposed `works.relux.curator.network`, carrying the Record) to the `extensions` struct (execution.go:52-58). **Until the destination host applies the `unset` half itself, `--network` in tracked mode refuses** (`network_scope_unsupported`), and direct mode ships alone (N9).
- Curator's own preparation (`curator env resolve`, install, repair) is not proxied (N5).

### 2.5 ax launch plan and the session host: `agent-session-host`, curator-spec 0013
**Advice from:** m1max_a2a-swarm_orch (curator-spec side) and ivan-tb-agents-infra-exit. **Lands in:** after N-B, before tracked `--network`.

- The `set` half travels in `env_literals` (non-secret, disjoint from `env_names`; 0013:187-200). Proxy names are already reserved from `env_names` across the ecosystem (curator contextpkg.go:418, agent-mcp-v1 schema), so no name can arrive both ways.
- On every resume, 0013:298-307 replays `env_literals` from the record verbatim. A replayed proxy literal alone would silently resume under the old endpoint after the profile changed, so the destination host must **re-resolve the recorded `profile_ref`, compare digests, and refuse `network_profile_drift`** (the precedent is `environment_drift`, 0013:625-640), then rebuild the child env: its own `unset`, then the fresh `set`.
- The ax spec proper (§5.1, §14.1) is not in the reference clones. Question Q5.

### 2.6 Curator umbrella and trust
**Advice from:** m1max_a2a-swarm_orch.
- `curator network …` dispatches to the provider `curator-network` on the trust roots (umbrella.go:120, curator-spec environments §11); no provider flags are required, and N-A still ships `--json` and a real `--version`.
- Provider installs follow curator-spec#100 / curator#96.
- `curator network confirm` (the widening gate, curator-trust §10) needs the presence-key signing, which does not exist yet. N-A ships a local confirmations ledger bound to the profile digest, refused inside agent sessions: TODO(decision) in the N-A PR. When curator-trust lands, the ledger is replaced and the resolver reads the trust store instead.

---

## 3. Ownership and order

| Piece | Owner | Milestone | Depends on |
| --- | --- | --- | --- |
| library, provider `curator-network`, contract appendix | this repo | N-A | none |
| launch-plane hook (2.1) | skill-agents-management | N-B / N-C1 | N-A tag; proposed Decision 0019 |
| `curator run --network`, direct mode (2.4) | curator-agent-launcher | N-B | N-A tag (2.1 optional: the launcher can patch in `Compose` first) |
| tracked `--network` (2.4, 2.5) | launcher + ax | after N-B | host-side `unset` + drift check in ax |
| task-board spawn + runner (2.2) | skill-project-management | N-C1 | M2 (task-board builds launches through the module) |
| tb-sessiond hosts (2.3) | skill-project-management | N-C1 | M3; LP Phase 0 where tb-sessiond composes its own plans |
| defaults: runtime binding, project, operator (2.2) | skill-project-management + agents-management | N-C2 | M4 (operator layer, bindings catalog) |
| confirm on the trust store (2.6) | curator-trust | after N-A | curator-trust §10 |
| managed gateway | this repo | N-D | none of the above |

The library is consumed **by tag**, never by `replace` or `go.work`.

## 4. Invariants every consumer keeps

1. The patch is applied on the host that creates the process, last, to the final child env; never `os.Setenv`, shell profiles, a shared tmux environment, a user `settings.json`, or the system proxy.
2. The `set` half is owned literals; proxy names never travel as `env_names`.
3. The orchestrator passes a profile id from the allowed set, never a raw proxy URL.
4. Any refusal stops the launch before spawn. No hidden fallback to direct, to another profile, or to another account. A mid-run drop goes to the run owner; nothing restarts unconditionally.
5. Manifests and logs keep the Record only: never env, `Authorization`, `Proxy-Authorization`, an endpoint with credentials, or request bodies.
6. A helper process that must reach a control plane by its own route (a stdio MCP server, the Apiary tool bridge) configures its outgoing client explicitly, never from the inherited proxy variables (N11).
7. No selection keeps today's behaviour exactly (`unmanaged`, empty patch).

## 5. Questions for the owners (advice requested)

- **Q1 (launch plane, agents-infra-exit):** is a `Network` member on `LaunchRequest`/`SpawnRequest`, applied right after `ChildEnv` in `BuildPlan`, the right shape for Decision 0019? Or should the plane expose a post-filter hook that the owners call instead?
- **Q2 (task-board, critical-fixes):** is `TASK_BOARD_NETWORK` (+ `_ORIGIN`) the right carrier for "inherit by reference" to recursive children, or should the reference ride in the existing run-context / goal-binding plumbing?
- **Q3 (task-board, critical-fixes):** a retry that keeps the agent but finds the profile changed: `network_profile_drift` (proposed: a retry is the same run), or a fresh binding (a retry is a new launch)?
- **Q4 (tb-sessiond, critical-fixes):** codex app-server hosts serve several threads. Can the host ever set a route per session, or is "equal binding or refuse" the only honest option for the first release (proposed: the latter)?
- **Q5 (ax / curator-spec, a2a-swarm side):** where in the ax record does the network Record live (an extension key, as proposed, or a first-class field in the next launch-plan schema), and who owns the host-side re-resolution on resume?
- **Q6 (launcher, both advisers):** with the ownership change on 2026-10-01, who takes N-B's launcher change?
- **Q7 (all):** muse: is `network_scope_unsupported` acceptable for the first release, or should its allowlist admit the six `set` names now?

Reply-by: **2026-10-01T16:00Z**, in #skill-project-management › `network-profiles-integration`. After the reply-by I send ivan-tb-keeper the proposal, the advice (quoted, objections flagged) and my recommendation, for Ivan's decision via ivan-desk.
