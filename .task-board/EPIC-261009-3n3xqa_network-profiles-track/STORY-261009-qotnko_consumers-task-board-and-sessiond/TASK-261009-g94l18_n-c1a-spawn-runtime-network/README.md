# TASK-261009-g94l18: n-c1a-spawn-runtime-network

## Description
POINTER to the shared board: TASK-261002-re5ew4 spawn-runtime-and-sessiond-consume-network-profiles (STORY-261002-1zlfo3, EPIC-261002-u3n7qh). Status there on 2026-10-09: blocked; tb-keeper's summary of 2026-10-08 puts it in the queue of the macmini critical-fixes orchestrator, about 4 hours after task dthlv6 lands.

What: task-board spawn ... --network <name>. The spawn runner, which is the host that creates the child, resolves the profile and applies the EnvPatch (unset, then set) in internal/spawn/launch_plan.go after RuntimeControlEnvironment. Retry, successor and recovery re-resolve the profile and check the recorded binding. Recursive children inherit by reference plus re-resolution (spec N8, the inherited origin of decision D2), never through the patched environment.

Unblocked by: the D4 launch-plane carrier (agents-management v0.5.41) and library v0.2.0 or later (now v0.3.1). Dev issue #8.

## Scope
(define task scope)

## Acceptance Criteria
Two spawns of one harness with --network A and --network B use two egresses at once; every refusal (unknown, denied, unreachable profile) stops the spawn with a typed reason; the run manifest records the binding.
