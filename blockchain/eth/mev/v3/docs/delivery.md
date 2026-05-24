# v3 Delivery

This file captures how changes are scoped and described for this system.

## Scope a change

- state the problem in one paragraph
- state what changes
- state what does not change
- name the risk
- name the rollback path
- keep public routes stable unless the change is explicitly about compatibility

## Break down work

- one behavior change per change set is the default
- split changes by subsystem when the risk differs
- keep the hot path small
- keep the public contract stable while internals change
- keep recovery work separate from unrelated refactors

## Proposal format

- problem
- proposal
- constraints
- implementation steps
- test plan
- observability changes
- alerting changes
- rollout steps
- rollback steps
- docs updates

## Delivery checks

- tests pass
- metrics exist
- alerts exist
- runbooks exist
- failure classes are named
- performance budget is not broken
- recovery path is clear
- public behavior is unchanged or explicitly versioned

## When blocked

- describe what failed
- describe what was tried
- describe what is needed
- state whether the block is local or external
- escalate with facts

## Trade-offs

- short-term fixes are acceptable when the rollback path is explicit
- technical debt is repaid when it affects correctness, operability, or delivery speed
- observability and recovery work are not deferred when they block incident response
