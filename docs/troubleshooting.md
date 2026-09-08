# Troubleshooting and FAQ

## A commit returned an error. May I retry?

Inspect `StoreCommitError` with `errors.As`. Retry the same immutable transition
only when the outcome is `StoreCommitNotCommitted`. For `StoreCommitUnknown`,
call `ReconcileTransition` with the transition identity and fingerprint first.
Blindly retrying an unknown outcome can duplicate an external action.

## A worker stopped processing after cancellation

That is expected. `Worker.Run` stops admission, cancels active processors, and
waits for them to return. Verify that every processor and external handler
honors its context and stops its own goroutines. A handler that ignores
cancellation can delay `Run` indefinitely; the package cannot forcibly stop
arbitrary caller code.

## A stale worker cannot complete a lease

Lease expiry or renewal loss transfers ownership with a higher fencing token.
Treat `ErrStaleWorkLease` as final loss of that lease. Do not retry completion
with the old token; reload or allow the current owner to proceed.

## Does Workflow provide exactly-once activity execution?

No. The package persists the attempt identity before invoking a handler and
records known or unknown outcomes afterward. Handlers must be idempotent, and
unknown outcomes must be reconciled before redispatch.

## Does compensation roll back an external transaction?

No. Compensation is another external action with its own identity, deadline,
retry policy, and possible unknown outcome. A failed or manually resolved
compensation is never represented as a successful rollback.

## Why does registry compilation reject a deployed definition?

Every running instance is pinned to a definition name, version, and
fingerprint. Keep all referenced versions registered during rolling deploys.
Publish changed behavior under a new version and use an explicit durable
migration or continue-as-new transition.

## Who owns schemas, pools, and workers?

The application creates the PostgreSQL schema, applies migrations, owns the
pool, starts one `Worker.Run` lifecycle, cancels and joins it, and then closes
external resources. The workflow module neither migrates automatically nor
supervises the process.

For reproducible defects use [GitHub Issues](https://github.com/faustbrian/go-workflow/issues).
For adoption questions use
[GitHub Discussions](https://github.com/faustbrian/go-workflow/discussions).
Report vulnerabilities through the private process in
[SECURITY.md](../SECURITY.md).
