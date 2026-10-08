# TASK-261009-3w2z0o: n-c2-defaults-in-consumers

## Description
Defaults without a flag. N4 precedence: host locks and the allowed set, then explicit --network, then inherited (D2), then runtime-default (a binding's network = "<name>" in runtimes.toml), then project-default (spawn.network.default in the project configuration), then operator-default (default = "<name>" in ~/.curator/network.toml). The caller's CandidateResolver resolves these dependent defaults before routing (N13); the router never switches profiles. The origin is recorded in the run manifest; an explicit --network still overrides every default.

Library side shipped: operator-local profile bindings (public PR #4, released in v0.3.0). Remaining: task-board and curator-run read the defaults through the library and record the origin. Dev issue #9.

## Scope
(define task scope)

## Acceptance Criteria
A spawn without --network gets its egress from the right layer, the manifest shows origin, and an explicit flag overrides.
