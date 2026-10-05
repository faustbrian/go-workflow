# V2 source model and residual ownership

Model revision 2, dated 2026-10-03, covers the root Workflow and PostgreSQL
packages on main derived from `e9c20463ee4b44a1b37c85310c6890334af3313d`.
The nominal v2 migration targets Go 1.27.0 and version 2.0.0. This source model
is not a publication or release-qualification verdict. The [historical v1 model](threat-model-v1.md)
remains unchanged and identifies published v1.0.0 separately.

## Corrected library-owned admission

`NewActivityRequest`, `NewActivityOutcome`, `NewChildStartRequest`,
`NewPendingWork` and `NewTransition` validate borrowed temporary views before
copying input bytes, event slices and nested work. Existing scalar, count,
aggregate and payload limits, errors, fingerprints and immutable accepted
results remain unchanged. `TestConstructorsRejectBeforeDefensiveCopies` retains
separate byte-bound and tiny early-rejection allocation assertions;
`TestActivityRequestOwnsBoundedAttemptMetadata`,
`TestChildStartValuesRejectInvalidAndOwnInput` and
`TestTransitionOwnsAtomicHistoryAndDueWork` retain ownership controls. These are
source-level contract oracles, not universal heap or throughput guarantees.

Published v1.0.0 copied before validation and does not contain this correction.
The nominal migration adds no runtime algorithms or persistence schema change.
Release CI, qualified scanner adoption and clean public consumers require
separate attributable evidence;
this model is not blanket security approval, an advisory or a severity assessment.

## Narrow native integer-conversion suppressions

Seven exact-site `#nosec G115` annotations cover eight native conversion
findings without changing runtime arithmetic, admission, schemas or scanner
thresholds. Maintainers own these range proofs, not a blanket scanner exemption:

- History traversal validates page sizes at most 1000; its minimum cannot
  exceed that uint32 page size. Validated returned pages also preserve the
  remaining event allowance.
- Immutable definitions validate positive `RetryPolicy` durations with
  `MaxDelay >= InitialDelay`. Each retry iteration retains `delay <= MaxDelay`
  and both durations are at most MaxInt64 nanoseconds. Unsigned doubling is at
  most MaxUint64 minus one; the final MaxDelay cap keeps the signed result at
  most MaxInt64. The existing 63-iteration bound remains unchanged.
- PostgreSQL entry points revalidate request limits: instance and dead-letter
  pages are at most 100 (lookahead at most 101), history pages at most 1000
  (lookahead at most 1001), and work claims at most 100. All fit int32.
- The owned PostgreSQL schema constrains the scanned instance sequence to a
  nonnegative BIGINT, which fits uint64. This proof trusts the owned database
  constraint; it does not classify corrupt negative rows or a modified schema
  as valid. Database operators retain schema-integrity ownership.

Review each annotation when admission limits, retry validation or saturation,
the iteration cap, query validation paths, or database constraints change.
Exact rule IDs and concrete justifications remain required; global exclusions
and broad linter disables are not substitutes. Changed native scanner and CI
results require separate evidence; these proofs are not vulnerability severity
or release-qualification claims.

## Retained trust boundaries and residual owners

The assets, entry points, replay/commit/fencing controls and detailed
owner/rationale/mitigation/review conditions in the
[v1 residual ownership sections](threat-model-v1.md#residual-ownership-and-review-conditions)
continue to apply to v2:

- Application identity and database operators own authorization, tenant access,
  encryption and credential handling; valid identifiers do not authenticate.
- Runtime owners bound aggregate intake, retention and callback concurrency;
  maintainers own stated admission limits. Per-value bounds are not process-wide
  memory or capacity budgets, and cancellation cannot terminate trusted code.
- Business owners and external providers own effect idempotency and uncertainty;
  maintainers own replay/transition validation. Durable state is not exactly-once
  external execution, and unknown outcomes require reconciliation.
- Store implementers/database operators own atomicity and recovery procedures;
  maintainers own the PostgreSQL adapter. Staging is not commit confirmation.
- Observability and export owners redact underlying causes, hooks and history
  before emission. The library has no automatic payload/error redaction policy.

Review these assumptions when collaborators, limits, schemas, retry policies,
tenant boundaries, sinks or deployment recovery change. Reports of violated
library invariants belong through [private reporting](../SECURITY.md). V1 support
and the unresolved public correction boundary are described in
[migration guidance](migration-v2.md), not silently treated as fixed.
