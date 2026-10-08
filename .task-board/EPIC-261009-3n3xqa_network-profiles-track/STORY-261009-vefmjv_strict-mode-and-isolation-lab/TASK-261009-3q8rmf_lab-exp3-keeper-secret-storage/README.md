# TASK-261009-3q8rmf: lab-exp3-keeper-secret-storage

## Description
Experiment 3: secret storage reachable only by the key keeper process: the System keychain from a LaunchDaemon on macOS, TPM2 on Linux, DPAPI/CNG on Windows. Measure that another user, including the agent users, cannot read or use the secret.

## Scope
(define task scope)

## Acceptance Criteria
Per-OS evidence that only the keeper identity can use the stored secret.
