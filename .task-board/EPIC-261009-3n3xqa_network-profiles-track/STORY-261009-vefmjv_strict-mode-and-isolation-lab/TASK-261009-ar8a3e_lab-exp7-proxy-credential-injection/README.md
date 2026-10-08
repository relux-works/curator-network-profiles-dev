# TASK-261009-ar8a3e: lab-exp7-proxy-credential-injection

## Description
Experiment 7 (research only, owners' call 2026-10-07, Q4): credential injection at the egress proxy. The proxy terminates TLS to the provider API with our CA trusted inside the agent user's environment and adds the authorization header from the credential broker, so the agent user never holds a credential. Measure per harness how the CA is accepted (NODE_EXTRA_CA_CERTS, SSL_CERT_FILE, pinning) and what breaks. Providers may flag intercepted traffic, so this never runs against real accounts without the owner's word.

## Scope
(define task scope)

## Acceptance Criteria
A per-harness compatibility table and a risk note for the owners.
