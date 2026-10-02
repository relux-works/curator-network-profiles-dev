# Track: network profiles

Status: **DRAFT**, 2026-09-27 (command names updated to the adopted command-line shape). Specification: `spec/network-profiles.md` (draft, N1–N14, two independent reviews folded in). Roadmap: milestone M7, after agents-infra is archived (operator decision 2026-09-23: network after launch profiles).

## Why

Several agents on one machine must reach the network through different application-level egresses (VPN A, VPN B, a corporate proxy). None of this may change the orchestrator, sibling agents or the operating system.

## What we are building

- A library `curator-network-profiles` (own public repository; consumed by task-board and the public `curator-run`): resolve → validate → conditional probe → patch; direct and plans/dry runs skip all probes (tcp/connect/tls = `skipped`).
- `--network <name>` on `curator run` and `task-board spawn` (`task-board claude|codex` are aliases of `curator run`); defaults from the operator layer (agent selection) and runtime bindings; the machine's network profiles in `~/.curator/network.toml`, managed with the Curator provider `curator-network` (`curator network list|show|add|remove|check [--probe]|confirm|exec`).
- The patch is applied by the host that creates the process (`unset` then `set` for proxy profiles; direct is unset-only with no set), never through a shared environment; bindings are compared by digest; a resume under a changed profile is refused.

## Milestones

| Step | Delivers | Done when | Tracking |
| --- | --- | --- | --- |
| N-A | library, the file `~/.curator/network.toml`, the provider `curator-network` with `curator network exec <name> -- <cmd>` and `check --probe`, contract appendix (digest, `EnvPatch`, equality) | two manual launches use two egresses | landed (PR #12, 2026-10-01) |
| N-B | `curator run --network` | the launcher applies the patch through this library | [#7](https://github.com/relux-works/curator-network-profiles/issues/7) |
| N-C1a | explicit profile on task-board subagent spawn paths, run manifests; depends on D4 + v0.2.0 only | a child uses the profile, including retry and resume | [#8](https://github.com/relux-works/curator-network-profiles/issues/8), N-C1 TASK-261002-re5ew4; D4 TASK-261002-143e84 |
| N-C1b | explicit profile on tb-sessiond host process paths; after M3 / LP Phase 0 | a board session uses the profile, including resume and host reuse | [#8](https://github.com/relux-works/curator-network-profiles/issues/8), N-C1 TASK-261002-re5ew4 |
| N-C2 | defaults from the operator layer, runtime bindings and the project | spawns get their egress without a flag | [#9](https://github.com/relux-works/curator-network-profiles/issues/9) |
| N-D | managed gateway (sing-box), then the enforced sandbox epic | automatic proxy provisioning | [#10](https://github.com/relux-works/curator-network-profiles/issues/10) |

## Integration

How each process owner applies the patch, where the binding is recorded, and how a resume under a changed profile is refused: `docs/integration-contract.md` (DRAFT rev 2, 2026-10-02; rule-27 advice closed 2026-10-01T16:00Z; Ivan decides (card C)). The normative digest, `EnvPatch` and equality rules are in `spec/contract-appendix.md`, delivered with N-A.

## Dependencies

Per tb-R151 (2026-10-02 07:21Z), N-C1 is split: N-C1a (subagent spawn) needs D4 (TASK-261002-143e84) + v0.2.0 only; N-C1b (tb-sessiond hosts) follows M3 / LP Phase 0. N-C1 remains TASK-261002-re5ew4; N-B is TASK-261002-1g5dyh. Network-profile consumers get priority after the R131 top items. N-C2 needs M4 (operator layer and bindings). The Apiary tool bridge must keep its own transport outside the harness's profile (NP-N11).

## Repository

This repository, `curator-network-profiles` (created 2026-09-24, renamed from curator-network the same day; private until the operator decides its visibility, proposed public). The draft specification is `spec/network-profiles.md`.
