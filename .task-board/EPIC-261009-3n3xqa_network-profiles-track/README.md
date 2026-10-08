# EPIC-261009-3n3xqa: network-profiles-track

## Description
Remaining work of the network-profiles track (milestone M7 of the ecosystem roadmap), recorded on 2026-10-09 when the track was put on hold.

Done so far: the library curator-network-profiles v0.3.1 is public (relux-works/curator-network-profiles, main 6e35565): profile model and digest, proxy egress policy, adapter admission (pkg/adapterprobe: qualified, unqualified, refused; strict, optimistic and pinned modes), the embedded known-bad list, operator-local profile bindings (N-C2 library side), the pkg/hosted boundary for process-creating hosts, and option C (an unqualified known vendor line runs with an operator-readable provenance record). N-B is delivered: curator run --network in curator-agent-launcher (PRs #31 direct mode, #35 option C, #34 pin v0.3.1). CR1 of the hosted launch (wire negotiation, task-board PR #527) landed on 2026-10-07.

Where things live: this board (track view); the shared task-board board (relux-works/skill-project-management, .task-board) for task-board-side work (EPIC-261002-u3n7qh network-profiles-consumer-wiring, EPIC-261004-n98i2b hosted-curator-run-host-side; EPIC-261002-1ts1q4 network-profiles-consumers is finished and waits for its review); the wiki pages network-profiles/README.md (status) and network-profiles/RESUME.ru.md (resume memo); parked files in wiki research/network-profiles-parked/ and the branch parked/qualify-claude in this repository.

Rule: an element here that mirrors a task on the shared board is a pointer. The shared-board element is authoritative for status and evidence; update this pointer when it changes.

## Scope
Consumers of the library in task-board and tb-sessiond (N-C1, N-C2), the hosted launch CR2-CR5, the managed gateway (N-D), strict mode and qualification through the isolation lab, Muse network items, and library follow-ups.

## Acceptance Criteria
Every child is done or explicitly dropped by the owner, and the wiki status page says so.
