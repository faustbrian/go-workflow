# Compatibility Policy

The repository has one releasable module at
`github.com/faustbrian/go-workflow`. It follows semantic versioning and uses
root tags such as `v1.0.0`. The `workflow` and `postgres` packages are released
together under that tag; `postgres/` is not a nested module and does not use a
directory-prefixed tag.

Before `v1`, minor releases MAY contain reviewed breaking changes, but every
break MUST be documented with migration guidance. Patch releases MUST remain
backward compatible. At and after `v1`, incompatible exported API or documented
behavior changes require a new major version.

Compatibility includes exported Go APIs, error classification, serialization,
protocol behavior, persistence schemas, environment variables, command output,
resource ownership, ordering, retry/idempotency semantics, and documented
defaults. A compile-compatible change can still be behaviorally breaking.

Specification-backed modules MUST NOT diverge from their declared standards.
Ambiguities require documented decisions and stable tests. Deprecated APIs
follow [`DEPRECATION.md`](DEPRECATION.md).

Go 1.27.0 is the supported and release-tested toolchain. Release CI runs on
Ubuntu 24.04 amd64. The packages contain no OS-specific build constraints, but
that does not claim release qualification for every Go-supported platform or
every PostgreSQL deployment.
