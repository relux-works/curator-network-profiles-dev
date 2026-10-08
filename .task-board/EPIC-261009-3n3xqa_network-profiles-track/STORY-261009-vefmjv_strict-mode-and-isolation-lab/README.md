# STORY-261009-vefmjv: strict-mode-and-isolation-lab

## Description
Strict adapter mode needs conformance evidence that the vendor code under test cannot forge. The first attempt (parked branch parked/qualify-claude) failed review three times for that reason. The isolation lab builds the missing process boundary on disposable GitHub-hosted runners (macOS, Linux, Windows; sudo is available there; our own machines are never touched). It is joint research with Alexis's side (mode S in VISION section 4.7 and O3). Design of the toolchain comes before any experiment.

## Scope
Lab toolchain design, seven experiments, the rebuilt first qualification, and the owner's choice of the qualification end state.

## Acceptance Criteria
At least one harness build is qualified with root-finalized evidence, and the owner has chosen end state A or B.
