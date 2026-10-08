# TASK-261009-2d22l0: n-a-review-follow-ups

## Description
Left from the round-4 review of N-A (dev issue #13; N2, N3 and the confirm lock landed in PR #15):
- N1 (low): the type-mismatch decode message echoes the key path with an unsafe profile name from the operator's own file (example: [networks."u:pw@h"] with bypass_hosts = "x" prints networks.u:pw@h.bypass_hosts: wrong type). In pkg/netprofile classifyDecodeError, the de.Key() branch. Fix: apply the plain-name rule to each key segment, or print only the last key.
- Stale lock: a network.toml.lock left by a crash has no CLI cleanup (TODO(decision): an explicit unlock verb or an age-based takeover).

## Scope
(define task scope)

## Acceptance Criteria
Issue #13 closed with a test for N1 and a decided stale-lock behaviour.
