# TASK-261009-3er5k7: muse-meta-api-key-passthrough

## Description
POINTER to the shared board: TASK-261007-hjwgiz muse-plugin-meta-api-key-passthrough (STORY-261007-2uzr37). Status: backlog, taken when a window fits. META_API_KEY, which Muse prefers over its account login, does not reach spawned children today because task-board spawn does not pass it. SECURITY class: exact-name pass-through only, the environment allowlist is not widened.

## Scope
(define task scope)

## Acceptance Criteria
A spawned Muse child sees META_API_KEY and nothing else new.
