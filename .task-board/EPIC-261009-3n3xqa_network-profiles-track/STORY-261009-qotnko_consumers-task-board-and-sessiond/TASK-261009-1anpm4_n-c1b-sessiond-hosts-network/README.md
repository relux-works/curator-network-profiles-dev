# TASK-261009-1anpm4: n-c1b-sessiond-hosts-network

## Description
tb-sessiond hosts apply the network profile: the codex app-server host and the claude PTY host apply the EnvPatch last to command.Env, record the binding in the host snapshot, and run the N7 entrypoint check when the host starts. Split from N-C1 under tb-R151; the split element TASK-261002-s2rz0z named in dev issue #8 does not exist on the shared board today, so file it there when the work is scheduled.

Blocked on the session-host extraction (M3 / LP Phase 0): the hosts are being moved out of tb-sessiond into the session host, and the patch should land where the host ends up.

## Scope
(define task scope)

## Acceptance Criteria
A hosted Codex and a hosted Claude session each run under a named profile, the host snapshot shows the binding, and a host restart re-resolves it.
