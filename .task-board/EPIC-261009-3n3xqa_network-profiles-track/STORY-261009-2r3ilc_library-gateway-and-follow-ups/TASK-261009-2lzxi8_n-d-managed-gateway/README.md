# TASK-261009-2lzxi8: n-d-managed-gateway

## Description
An optional managed backend behind the pkg/gateway interface from N-A: acquire, inspect, release. The lease belongs to the owner of the actual runtime; one agent finishing never stops the others' proxy; a configuration change creates a new gateway generation and never moves active sessions silently (N14). First backend candidate: sing-box with a loopback mixed listener, set_system_proxy false, no TUN, WireGuard without a system interface, a pinned version (the spec section 6 leaves the choice to the implementer). After it, a separate enforced-sandbox epic. Not started. Dev issue #10.

## Scope
(define task scope)

## Acceptance Criteria
Two runtimes share one managed gateway through leases; a config change creates generation N+1 while sessions on N keep working; tagged release.
