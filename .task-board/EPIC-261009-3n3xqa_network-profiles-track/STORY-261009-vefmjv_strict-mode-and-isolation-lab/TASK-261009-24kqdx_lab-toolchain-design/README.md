# TASK-261009-24kqdx: lab-toolchain-design

## Description
Design the isolation-lab toolchain before any experiment: workflows on GitHub-hosted runners for macOS, Linux and Windows (sudo available, disposable machines, nothing on our own hosts), how a stand is provisioned and torn down, how evidence is collected and finalized by root, and how results are published. Joint with Alexis's side (mode S, VISION 4.7, O3). Constraint from the Mac mini rules: no executables built or run on the mini (R194, R223 runners-first).

## Scope
(define task scope)

## Acceptance Criteria
A reviewed design document and a workflow skeleton that boots one stand per OS.
