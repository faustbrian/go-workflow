# V3 migration

V3 releases the root and PostgreSQL packages together at
`github.com/faustbrian/go-workflow/v3`. Use `v3.0.0` only after that stable tag
is published; a source checkout is not evidence of publication.

## Why this is a major release

The pgx 5.11 dependency adds `TypeMap` to `pgx.Rows`. Workflow exposes a
caller-owned `pgx.Tx` through `postgres.Store.Stage`, and `pgx.Tx.Query` returns
`pgx.Rows`. Previously valid custom transaction implementations that return
custom Rows can therefore stop compiling unless those Rows implement the new
method. Workflow's exported-contract compatibility policy treats this as a
source break even though pgx permits interface additions under its own policy.

The module migration makes adoption explicit. It does not change owned
workflow algorithms, history encoding, fingerprints, error classification,
PostgreSQL schemas, or transaction ownership. Minimum Go remains 1.27.0.

## Migrate both imports together

After publication:

```sh
go get github.com/faustbrian/go-workflow/v3@v3.0.0
```

Use `github.com/faustbrian/go-workflow/v3` for workflow values and
`github.com/faustbrian/go-workflow/v3/postgres` for their store. V2 and v3 named
Go types and sentinels remain separate identities; do not mix a v2 transition
with a v3 store, or compare v3 errors only against v2 sentinels. Update callers,
examples, tests, and any version-matched adapter as one coherent cohort.

Custom `pgx.Rows` implementations need `TypeMap() *pgtype.Map`, consistent with
their decoding setup. Wrappers should forward the wrapped Rows' `TypeMap`;
custom mocks should supply the map appropriate to their values. Custom
transactions using concrete pgx Rows do not need a separate Rows method.

## Retain ownership and durable records

`Stage` never commits or rolls back the caller's transaction. A nil result
means staged or exact replay, not confirmed durable success. Publish or
acknowledge work only after the caller confirms commit; an unknown commit
outcome still requires reconciliation.

No PostgreSQL schema or history rewrite is introduced. Persisted definitions
remain pinned to their recorded name, version, and fingerprint. The package
major version does not authorize changing those application identities,
replaying external effects, or converting records during deployment.

## Keep older adapter cohorts explicit

Published CloudEvents Workflow v2 and Golib v3 adapters that consume Workflow
v2 remain v2 consumers. Their adapter major number is not the Workflow major
number. This release does not silently migrate or certify those adapters for
Workflow v3. Use a separately verified version-matched adapter before passing
v3 values through such a typed API.

The owned Workflow/Outbox/Kafka fixture adopts both v3 packages together. Its
candidate mode uses this source; public mode must resolve the actual new tag
without a replacement after publication. Neither mode establishes deployment
or production recovery.

Historical v2 APIs and completed evidence remain retained. See
[v2 migration](migration-v2.md) for the earlier nominal migration and
[verification](verification.md) for the current release boundaries.
