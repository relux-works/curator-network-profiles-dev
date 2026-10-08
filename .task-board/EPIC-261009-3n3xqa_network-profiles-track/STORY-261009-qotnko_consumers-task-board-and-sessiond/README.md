# STORY-261009-qotnko: consumers-task-board-and-sessiond

## Description
Wire the library into every process-creating path that task-board and tb-sessiond own, and make defaults work without a flag. Library side is done (v0.3.1); this story is consumer work in other repositories.

## Scope
task-board spawn, tb-sessiond hosts, default resolution, hosted launch CR2-CR5.

## Acceptance Criteria
A tracked child, a hosted session and a defaulted spawn all get their egress from a network profile, with the binding and its origin recorded.
