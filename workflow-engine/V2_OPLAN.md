# Workflow Engine V2 Plan

## Goal

Build a normal, production-safe workflow engine:

- one input in, one decision out
- no inline side effects
- no replay ambiguity
- no simulator/runtime drift
- no unbounded queues or dedupe growth

## Scope

V2 focuses on the runtime and the guarantees around it:

- durable state updates
- durable history
- durable idempotency
- durable scheduling
- effect execution through a separate path
- spec validation and linting
- CLI parity with runtime behavior

## What We Are Not Doing

- no general-purpose scripting
- no dynamic code execution
- no broad framework abstractions
- no feature creep beyond the core workflow contract
- no overbuilt architecture for its own sake

## Success Criteria

V2 is successful when all of these are true:

- `100%` of accepted inputs produce one durable commit.
- `100%` of retries for the same input are deduped.
- `100%` of scheduled work is persisted with a dedupe key.
- `100%` of effects run after commit, not inline.
- `100%` of simulator results match runtime decisions for the same spec, instance, and input.
- `0` known critical or high-severity defects remain open in the workflow engine package.
- `>= 90%` test coverage on `src/v1/core` and `src/v1/runtime`.

## Hard Rules

- Keep the decision core pure and deterministic.
- Fail closed on validation, persistence, quota, and policy errors.
- Pin instances to one `specId` and `specVersion`.
- Bound payload size, context size, guard complexity, and scheduled work.
- Use stable, documented reason codes for all rejections.
- Keep code small, direct, and easy to review.

## Main Work Items

### 1. Commit path

Make accepted inputs commit state, history, and scheduling data as one durable unit.

Done when:

- state is not advanced unless the commit succeeds
- history is not written without a matching durable result
- crash/restart does not duplicate the transition

### 2. Idempotency

Make dedupe durable and scoped to the workflow identity.

Done when:

- idempotency survives restart
- dedupe uses `workflowId` plus `eventId`, and `tenantId` when present
- retention is bounded

### 3. Scheduler

Make due work replay-safe and deduped.

Done when:

- due tasks can be claimed once
- duplicate schedule writes do not cause duplicate execution
- timer volume is capped by spec limits

### 4. Effects

Move effects to an outbox-style path.

Done when:

- no effect runs inline during input handling
- each effect has an idempotency key
- failures are observable and retryable

### 5. Simulator and CLI

Keep CLI simulation aligned with runtime behavior.

Done when:

- `simulate` and `run` agree on state, context, and rejection reason
- help text matches implemented commands
- outputs are stable and easy to parse

### 6. Validation and linting

Catch bad specs before runtime.

Done when:

- invalid specs fail closed
- risky specs produce warnings with a path and code
- permissions and limits cannot be bypassed silently

## Quantified Milestones

### Milestone 1: Core correctness

- runtime commit path
- durable idempotency
- simulator/runtime parity

Exit gate:

- `100%` of current tests pass
- `0` known high-severity runtime defects

### Milestone 2: Durable work

- scheduler dedupe
- effect outbox
- crash/restart tests

Exit gate:

- `0` duplicate effect executions in replay tests
- `0` duplicate scheduled jobs in restart tests

### Milestone 3: Production polish

- stronger validation
- clearer observability
- cleaner CLI output

Exit gate:

- `0` unsupported public code paths
- `100%` of runtime outcomes have documented reason codes

## Test Minimums

We should have at least:

- `1` test for every public command
- `1` replay test for every durable write type
- `1` negative test for every rejection reason
- `1` adversarial test for each known abuse class

## Open Tasks

1. Redesign the commit boundary.
2. Fix durable idempotency.
3. Add replay-safe scheduler handling.
4. Move effects to an outbox path.
5. Align simulator behavior with runtime.
6. Add crash/restart integration tests.
7. Tighten validation and reason codes.

