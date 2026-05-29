## v0 Reality

### What It Actually Does

v0 is an in-process Scala ledger demo. The core object is `Calculator`, which takes an immutable `Map[String, Double]` and returns a new map after an add or subtract operation.

`addAmount`:
- inserts a new address if it does not exist
- increments an existing balance if it does
- always returns a new immutable map

`subtractAmount`:
- looks up an address
- returns `Left(TransactionError)` if the address is missing
- returns `Left(TransactionError)` if the subtraction would make the balance negative
- returns `Right(updatedAccounts)` otherwise

The model is local and in-memory. There is no request envelope, no request identity, no durable state, no replay log, no transport boundary, and no persistence layer.

### Implementation Shape

The implementation uses Scala's immutable collections and basic pattern matching. The code path is simple:
- read a map
- compute a new map
- return the new map or an error

There is no cross-process boundary and no offload path. The data model is `Map[String, Double]`, so the "state" is just a local value passed through functions.

The error model is also local:
- `TransactionError` is a case class exception
- subtraction failures are reported through `Either`
- add operations do not fail

The code does not model transaction lifecycle, audit, or delivery guarantees.

### Concurrency Reality

`MultithreadedCompute` uses `Future` on the global execution context to fan out random add and subtract operations. That is concurrency at the call site, not a bounded request system.

The concurrency shape is:
- random address generation
- random amount generation
- random choice between add and subtract
- repeated work across multiple Futures

The concurrency test does not establish safe shared-state behavior. The test code in `IntegrationTest` mutates a shared `var accounts` inside concurrent Futures. That is race-prone and non-deterministic. The code comments even acknowledge the need for thread-safety if the state were mutable.

So the v0 concurrency story is not a state machine, not actor partitioning, and not backpressure. It is concurrent access to a shared in-memory value with no durable correctness contract.

### Testing Reality

The unit tests validate local function behavior:
- add creates or increments balances
- subtract returns non-negative balances when possible
- subtract fails on missing accounts
- subtract fails on insufficient funds

Those tests prove the functions return the expected local values. They do not prove:
- concurrency safety
- determinism under load
- durable replay
- idempotency
- auditability
- transport behavior
- system recovery

The integration test is a load-shaped loop over random operations. It does not define a production throughput target, a recovery target, or a correctness target under duplicate delivery.

### Production Claims vs Reality

The README describes multithreaded compute and production-like load simulation, but the code does not implement a production service. There is no API boundary, no queue, no worker pool contract, no persistence, no durable log, no measured latency, and no measured throughput envelope.

What exists is a local Scala demo that:
- computes on a map
- uses Futures for fan-out
- prints results
- asserts that balances remain non-negative

That is useful as a toy ledger and a concurrency sketch. It is not a bounded request service.

### Gaps To v1

v1 has to add what v0 does not have:
- request identity
- dedupe key
- terminal state
- durable decision record
- replay lookup
- transport boundary
- Rust compute boundary
- bounded queue
- bounded worker pool
- per-key serialization
- observability
- auditability
- recovery semantics
- economic decisioning

v0 also has to stop claiming things it does not show:
- no production capacity proof
- no measured load behavior
- no replay safety
- no idempotency contract
- no concurrency safety contract

### Short Version

v0 is a local immutable-map calculator with concurrent Futures layered on top. It is not a transaction platform, not a request processor, and not a production boundary.
