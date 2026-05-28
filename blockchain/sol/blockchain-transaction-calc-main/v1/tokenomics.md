# Swap Economics

This doc describes the off-chain economics and math for Solana swap workflows.

It is not the swap itself.
It is the decision layer before the swap:
- quote
- route scoring
- slippage estimation
- fee estimation
- breakeven
- accept / defer / reject

Why this exists:
- do not send bad swaps
- do not waste fees
- do not execute duplicate swaps
- do not route through bad liquidity
- do not execute when slippage or risk is too high

It uses the same execution model as [`REALITY.md`](./REALITY.md):
- Scala orchestrator
- request ID / dedupe key
- bounded queue
- durable decision record
- gRPC boundary to Rust for v1
- replay-safe terminal outcome

Operational contract:
- every request is typed
- every decision is observable
- every terminal outcome is durable
- every quote is tied to source hashes and slot freshness
- every duplicate request resolves to the recorded terminal outcome
- every stale or conflicting source resolves to defer or reject

Operational truth:
- the service is not the swap
- the service is not the oracle
- the service is not the on-chain execution engine
- the service decides whether the swap is worth forwarding under fresh data
- any estimate becomes a decision only if it survives freshness, source, and risk checks
- one live request is one swap-preflight decision request
- the request is keyed by request ID and dedupe key
- the request carries token pair, amount, route candidates, source hashes, and freshness state
- the output is one of accept, defer, or reject

## Problem

Decide whether a proposed swap is worth forwarding. This is a finite-state economic decision process, not the swap itself.

Typical questions:
- what amount out should we expect
- how much slippage should we tolerate
- which route is best
- what fee budget is acceptable
- what is the breakeven point
- is the request safe to forward

Economic reality:
- a swap request is only worth forwarding if its expected net value after fees, slippage, execution cost, and risk penalty is positive
- stale quotes, stale liquidity, or conflicting sources are not acceptable inputs to an accept decision
- duplicate requests must resolve to the recorded terminal outcome
- the machine must end in accept, defer, or reject; there is no soft success
- a positive estimate is not a promise of execution; it is only a forwarding decision
- the economic output is a forwardability verdict, not a fill guarantee

State machine reality:
- requests move through `received -> normalized -> priced -> routed -> scored -> decisioned -> persisted -> audited -> terminal`
- every transition has a guard, a cost, and a terminal outcome
- route scoring and breakeven are numeric transitions, not commentary
- the state machine is the control loop; economics determine whether the next transition is allowed
- any transition without fresh source data is invalid
- a quote is actionable only when the source hashes, slot age, and model version are recorded

## Typed model

Typed values:
- `RequestId`: opaque string, unique per swap request
- `DedupeKey`: opaque string, idempotency key
- `TokenAmount`: numeric, token base units or normalized decimals
- `Price`: numeric, quote currency per token
- `FeeLamports`: numeric, lamports
- `Bps`: integer, basis points
- `Slot`: integer Solana slot
- `SlotAge`: current slot minus quote slot
- `RouteId`: opaque string, AMM / aggregator / venue route
- `RiskScore`: normalized scalar
- `Quote`: struct with expected output, fee, slippage, route id
- `Decision`: enum with `accept`, `defer`, `reject`

Typing rules:
- all prices are explicit about quote currency
- all token amounts are explicit about base units or normalized decimals
- freshness is typed as slot age or timestamp age
- route identity is explicit in every quote
- request identity, decision identity, model version, slot, quote age, and source hashes must be recorded with the terminal decision

## Core quantities

- `x`: input amount in token base units
- `y`: expected output amount in token base units
- `p_in`: input token price in quote currency
- `p_out`: output token price in quote currency
- `L_t`: liquidity depth at time `t`
- `R_t`: reserve state at time `t`
- `F_t`: fee at time `t` in lamports or quote currency
- `C_t`: compute / execution cost at time `t`
- `S_t`: slippage at time `t`
- `Q_t`: quote output at time `t`
- `B_t`: breakeven margin at time `t`
- `K_t`: risk score at time `t`

Source-of-truth rules:
- liquidity, fee, and price inputs must be tied to an authoritative source set
- slot state must be tied to a current slot reference
- stale inputs are not allowed to silently become accept decisions
- conflicting sources require explicit precedence or defer/reject

