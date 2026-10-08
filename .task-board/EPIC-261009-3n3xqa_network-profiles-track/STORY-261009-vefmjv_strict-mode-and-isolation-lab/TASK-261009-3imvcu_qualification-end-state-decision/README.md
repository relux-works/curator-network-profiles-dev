# TASK-261009-3imvcu: qualification-end-state-decision

## Description
Owner decision (Ivan). End state of adapter qualification: A, central qualification with signed evidence; or B, a local sandbox behind an R141 review panel. Until then option C stays the default (optimistic with a provenance record; strict is one flag away and the default for sensitive_egress profiles). Strict mode refuses every build today because none has conformance evidence; nobody uses a sensitive_egress profile, so this blocks no one.

## Scope
(define task scope)

## Acceptance Criteria
Decision recorded in the wiki decisions.md and reflected in the library defaults.
