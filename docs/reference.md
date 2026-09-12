# API and lifecycle reference

## Construction, defaults, and validation

`NewDefinition` validates names, versions, step shape, limits, deadlines, retry
policy, branch references, and child definition references. It copies the step
slice, branch slices, and compensation specs. `CompileDefinitions` then checks
duplicate name/version keys and child references and returns an immutable
registry. A definition version is never replaced in place; running instances
stay pinned by name, version, and fingerprint.

Value constructors reject invalid zero values with their documented sentinel
errors. There is no process-global registry, service locator, environment
loading, or implicit I/O. Callers supply timestamps, identities, limits,
interfaces, and policies. `postgres.Config{}` selects the `workflow` schema;
an explicit schema must satisfy the package's lowercase identifier policy.

## Persistence and ownership

`Transition` owns a contiguous history batch and its associated due-work
records. A `TransitionStore` must commit all of them atomically or none of them.
Returned slices and byte payloads are copies, so callers do not acquire mutable
aliases to a constructed value.

`postgres.New` borrows the supplied `*pgxpool.Pool` for the lifetime of the
store. The caller owns opening and closing the pool, creating the schema,
applying `SchemaMigrationsFor` in order, and authorizing any rollback. The store
is safe for concurrent operations to the extent supported by the supplied
pool. It starts no goroutines.

`postgres.Store.Stage` writes through a caller-owned `pgx.Tx`; it does not
commit, roll back, or retain that transaction. A nil result means the records
are staged or were an exact idempotent replay, not that they are durably
committed. Publication and source acknowledgement must wait for the caller's
commit to be confirmed.

Workers and activity, compensation, and child processors retain their
configured store, processor, clock, hook, registry, and callback interfaces.
The caller must keep those collaborators valid and safe for every concurrent
call until the owning `Run` or `Process` operation has returned.

## Cancellation, concurrency, and shutdown

Blocking public operations take `context.Context` and do not retain it after
the call. Cancellation requests exit, but the bound depends on every
caller-supplied processor and handler honoring the context and finishing its
cleanup. Cancellation cannot undo an external side effect that already
occurred. A commit transport error may still have committed and therefore
remains an unknown outcome until reconciliation.

Definitions, registries, history values, transitions, requests, outcomes, and
the configured PostgreSQL store are immutable after construction and may be
read concurrently. A `Worker` owns at most `MaxConcurrent` handlers and their
lease-renewal goroutines during one `Run` call. Use one active `Run` call per
worker. Canceling its context stops new claims, cancels active processors, and
waits for every processor-owned goroutine to exit before `Run` returns.

`WorkerHooks.OnWorkerEvent` is synchronous and may be called concurrently by
bounded handlers. Implementations own synchronization, must return promptly,
must not panic, and must avoid secrets or high-cardinality identities in metric
labels. Activity, compensation, and child-start callbacks receive the operation
context, may be concurrent, and must obey their deadline and idempotency
contract.

There is no `Close` or `Shutdown` method: constructed values acquire no
independently owned external resource. Cancel and join `Worker.Run`; close the
caller-owned pool and other integrations separately after workers return.

## Errors and outcomes

Use `errors.Is` with exported sentinels for validation, lookup, conflict,
fencing, and traversal-limit categories. Use `errors.As` for
`*StoreCommitError`, then inspect `CommitOutcome`:

- `StoreCommitNotCommitted` permits retrying the same immutable transition
  intent.
- `StoreCommitCommitted` means the exact transition is durable.
- `StoreCommitUnknown` requires `ReconcileTransition` before retrying.

PostgreSQL operations preserve safe underlying causes through wrapping.
`postgres.ErrCorruptStore` reports durable data that cannot satisfy the public
model. Error strings are diagnostic only and must not be parsed. Payloads,
credentials, tenant data, and external handler details must be redacted before
logging returned errors or hook events.

## Integrations and limitations

The module can compose with a caller transaction, transactional outbox, Kafka
or queue publication, and CloudEvents mapping, but it does not import or
initialize those systems. It does not provide a broker, scheduler daemon,
process supervisor, authorization policy, tenant isolation, encryption, or
exactly-once external effects.

There is no dedicated test-helper package. Tests use the public value
constructors and inject `Clock`, store, processor, callback, and sink interfaces.
Use a deterministic clock implementation and explicit timestamps and IDs;
tests must not mutate process-global state.

The source has no OS-specific build tags. Release CI verifies Go 1.27.0 on
Ubuntu 24.04 amd64; other Go-supported targets and PostgreSQL deployments are
not claimed as release-qualified environments. Capacity, schema, restore,
rolling-deploy, and observability guidance is in [Operations](operations.md).
Executable failure and benchmark evidence is in
[Verification](verification.md).