Freshness rules:
- a quote is only valid inside a freshness window
- freshness is measured by slot age or timestamp age, not by intuition
- stale quotes defer or reject before route selection becomes terminal

Solana market reality:
- the live problem is one pre-trade swap request against current pool state, current fee state, and current slot state
- the important variables are pool depth, reserve balance, route count, fee rate, compute-unit price, priority fee, slot age, and expected execution price
- a route is only useful if the marginal output exceeds fees, slippage, compute cost, and risk penalty at the current slot
- thin liquidity or account contention can make the best quoted route economically invalid even if the raw quote is positive

## Computation classes

- Quote math: AMM quote, route quote, expected output -> `Q_t`, `y`
- Slippage math: execution impact vs quote -> `S_t`
- Route math: compare candidate routes -> best route
- Fee math: priority fee, CU cost, tx cost -> `F_t`
- Breakeven math: expected value after costs -> `B_t`
- Risk math: stale liquidity, concentration, hot account risk -> `K_t`
- Decision math: forward / defer / reject -> terminal decision

These computations are the economic transitions of the state machine. The service is only useful if the computed route, slippage, fee, and breakeven all fit the freshness window and the source-of-truth contract.
If the market snapshot is stale, the computation is only informational, not actionable.

Route-level economics:
- for each candidate route `i`, compute gross output, fee cost, slippage cost, compute cost, and risk penalty
- `net_value_i = gross_output_i - input_value - fee_cost_i - slippage_cost_i - compute_cost_i - risk_penalty_i`
- choose the route with the highest positive `net_value_i`
- if every candidate route has `net_value_i <= 0`, the request is not economically viable and must defer or reject
- if route uncertainty is high, the output is informational until fresh state arrives

Observable outputs:
- `trace_id`
- `request_id`
- `decision_id`
- `model_version`
- `route_id`
- `slot`
- `quote_age`
- `source_hashes`
- structured decision record
- append-only audit event

The decision layer is reconstructible only if the quote, route, model version, source hashes, and terminal state are all persisted together.

## Core formulas

Expected value:

`expected_value = y * p_out - x * p_in`

Slippage:

`S_t = (quote_out - execution_out) / max(quote_out, 1)`

Breakeven margin:

`B_t = expected_value - (F_t + C_t + slippage_cost)`

Route score:

`route_score_i = expected_out_i - fee_cost_i - slippage_cost_i - risk_penalty_i`

Quote delta:

`delta = best_route_out - alternative_route_out`

Best route:

`route* = argmax_i(route_score_i)`

Decision rule:

`accept if B_t > 0 and K_t <= risk_limit and slot_age <= freshness_limit`

`defer if B_t is near 0 or data is stale`

`reject if B_t < 0 or risk_limit is violated`

The economic decision is not allowed to be implied. It must be explicit, replayable, and tied to the exact source state used to compute it.

Operational controls:
- accept is allowed only when positive EV, risk limit, freshness limit, and source integrity all pass
- defer is required when EV is marginal, freshness is weak, or route certainty is low
- reject is required when EV is negative, source hashes are missing, or risk exceeds limit
- stale data is a hard gate, not a soft warning
- duplicate requests use the recorded terminal decision, not a fresh compute path
- a route score is valid only when the route count is within cap and the source set is complete
- if the model version or source set changes, the prior decision is not reused blindly
- if the compute path cannot prove freshness, the output is informational only
- if the service cannot prove a positive lower bound on EV, it must not accept
- if execution, persistence, or audit controls fail, the system must stop accepting and fail closed

Economic viability checks:
- `EV_lower_bound > 0` is required for accept
- `EV_expected` must exceed the minimum viable EV for the request class
- `EV_expected` must exceed operating cost per request plus safety margin
- `fee_cost / expected_output_value` must remain below the route-class fee ceiling
- `slippage_cost / input_value` must remain below the route-class slippage ceiling
- `compute_cost / expected_value` must remain below the compute ceiling
- `risk_penalty` must remain below the configured risk budget
- the route set must contain at least one viable route or the request must defer
- source freshness must be inside the accepted window or the request must defer/reject
- a positive point estimate is not enough; the lower bound must stay positive after penalties

