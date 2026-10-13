# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected package's docs.
2. Run the narrow standalone baseline for the affected package.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.

Selected mutation gates must finish with zero surviving viable mutants.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Use Go 1.27.2 for development and ordinary CI; the public minimum remains
Go 1.27.0. A standalone checkout supports ordinary untagged tests and a
focused active fuzz smoke without sibling repositories or goal ledgers:

```bash
GOWORK=off go test ./...
GOWORK=off go test . -run '^$' -fuzz '^FuzzWorkDecisionBoundaries$' -fuzztime=2s
```

Select the native fuzz target affected by your change. Ordinary tests exercise
deterministic fuzz seeds and corpus; base-free local checks do not start an
active fuzz campaign. Install the public `golib` v2.4.0 release declared in
`.golib.yaml` and verify its checksums for the shared routes:

```bash
make check
make ci
```

Both Make routes are bounded local checks, including one race run of worker
shutdown/join and persisted race-winner assertions. They retain the catalog's
`integration` tag: PostgreSQL tests need `WORKFLOW_POSTGRES_URL` or Docker for
Testcontainers. The untagged Go command above is a distinct backend-free route.
Ordinary CI uses the published verifier without building it from source;
changed-source CI can select a bounded active fuzz smoke. Optional mutation
analysis can select only the root package:

```bash
golib mutation --module . --package .
```

Use `make check-full` for explicit full assurance, including online specification
validation, the unchanged 10000x fuzz targets, full race/coverage/mutation,
API, docs, benchmarks and PostgreSQL/Kafka interoperability. Persistence changes
need the tagged backend tests; lifecycle or public-contract changes need the
directly affected consumers. The explicit source-owned commands and separate
candidate/public composition boundaries are in
[verification guidance](docs/verification.md). Full campaigns remain available
for selected scheduled or release qualification; ordinary scheduled CI remains
bounded. Release rehearsal is explicitly selected, not an ordinary CI result.
Report unavailable or failing checks without claiming release readiness.
CI/configuration-only changes do not require a library release.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.

## Source Documentation

Install the checksum-pinned public verifier declared in `.golib.yaml`.
`make source-docs-check` validates objective Go comment, marker, link and
generated-producer structure; `make check`, `make check-full` and `make ci`
require it before their existing checks. Shared CI runs the same strict
contract and includes its result in Required. This is structural enforcement,
not a substitute for source-truth review or behavioral verification.
