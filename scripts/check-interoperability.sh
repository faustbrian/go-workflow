#!/usr/bin/env bash
set -euo pipefail

mode="${1:-public}"
case "${mode}" in
    candidate|public) ;;
    *) printf 'usage: %s [candidate|public] [--compile-only]\n' "$0" >&2; exit 2 ;;
esac
compile_only=false
if (( $# > 1 )); then
    if (( $# != 2 )) || [[ "$2" != --compile-only ]]; then
        printf 'usage: %s [candidate|public] [--compile-only]\n' "$0" >&2
        exit 2
    fi
    compile_only=true
fi

module_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
module_path=github.com/faustbrian/go-workflow/v3
if [[ "$(awk '$1 == "module" { print $2 }' "${module_root}/go.mod")" != "${module_path}" ]]; then
    printf 'owning Workflow module identity mismatch\n' >&2
    exit 1
fi
# Public is the default and never replaces Workflow. Candidate composition uses
# this exact owning source only in the disposable, non-releasable fixture.
task_root="$(mktemp -d "${TMPDIR:-/tmp}/workflow-interoperability.XXXXXX")"
task_gocache="$(mktemp -d "${TMPDIR:-/tmp}/workflow-interoperability-build.XXXXXX")"
task_modcache="$(mktemp -d "${TMPDIR:-/tmp}/workflow-interoperability-modules.XXXXXX")"
task_gotmpdir="$(mktemp -d "${TMPDIR:-/tmp}/workflow-interoperability-tmp.XXXXXX")"

cleanup() {
    chmod -R u+w "${task_root}" "${task_gocache}" "${task_modcache}" "${task_gotmpdir}" 2>/dev/null || true
    find "${task_root}" -depth -delete
    find "${task_gocache}" -depth -delete
    find "${task_modcache}" -depth -delete
    find "${task_gotmpdir}" -depth -delete
}
trap cleanup EXIT

cp "${module_root}/testdata/interoperability/interoperability_test.go.txt" \
    "${task_root}/interoperability_test.go"
cd "${task_root}"
export GOCACHE="${task_gocache}"
export GOMODCACHE="${task_modcache}"
export GOTMPDIR="${task_gotmpdir}"
export GOWORK=off
if [[ "${mode}" == public ]]; then
    # A public consumer must not resolve an unpublished bootstrap artifact or
    # inherit flags that divert its module inputs or selected test execution.
    export GOENV=off
    export GOPROXY=https://proxy.golang.org
    export GOSUMDB=sum.golang.org
    export GONOPROXY= GONOSUMDB= GOPRIVATE=
    export GOFLAGS=-p=2
fi

go mod init workflow-interoperability.invalid/test
go mod edit -go=1.27.0
go mod edit -require=github.com/faustbrian/go-workflow/v3@v3.0.0
go mod edit -require=github.com/faustbrian/go-transactional-outbox@v1.0.0
go mod edit -require=github.com/faustbrian/go-kafka@v1.0.0
go mod edit -require=github.com/faustbrian/go-transactional-outbox/adapters/kafka@v1.0.0
if [[ "${mode}" == candidate ]]; then
    go mod edit "-replace=${module_path}=${module_root}"
fi
go mod tidy
go mod verify
if [[ "${mode}" == candidate ]]; then
    test "$(go list -m -f '{{.Replace.Dir}}' "${module_path}")" = "${module_root}"
else
    test "$(go list -m -f '{{if .Replace}}replaced{{end}}' "${module_path}")" = ""
fi
if [[ "${compile_only}" == true ]]; then
    go test ./... -run '^$'
    printf '%s interoperability compile-only: runtime composition not exercised\n' "${mode}"
else
    go test ./... -count=1
    printf '%s interoperability composition passed\n' "${mode}"
fi
