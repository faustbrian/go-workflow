# Support

GitHub Issues and Discussions are not enabled for this repository. There is no
published ordinary support contact or response commitment.

For self-service adoption and diagnosis, use the [README](README.md),
[API and lifecycle reference](docs/reference.md), and
[troubleshooting guide](docs/troubleshooting.md). Keep the module and Go
versions, platform, minimal reproduction, and non-secret diagnostics when
investigating a defect.

For suspected vulnerabilities, use the enabled private reporting process in
[`SECURITY.md`](SECURITY.md), not an ordinary support channel.

Support covers released module versions according to
[`COMPATIBILITY.md`](COMPATIBILITY.md) and [`SECURITY.md`](SECURITY.md), including
the latest patch of published stable v1 and v2 lines. Unreleased changes on
`main` are not a released compatibility set. A correction may require a newer
major rather than a backport. The constructor-admission correction belongs to the v2
source, not v1.0.0; public availability requires a published stable v2 tag. See
[migration guidance](docs/migration-v2.md) for the exact distinction.