System viability checks:
- realized EV per accepted request must stay positive on average
- positive requests per day must exceed operating cost per day
- accepted-request false-positive rate must stay below the configured ceiling
- rejected-request false-negative rate must stay below the configured tolerance
- audit coverage must stay at 100 percent for terminal requests
- duplicate suppression must stay near 100 percent for repeated request IDs
- stale-source rejection rate must remain visible and within tolerance
- model calibration error must remain below the configured threshold
- p99 latency, queue depth, and retry amplification must stay inside control bands

## Inputs

- Swap intent: token pair, amount, side, request ID
- Route candidates: AMM / aggregator / venue choices
- Liquidity state: pools, depth, reserves, route data
- Fee state: CU estimate, priority fee, market state
- Request identity: request ID, dedupe key, schema version
- Freshness state: slot / timestamp / quote age

Input quality rules:
- every input has a declared unit
- every input has a declared freshness state when external
- missing source state is a defer/reject condition
- input schema version is mandatory

## Outputs

- Expected output amount: best estimated fill in base units
- Slippage estimate: expected execution penalty, typically bps
- Route recommendation: best route by score
- Fee estimate: cost to execute in lamports or quote currency
- Breakeven margin: whether swap is economically worth it
- Decision: accept, defer, reject
- Audit record: durable result with inputs and version
- Replay record: cached terminal outcome for duplicates

Output quality rules:
- a terminal decision is stored durably before acknowledgment
- a duplicate request returns the recorded output, not a fresh recomputation
- every output must be attributable to a request ID, model version, and source hash set
- every output must be explainable from the input snapshot and route scoring path
- every output must state whether it is actionable or informational

Operational health checks:
- queue depth below `Q_max`
- worker utilization below sustained limit
- dedupe hit rate within expected retry band
- replay drift equals zero for identical state
- audit coverage equals 100 percent for terminal requests
- freshness failure rate below tolerance
- source conflict rate below tolerance
- model-version mismatch rate equals zero
- transport error rate below the retry budget
- p50, p95, and p99 decision latency below configured deadlines
- stale decision rate below the reject threshold
- non-actionable output rate visible and bounded

## Rust responsibilities

Rust computes the bounded heavy math:
- route scoring
- quote estimation
- slippage estimation
- fee math
- breakeven math
- stale-state and risk scoring
- batch comparison over routes or pools
- canonicalization of numeric inputs

Rust also owns the bounded compute side of freshness-sensitive math:
- if the source state is stale, Rust should return a stale result marker, not a guessed accept decision
- if the inputs conflict, Rust should return a conflict marker
- if the lower bound on EV is not positive, Rust should return a non-actionable marker

Rust should answer:
- “What is the best route?”
- “What is the quote?”
- “What is the slippage?”
- “What is the breakeven?”
- “What is the risk-adjusted decision input?”

Operational control loop:
- compute EV lower bound
- check freshness and source integrity
- apply route cap and risk cap
- if all gates pass, return actionable accept
- otherwise return defer or reject with reason code

System-level economic loop:
- estimate request-level EV
- accumulate realized EV by token pair, route, and venue
- compare realized EV against infrastructure and audit cost
- reject or throttle cohorts that drift below breakeven
- increase acceptance only when calibration and freshness remain inside tolerance
- treat sustained negative cohort EV as a routing or model defect, not noise

## Scala responsibilities

Scala owns:
- ingress
- normalization
- dedupe
- queue control
- routing
- retries
- persistence
- audit
- request lifecycle

Scala also owns:
- trace propagation
- decision durability
- retry ceilings
- queue control
- duplicate suppression
- observability and audit emission

Scala should answer:
- “Should this run?”
- “Should this be retried?”
- “Should this be deferred?”
- “Should this be recorded?”

## How it fits the wider design

- Scala orchestrator: receives request, normalizes, dedupes, routes
- State machine: each swap request ends in one terminal state
- Bounded queue: burst traffic is deferred, not absorbed unboundedly
- gRPC boundary: Rust executes bounded numeric compute
- Idempotency: repeated swap request returns same terminal outcome
- Audit log: every quote and decision is recorded
- Recovery: replay returns recorded result, not recomputation

