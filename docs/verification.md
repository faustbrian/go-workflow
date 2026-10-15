# Verification strategy

[Documentation map](index.md) · [Repository README](../README.md)
[Next: API and lifecycle reference](reference.md)

Executable evidence is scoped to the workflow module and its PostgreSQL
package. Passing adjacent repository packages does not prove workflow behavior.

## Deterministic model

Tests construct all times, identities, definitions, events, and retry inputs
explicitly. Replay tests compare durable snapshots and reject gaps, duplicates,
stale sequences, invalid lifecycle changes, version mismatches, and malformed
event-specific fields. Fuzz targets exercise history, transitions, work leases,
activities, compensation, child dispatch, waits, operators, and worker
decisions at hostile boundaries. A mixed-version rolling-deploy scenario keeps
version-one history readable by both old and new registries, rejects version-two
history in an old worker, rejects an incompatible same-version fingerprint, and
uses only persisted migration and continue-as-new decisions.

## Failure matrix

| Boundary | Required invariant |
| --- | --- |
| Transition write before commit | rollback exposes neither history nor work |
| Caller transaction plus outbox | rollback or commit affects both records |
| Commit response lost | exact reconciliation decides retry safety |
| Attempt start before activity | an OS process exit after the side effect leaves the start durable; redelivery records unknown without redispatch |
| Activity result persistence | unknown result remains unknown |
| Timer firing before completion | crash leaves recoverable durable work |
| Signal persistence before acknowledgement | duplicate source identity is an exact replay |
| Lease renewal and finalization | stale fencing token is rejected |
| Compensation attempt and outcome | failure never appears as successful rollback |
| Child start | uncertain creation requires reconciliation |
| Dead-letter resolution | retry or discard is audited and token-fenced |

PostgreSQL integration tests exercise atomic transition/history/work writes,
optimistic conflicts, stable pagination, archive membership, fencing, dead
letters, migration order, rollback, process death, deadlocks, restart and
snapshot restore, streaming-replica promotion, and caller-owned transaction
composition. The activity process-kill drill commits a marker side effect in a
child process, exits before outcome persistence, then proves a higher-fenced
redelivery records an unknown result without invoking the handler again. The
interoperability target builds a temporary candidate consumer and proves workflow
transitions and optional outbox envelopes share one PostgreSQL commit while
inbound signal redelivery remains exactly deduplicated. It also partitions and
recovers a live Kafka broker, preserves retryable or ambiguous publication
classification, and proves stable duplicate identity and keyed order after
recovery. Caller acknowledgement still follows the confirmed workflow commit;
the package does not own it. The optional CloudEvents adapter separately proves
loss-aware workflow-history round trips, and queue settlement suites prove that
successful handlers acknowledge only after return while poison and exhausted
deliveries reach durable dead-letter handling before source acknowledgement.
Those transport-owned checks complement, rather than weaken or duplicate, the
core workflow persistence boundary.

Before publication, `make -f verification/package.mk interoperability` runs
`scripts/check-interoperability.sh candidate`: its disposable non-releasable
module replaces only Workflow with the exact owning source. The script defaults
to `public`; explicit `public` has the same strict no-replacement behavior and
uses the published v3.0.0 after release. That actual public consumer is a separate
required publication check, not satisfied by candidate composition. Both full
modes retain the PostgreSQL and Kafka runtime tests. The optional
`candidate --compile-only` verifies fixture compilation without starting those
services; it is not runtime composition or public-consumer evidence.

Operational Assurance additionally selects `selected-public`: Workflow
`/v2@v2.0.0`, Outbox `/v2@v2.0.0`, its Kafka adapter `/v2@v2.0.0`, and Kafka
`v1.1.0`. It instantiates the same assertions with those canonical imports,
uses the checked-in consumer module and checksums, and rejects any replacement
or MVS change to the four selected releases. It does not upgrade the frozen
compatibility catalog or replace the existing v3 candidate/public recipe.

Run `bash scripts/check-interoperability.sh selected-public --compile-only`
for an authenticated public compile without services. The optional hosted
`selected_public_composition` input runs the full public transaction and
live-broker recovery tests using digest-pinned PostgreSQL 18.4 and Confluent
Kafka 7.5.0; a requested failure blocks Required. PostgreSQL
rollback and commit remain caller-owned, broker timeout remains retryable or
unknown, and repeated publication retains stable identity without claiming
exactly-once external effects. Existing compensation, dead-letter and operator
recovery evidence is separate and is not repeated by this selection.

## Ordinary and full routes

`GOWORK=off go test ./...` runs untagged tests and deterministic fuzz seeds.
`make check` and `make ci` use the checksum-verified public verifier's bounded
local route, with one named worker shutdown/join and race-winner run under the
race detector. The Make routes retain the catalog's `integration` tag, so the
PostgreSQL tests use `WORKFLOW_POSTGRES_URL` or Docker/Testcontainers. Base-free
local checks do not run active fuzz; select a native target with `-fuzztime=2s`
for development, or use changed-source CI's selected smoke. Ordinary scheduled
CI is also bounded. `make ci` validates specification structure offline;
`make check-full` explicitly checks authoritative sources online and retains
the original full operations and budgets.

## Release gates

The scoped release contract includes formatting, tidy, safety, vet, tests,
race, exact statement coverage, lint, static analysis, vulnerability, secret,
license, SBOM, fuzz, exact viable mutation kills, documentation, API,
conformance, interoperability, benchmarks, and clean-consumer build proof. Go
commands use a disposable task-owned build cache.

`make -f verification/package.mk soak` runs a bounded accelerated continue-as-new
replay and worker-churn audit. Its default 72 batches advance the deterministic workflow clock by one
hour each, exercising three days of logical execution without waiting three
wall-clock days. Each batch processes 128 deterministic replays and 64 leased
work items under bounded concurrency; periodic checkpoints enforce retained-
heap and goroutine ceilings. The target uses disposable Go build and module
caches against an immutable archive of the clean committed workflow tree and
reports its revision and SHA-256 input digest. `WORKFLOW_SOAK_BATCHES` scales
the workload from 1 through 720 batches, while `WORKFLOW_SOAK_LIVE_TREE=1` runs
an uncommitted tree during development. This accelerated audit proves
deterministic state progression and bounded churn for its stated workload; it
does not claim production uptime or replace deployment monitoring.
