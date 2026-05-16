# MEV Relay v1

v1 is the first release. The goal is to prove the core relay loop:

1. accept a bundle
2. validate it
3. queue it
4. simulate it on Anvil
5. score it
6. persist the result
7. return a clear decision

## v1 Scope

**Included**

- public bundle submission
- request validation
- tracking ID assignment
- bounded queueing
- simulator worker execution
- Anvil as the default execution backend
- scoring and decision output
- storage of submissions and simulation results
- audit logging
- metrics and logs

**Not included**

- live mainnet relay connectivity
- microservices
- production HA
- validator integrations
- complex auction logic
- customer auth systems beyond basic request validation
- multi-region infrastructure

## Functional Requirements

- accept a bundle submission
- validate the request shape and required fields
- assign a tracking identifier
- enqueue the bundle for simulation
- execute the bundle against the configured backend
- capture simulation outcome and timing
- compute a score or decision
- persist the request, state, and result
- expose status
- reject malformed or unsupported input safely

## Non-Functional Requirements

- cheap to run
- easy to understand in one codebase
- simple to debug manually
- repeatable enough for development
- fail visibly
- avoid logging sensitive bundle contents
- avoid unbounded queues and retries

## v1 Test Plan

This test plan proves the relay core before we add more scope.

### 1. Request Validation

Bad submissions are rejected before they enter the queue.

Cases:

- missing `txs`
- empty `txs`
- malformed transaction payload
- missing `block_target`
- invalid `block_target` format
- missing signature
- invalid signature format
- unsupported fields
- oversized payload

Expected:

- request is rejected
- response is clear and stable
- no queue item is created
- no simulation is triggered
- rejection is logged without leaking bundle contents

### 2. Submission Identity

Each accepted bundle gets a stable identity.

Cases:

- first submission of a bundle
- repeated submission of the same bundle
- repeated submission after restart

Expected:

- tracking ID is assigned once
- duplicate handling is consistent
- idempotency behavior is defined
- stored records can be correlated with logs and metrics

### 3. Queue Behavior

The queue must behave safely under load.

Cases:

- single bundle enqueue
- burst of bundles
- queue at capacity
- queue overflow
- worker unavailable
- queue drain after backlog

Expected:

- queue accepts normal traffic
- bounded capacity is enforced
- overflow is handled by a defined policy
- no silent loss of accepted work
- queue depth is observable

### 4. Simulation Execution

Bundles can be executed against Anvil.

Cases:

- valid bundle with successful execution
- valid bundle with revert
- valid bundle with gas-heavy path
- bundle with unsupported transaction shape
- bundle targeting an invalid block height
- bundle simulated after fork reset

Expected:

- simulation result is captured
- timing is recorded
- success or failure is explicit
- backend errors are separated from bundle errors
- result storage is consistent with the observed execution

### 5. Scoring and Decisioning

The relay must produce a clear decision from simulation output.

Cases:

- profitable bundle
- unprofitable bundle
- bundle with neutral outcome
- bundle with simulation failure
- bundle with missing profitability signal

Expected:

- decision is one of accept, forward, or reject
- score is explainable
- decision is persisted
- decision is visible in logs and metrics

### 6. Persistence

Results are stored correctly.

Cases:

- submission persisted before simulation completes
- simulation result persisted after success
- rejection persisted after validation failure
- restart after partial write

Expected:

- records survive process restart
- stored state matches the relay outcome
- partial failure does not corrupt prior records
- database errors are handled explicitly

### 7. Observability

We can understand the system while it runs.

Cases:

- successful request path
- validation failure path
- queue overflow path
- simulation failure path
- backend unavailable path

Expected:

- structured logs contain bundle ID or tracking ID
- sensitive payload data is redacted
- metrics expose request count, queue depth, simulation latency, and decision counts
- health and readiness endpoints reflect service state

### 8. Failure Handling

The system fails in bounded ways.

Cases:

- simulator crash
- queue saturation
- database unavailable
- backend timeout
- malformed runtime config
- process restart mid-flight

Expected:

- failures are visible
- work is retried only when safe
- dead-letter or overflow behavior is defined
- the relay does not hang indefinitely
- operators can recover the system

### 9. Privacy and Leakage

Bundle contents do not leak unnecessarily.

Cases:

- normal request path
- validation failure
- simulation failure
- debug logging enabled
- metrics export

Expected:

- transaction payloads are not blindly printed
- logs carry metadata, not raw sensitive content
- tracing is scoped
- storage only retains what the product needs

### 10. Backend Swappability

The chain backend can be switched without rewriting relay logic.

Cases:

- default Anvil backend
- forked Anvil backend
- live RPC backend stub
- invalid backend config

Expected:

- relay logic stays the same
- backend is selected by configuration
- invalid config fails fast
- tests can run without live chain access

## Test Layers

- unit tests for validation, scoring, queue policies, and state transitions
- integration tests for relay to simulator to persistence
- happy-path end-to-end tests
- failure-path end-to-end tests
- restart and recovery tests
- backend adapter tests for Anvil and stubbed RPC behavior

## Exit Criteria For v1

v1 is done when:

- a bundle can move through the full relay loop
- failures are explicit and bounded
- simulation on Anvil works consistently
- results are persisted and observable
- the codebase is still simple enough to extend into v2
