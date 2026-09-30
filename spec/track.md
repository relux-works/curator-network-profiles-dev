# Track: network profiles

Status: **DRAFT**, 2026-09-27 (command names updated to the adopted command-line shape). Specification: `spec/network-profiles.md` (draft, N1–N14, two independent reviews folded in). Roadmap: milestone M7, after agents-infra is archived (operator decision 2026-09-23: network after launch profiles).

## Why

Several agents on one machine must reach the network through different application-level egresses (VPN A, VPN B, a corporate proxy). None of this may change the orchestrator, sibling agents or the operating system.

## What we are building

- A library `curator-network-profiles` (own public repository; consumed by task-board and the public `curator-run`): resolve → validate → probe → patch.
- `--network <name>` on `curator run` and `task-board spawn` (`task-board claude|codex` are aliases of `curator run`); defaults from the operator layer (agent selection) and runtime bindings; the machine's network profiles in `~/.curator/network.toml`, managed with the Curator provider `curator-network` (`curator network list|show|add|remove|check [--probe]|confirm|exec`).
- The patch is applied by the host that creates the process (`unset` then `set`), never through a shared environment; bindings are compared by digest; a resume under a changed profile is refused.

## Milestones

| Step | Delivers | Done when | Tracking |
| --- | --- | --- | --- |
| N-A | library, the file `~/.curator/network.toml`, the provider `curator-network` with `curator network exec <name> -- <cmd>` and `check --probe`, contract appendix (digest, `EnvPatch`, equality) | two manual launches use two egresses | in progress (2026-09-30) |
| N-B | `curator run --network` | the launcher applies the patch through the module | #7 |
| N-C1 | explicit profile on every task-board and tb-sessiond process path, run manifests | a child and a board session use the profile, including retry and resume | #8 |
| N-C2 | defaults from the operator layer, runtime bindings and the project | spawns get their egress without a flag | #9 |
| N-D | managed gateway (sing-box), then the enforced sandbox epic | automatic proxy provisioning | #10 |

## Integration

How each process owner applies the patch, where the binding is recorded, and how a resume under a changed profile is refused: `docs/integration-contract.md` (a proposal sent for advice under rule 27 on 2026-09-30; Ivan decides). The normative digest, `EnvPatch` and equality rules are in `spec/contract-appendix.md`, delivered with N-A.

## Dependencies

N-C1 needs M2 and M3 (task-board and tb-sessiond build launches through the module); N-C2 needs M4 (operator layer and bindings). The Apiary tool bridge must keep its own transport outside the harness's profile (NP-N11).

## Repository

This repository, `curator-network-profiles` (created 2026-09-24, renamed from curator-network the same day; private until the operator decides its visibility, proposed public). The draft specification is `spec/network-profiles.md`.
