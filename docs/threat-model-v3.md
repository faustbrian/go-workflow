# V3 source model

V3 retains the v2 admission, defensive-copy, history, replay, and bounded-worker
contracts described in the [v2 source model](threat-model-v2.md). The nominal
module migration does not itself create a new security guarantee or alter
persisted schemas, resource ownership, retries, fencing, or unknown outcomes.

The public caller-owned PostgreSQL transaction seam still uses `pgx.Tx`.
Pgx 5.11 requires custom Rows implementations to provide `TypeMap`; the
[migration guide](migration-v3.md) explains that compiler-visible source break
and the distinct v2/v3 package identities. The store still does not commit or
roll back a transaction supplied to `Stage`.

Use version-matched root, PostgreSQL, errors, and adapters. Retained v2 adapter
cohorts are not v3-compatible merely because a dependency or an adapter has a
new major number. Candidate composition, actual public-tag composition, and
production adoption are separate evidence boundaries.

The public Go minimum remains 1.27.0. Local maintenance verification with
Go 1.27.2 does not imply every consumer rebuilt with that compiler or every
supported platform was release-qualified. Publication and runtime claims must
follow their attributable release and deployment evidence.
