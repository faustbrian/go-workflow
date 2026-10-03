# V1 threat model and residual ownership

## Source and release scope

Model revision 1, dated 2026-10-03, covers the root `workflow` package and its
`postgres` adapter. The source baselines are:

- Published `v1.0.0`: `aef739f62aa389008eebcbedb74492d2e3067b9e`.
- Assessed main: `7066c265edb25a30984de304859110a641812051`.

Production Go source is unchanged between these baselines. Main adds executable
compensation examples and integration coverage, changes test and tooling files,
and requires Go 1.27.0; the published baseline declares Go 1.26.6. The additional
examples and current-main verification are not evidence of a new public release.
Later source or dependency changes require reassessing the affected boundaries.

The unreleased constructor-admission correction validates borrowed temporary
views in `NewActivityRequest`, `NewActivityOutcome`, `NewChildStartRequest`,
`NewPendingWork` and `NewTransition` before making their defensive byte/slice
copies. The published and original assessed baselines copied those inputs
before validation. Existing limits, error classifications and accepted-value
ownership remain unchanged; this correction is not yet a published guarantee.

This model records source contracts, not a completed security audit, scanner
clearance, verified deployment, or approval to accept an application's risk.
Existing tests below are contract oracles; this documentation change does not
claim they were executed. The supported release policy remains in
[SECURITY.md](../SECURITY.md).

## Assets, entry points, and trust boundaries

Protected assets are durable history and workflow state, transition identities,
work ownership and fencing tokens, operator audit records, and the integrity and
confidentiality of application payloads.

Applications supply definitions, activity and compensation handlers, child-start
callbacks, processors, clocks, stores, hooks, and authorized operator identities.
These collaborators are trusted code, not a sandboxed extension boundary.
External message data, signals, results, and exported history may contain
untrusted or sensitive application data even when their library value is valid.
Validation does not authenticate the sender or authorize an operation.

The PostgreSQL adapter borrows the application's pool and transaction. The
application controls credentials, TLS, schema migration, tenant access, backups,
and external publication. A custom store must implement the same atomicity,
sequence, idempotency, and fencing contracts as the PostgreSQL adapter.

## Source controls and ordinary contract oracles

- **Admission and immutable values:** constructors reject invalid zero values
  and own defensive copies. Definitions are versioned and fingerprinted;
  payloads are bounded by `MaxPayloadBytes` (16 MiB), fan-out by `MaxFanOut`
  (10,000), and history pages by `MaxHistoryPageEvents` (1,000). Worker creation
  requires explicit valid collaborators, concurrency, claim and timing limits.
  See [definition.go](../definition.go), [store.go](../store.go), and
  [worker.go](../worker.go). Ordinary oracles include
  `TestDefinitionOwnsImmutableVersionedBehavior` and
  `TestNewWorkerRejectsUnboundedOrMissingConfiguration`.
- **Replay and commit ownership:** history is replayed against pinned definition
  identity. A transition owns its contiguous history and due-work batch;
  stores must commit the batch atomically. Staging in a caller transaction is
  not commit confirmation. `StoreCommitUnknown` requires reconciliation before
  retrying external progression. See [history.go](../history.go),
  [store.go](../store.go), and [the lifecycle reference](reference.md).
  `TestReplayRejectsUnpinnedOrConflictingHistory` is an ordinary replay oracle.
- **Retry, cancellation, and leases:** retries preserve semantic attempt
  identities; uncertain external outcomes must not be guessed as success or
  failure. Workers stop admission on cancellation and join active processors;
  stale ownership fences prevent finalization by a displaced worker. This
  depends on collaborators honoring cancellation. See [worker.go](../worker.go)
  and [recovery guidance](operations.md). Ordinary oracles include
  `TestActivityOutcomeAndRetryPreserveSemanticAttemptIdentity`,
  `TestCompensationTransitionsPersistIndependentRetryLifecycle`, and
  `TestWorkerHandleCancellationPreservesOnlyKnownDecisions`.
- **Observability:** hooks are synchronous, may run concurrently, and expose
  underlying causes to caller-owned logs and traces. Identity fields are not
  bounded-cardinality metric labels, and payload/error data is not automatically
  redacted. See [worker.go](../worker.go) and
  [observability guidance](operations.md#observability).

## Residual ownership and review conditions

These are documented trust assumptions and limitations, not blanket risk
acceptances. The owners below must apply the mitigations appropriate to their
deployment; library maintainers retain responsibility for defects in the stated
library contracts.

### Application authorization and data confidentiality

- **Owner:** application identity/security owner and database operator.
- **Rationale:** stable actor and tenant identifiers are data, not proof of
  authorization. The library does not provide tenant isolation, encryption,
  credential management, or a handler sandbox.
- **Mitigation:** authenticate and authorize every inbound and operator action;
  restrict database access, configure transport/storage protection, and keep
  secrets out of workflow data where possible.
- **Review condition:** new tenant boundaries, operator routes, payload classes,
  database roles, or exported-history recipients.

### Aggregate resource use and collaborator termination

- **Owner:** application runtime owner; library maintainers own admission bounds.
- **Rationale:** per-value limits do not establish a process-wide memory, queue,
  history-retention, or database-capacity budget. Context cancellation cannot
  forcibly terminate trusted code; hooks must return promptly and not panic.
- **Mitigation:** use workload-appropriate limits and retention, bound intake and
  callback concurrency, propagate deadlines, and join workers before closing
  their collaborators. Do not share one worker across concurrent `Run` calls.
- **Review condition:** changed limit defaults, workload size, handler/hook code,
  shutdown policy, or dependency cancellation behavior.

### Replay, retries, and external-effect uncertainty

- **Owner:** application workflow/business owner and external-effect provider;
  library maintainers own replay and transition validation.
- **Rationale:** durable workflow state does not make remote effects exactly
  once. Cancellation or a lost response may follow an already-applied effect.
- **Mitigation:** retain immutable definition versions, use stable effect and
  transition identities, reconcile unknown outcomes, and authorize explicit
  compensation or manual resolution rather than recording guessed success.
- **Review condition:** definition migration, changed retry/compensation policy,
  new external-effect provider, or changed idempotency/reconciliation semantics.

### Store atomicity, fencing, and recovery

- **Owner:** store implementer and database operator; library maintainers own the
  supplied PostgreSQL adapter's implementation.
- **Rationale:** a custom store or inconsistent restore can violate atomicity and
  ownership even when in-memory values pass validation. Staged records are not
  necessarily committed, and a commit transport error can leave an unknown state.
- **Mitigation:** implement the documented atomic batch, sequence and fence
  checks; reconcile unknown commits before progression; restore related tables
  consistently and reconcile publication/acknowledgement projections.
- **Review condition:** store replacement, schema or isolation changes, restore
  procedures, failover assumptions, or publication/acknowledgement ordering.

### Error, hook, and history disclosure

- **Owner:** application observability owner and history-export operator.
- **Rationale:** underlying driver/handler causes, signals, hook identities and
  history exports may reveal secrets or tenant data. The library does not supply
  an automatic redaction policy.
- **Mitigation:** redact before emission, authorize exports, restrict retention
  and recipients, and use bounded-cardinality metric dimensions.
- **Review condition:** new sinks, hook consumers, diagnostic fields, export
  formats, or sensitive data categories.

Suspected violations of library-owned invariants use the
[private vulnerability reporting route](../SECURITY.md), not public support
issues. This model alone does not close any release or ecosystem security gate.
