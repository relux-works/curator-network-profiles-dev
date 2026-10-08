# TASK-261009-3agse4: muse-r7b-shell-children

## Description
POINTER to the shared board: TASK-261002-lfm3e9 muse-r7b-shell-child-network-env (STORY-261001-3hx77k). Status: backlog, blocked by R2 TASK-261001-3hz2af (the PTY host). Dev issue #18.

Ivan decided D8 = (A) on 2026-10-02: Muse 1.4.1-R4503.1 and 1.4.2-R4684.1 are supported with adapter muse-env-v1 (the generic patch, the set half written into each MCP server entry's env block, hook-command wrapping). Open gap R7b ("не забыть"): shell and tool children started by the model cannot be checked offline. Once R2 can drive the TUI ! escape, run the R7b row on both binaries with the non-forwarding sink proxy, the loopback-only sandbox with SIGKILL and temporary XDG directories (method: docs/harness-verification.md).

## Scope
(define task scope)

## Acceptance Criteria
The R7b row passes on both Muse binaries or the adapter is narrowed.