Operational fit:
- observability is 100 percent coverage of terminal swap requests
- auditability is append-only and request-addressable
- idempotency is keyed by request ID plus dedupe key
- concurrency is bounded by key, queue, and worker pool
- freshness is slot-aware
- security depends on authenticated ingress and source validation

## Quantitative budget view

- Normalize: `O(n)` in request size
- Dedupe: `O(1)` average
- Route scoring: `O(r)` where `r <= R_max_routes`
- Rust compute: bounded by workload class
- Persistence: one terminal write per request
- Audit: one append per request
- Request envelope bytes: `B_req`
- Quote bytes: `B_quote`
- Route-candidate bytes: `B_routes`
- Decision bytes: `B_decision`
- Audit bytes: `B_audit`

The live byte budget matters because every in-flight request is a memory multiplier. The system is only bounded if request size, quote size, route count, decision size, and audit size are all capped.

`B_inflight ≈ N_inflight × (B_req + B_quote + B_routes + B_decision + B_audit)`

`E[N_inflight] = λ × E[W]`

`Throughput_max ≈ min(μ_normalize, μ_dedupe, μ_route, μ_rust, μ_persist, μ_audit)`

## In-flight request model

Let `N_s(t)` be the number of swap requests in non-terminal processing states at time `t`.

`N_inflight(t) = N_received + N_normalized + N_deduped + N_classified + N_queued + N_dispatched + N_computing + N_persisted + N_audited`

Little's Law:

`E[N_inflight] = λ × E[W]`

where:
- `λ` = swap request arrival rate
- `E[W]` = end-to-end waiting + service time

Route selection math:
- compare all candidate routes
- compute `route_score_i` for each route
- choose `route* = argmax_i(route_score_i)`
- forward only if `route*_score` clears the breakeven threshold and freshness limit

Decision thresholds:
- `accept` if `B_t > 0`, `K_t <= risk_limit`, and freshness is valid
- `defer` if data is stale or breakeven is marginal
- `reject` if breakeven is negative or risk is too high

Decision visibility:
- every accept, defer, or reject carries a reason code
- every reason code is stored with the model version and source hashes
- every stale-data decision is distinguishable from a risk decision
- every duplicate decision is distinguishable from a fresh decision
- every estimate must carry the data age that produced it
- every live decision must be reconstructible from the request ID, source hashes, and recorded route score

This is the economic contract in live operation:
- accept means positive net value under fresh state
- defer means the decision is not fresh enough or not strong enough to execute now
- reject means the economics or risk fail the threshold
- the same request ID cannot oscillate between outcomes unless the source state changes, and that change is part of the recorded trace

## Failure modes

| Failure | Response |
|---|---|
| Missing liquidity state | reject or defer |
| Invalid schema | reject |
| Duplicate request | cached terminal outcome |
| Quote stale | defer |
| Queue full | defer |
| Compute timeout | failed |
| Persistence failure | failed |
| Audit failure | failed |

Failure handling rules:
- stale quotes are not accepted
- missing source hashes are not accepted
- duplicate requests do not trigger new compute
- a persistence or audit failure never produces a silent success
- a queue full condition is a deferred request, not a dropped request
- a stale quote is never upgraded to accept without a fresh recomputation
- a decision without recorded freshness is not valid operationally

## What this is for

- swap quote and route selection
- slippage and price impact estimation
- breakeven calculation
- fee / execution cost estimation
- risk scoring before forwarding a swap
- duplicate suppression for repeated swap requests

This is the economic front end for Solana swap traffic. It exists to prevent negative-EV execution, stale routing, duplicate spend, and fee waste.

This is the off-chain swap decision and compute layer that protects users from bad execution quality, duplicate spend, stale data, and non-positive economics.
Operationally, its truth is measured by whether it prevents negative-EV or stale swaps from reaching execution.

What Solana economics means here:
- not token supply math in the abstract
- not governance math in the abstract
- not settlement math in the abstract
- the live problem is whether a pre-trade swap request is worth forwarding against current pool state, fee state, and slot state
- the relevant economic unit is net value after fees, slippage, compute, and risk at the current slot
- the service is healthy only if it filters out negative-EV requests faster than the chain would consume them

## What it is not

- not the swap transaction itself
- not on-chain execution
- not consensus
- not validator logic
- not a quote oracle with no audit trail
