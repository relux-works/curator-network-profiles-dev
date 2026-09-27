# curator-network-profiles

Per-launch network profiles for coding agents. A network profile is a named, machine-local description of the application-level proxy through which a harness's supported network clients connect. The process owner that starts the harness resolves the profile, validates and probes it, and applies an environment patch immediately before spawn. Several agents on one machine can then use different egresses at the same time, without changing the orchestrator, sibling agents or the operating system. The first release is cooperative proxy routing of supported clients, not an enforced sandbox.

**Status: DRAFT design. No code yet; nothing here is normative.**

## Layout

| Path | Content |
| --- | --- |
| `spec/network-profiles.md` | draft specification (decisions N1–N14) |
| `spec/track.md` | delivery track (steps N-A to N-D) |
| `spec/network-profiles.ru.md`, `spec/track.ru.md` | Russian translations (the English files are canonical) |

## How it fits

- A Go library (`resolve → validate → probe → patch`) with no daemon; it creates no processes and imports none of its callers.
- Its callers are the process owners: `curator-run` for interactive launches, task-board for tracked children, tb-sessiond for board sessions. Each applies the patch on every process-creation path it owns.
- The patch reaches argv and environment through the agents-management launch plane (`skill-agents-management`), the one place where launches are built (proposed Decision 0019).
- Selected with `--network <name>`; defaults come from the operator layer of agent selection and from runtime bindings. The machine's network profiles live in `~/.curator/network.toml` and are managed with `curator network …`, the Curator provider `curator-network` (the command-line shape adopted on 2026-09-27).
- Roadmap: milestone M7 in `wiki/roadmap/ecosystem-roadmap.md`.
