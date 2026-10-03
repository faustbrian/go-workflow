# V2 migration and pending publication

Main targets `github.com/faustbrian/go-workflow/v2` version 2.0.0 with minimum
Go 1.27.0. It stays at the repository root; PostgreSQL remains in the same module
at `github.com/faustbrian/go-workflow/v2/postgres`. Publication is pending. After
release, install with `go get github.com/faustbrian/go-workflow/v2@v2.0.0` and
change both imports together. V1 and v2 types are distinct, even where their
declarations match; do not mix definitions, stores or processor contracts.

The nominal migration does not change algorithms, schemas, fingerprints, error
classification or accepted-value ownership. Main already contains a separate
constructor-admission correction: five constructors validate borrowed views
before allocating defensive copies. Published v1.0.0 at
`aef739f62aa389008eebcbedb74492d2e3067b9e` copied first and declares Go 1.26.6.
This document does not claim that correction was backported or publicly released.

Historical v1 API baseline bytes remain in `api/baseline.txt`; current v2 API
checks use `api/v2-baseline.txt`. The public declarations are preserved except
for nominal import identity. Existing durable records and definition versions
must still obey the established replay and rolling-deployment contracts.

Both `github.com/faustbrian/go-cloudevents/adapters/workflow` and
`github.com/faustbrian/go-cloudevents/adapters/golib` currently consume v1
Workflow types. These existing graphs, and Tools' historical consumer, are not
rewritten here. Their deliberate owned-major migrations and actual public v2
composition must happen separately after publication. Coordination inventory
is not proof of adopted imports or a compatible public consumer.

Shared CI retains the published Tools v1.7.1 workflow pin. Tools/v2 is not yet
published; future scanner/diagnostic adoption requires a qualified public
version and its own compatible configuration review, not an unpublished SHA
or fabricated checksum. Existing gates remain unchanged.

`make -f verification/package.mk interoperability` selects explicit candidate
composition before tagging:
only its disposable non-releasable fixture replaces Workflow with the exact
owning module. The script's default and explicit `public` modes instead require
public v2.0.0 with no Workflow replacement, and must run after publication.
Neither candidate composition nor `--compile-only` is public adoption proof;
compile-only does not exercise PostgreSQL/Kafka runtime behavior. Required
release CI, rehearsal, publication and actual public-consumer verification
remain separate gates.
