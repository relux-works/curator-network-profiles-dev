# TASK-261009-1bgcaq: lab-exp2-per-uid-firewall

## Description
Experiment 2: per-user firewall rules: pf with user matching on macOS, nftables owner match on Linux, WFP on Windows. Goal: an agent user reaches only the message broker and the profile's egress proxy; everything else is dropped and counted.

## Scope
(define task scope)

## Acceptance Criteria
Measured allow and deny matrices per OS, with the rules as code.
