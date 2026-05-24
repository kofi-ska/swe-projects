# v3 Delivery

This file captures how changes are scoped and described for this system.

## Scope a change

- A change record states the problem in one paragraph.
- It lists what changes and what does not change.
- It names the risk and the rollback path.
- Public routes stay stable unless the change is explicitly about compatibility.

## Break down work

- One behavior change per change set is the default.
- Changes are split by subsystem when the risk differs.
- The hot path stays small.
- The public contract stays stable while internals change.
- Recovery work stays separate from unrelated refactors.

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

- The block is described in terms of what failed, what was tried, what is needed, and whether the block is local or external.
- Escalation carries facts.

## Trade-offs

- Short-term fixes are acceptable when the rollback path is explicit.
- Technical debt is repaid when it affects correctness, operability, or delivery speed.
- Observability and recovery work are not deferred when they block incident response.
