#!/usr/bin/env bash
set -euo pipefail

mode="${1:-public}"
case "${mode}" in
    candidate|public|selected-public) ;;
    *) printf 'usage: %s [candidate|public|selected-public] [--compile-only]\n' "$0" >&2; exit 2 ;;
esac
compile_only=false
if (( $# > 1 )); then
    if (( $# != 2 )) || [[ "$2" != --compile-only ]]; then
        printf 'usage: %s [candidate|public|selected-public] [--compile-only]\n' "$0" >&2
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
if [[ "${mode}" == selected-public ]]; then
    # Instantiate the same public-contract assertions for the compatibility
    # set's released v2 cohort; retain the existing v3/legacy-Outbox recipe.
    sed \
        -e 's@"github.com/faustbrian/go-workflow/v3@"github.com/faustbrian/go-workflow/v2@g' \
        -e 's@"github.com/faustbrian/go-transactional-outbox/adapters/kafka"@"github.com/faustbrian/go-transactional-outbox/adapters/kafka/v2"@g' \
        -e 's@"github.com/faustbrian/go-transactional-outbox/postgres"@"github.com/faustbrian/go-transactional-outbox/v2/postgres"@g' \
        -e 's@"github.com/faustbrian/go-transactional-outbox"@"github.com/faustbrian/go-transactional-outbox/v2"@g' \
        -e 's|"postgres:18-alpine"|"postgres:18.4-alpine@sha256:9a8afca54e7861fd90fab5fdf4c42477a6b1cb7d293595148e674e0a3181de15"|g' \
        "${task_root}/interoperability_test.go" >"${task_root}/selected_test.go"
    mv "${task_root}/selected_test.go" "${task_root}/interoperability_test.go"
    cp "${module_root}/testdata/interoperability/selected-public.go.mod" "${task_root}/go.mod"
    cp "${module_root}/testdata/interoperability/selected-public.go.sum" "${task_root}/go.sum"
    cp "${module_root}/testdata/interoperability/selected-public-restore_test.go.txt" "${task_root}/restore_test.go"
fi
cd "${task_root}"
export GOCACHE="${task_gocache}"
export GOMODCACHE="${task_modcache}"
export GOTMPDIR="${task_gotmpdir}"
export GOWORK=off
if [[ "${mode}" != candidate ]]; then
    # A public consumer must not resolve an unpublished bootstrap artifact or
    # inherit flags that divert its module inputs or selected test execution.
    export GOENV=off
    export GOPROXY=https://proxy.golang.org
    export GOSUMDB=sum.golang.org
    export GONOPROXY='' GONOSUMDB='' GOPRIVATE=''
    export GOFLAGS=-p=2
fi

if [[ "${mode}" == selected-public ]]; then
    export GOFLAGS='-p=2 -mod=readonly'
    go mod download
    cmp "${module_root}/testdata/interoperability/selected-public.go.mod" go.mod
    cmp "${module_root}/testdata/interoperability/selected-public.go.sum" go.sum
else
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
fi
go mod verify
if [[ "${mode}" == selected-public ]]; then
    test -z "$(go list -m -f '{{if .Replace}}{{.Path}}{{end}}' all)"
    for dependency in \
        github.com/faustbrian/go-workflow/v2@v2.0.0 \
        github.com/faustbrian/go-transactional-outbox/v2@v2.0.0 \
        github.com/faustbrian/go-transactional-outbox/adapters/kafka/v2@v2.0.0 \
        github.com/faustbrian/go-kafka@v1.1.0; do
        path="${dependency%@*}"
        test "$(go list -m -f '{{.Path}}@{{.Version}}' "${path}")" = "${dependency}"
        printf 'selected public dependency: %s\n' "${dependency}"
    done
elif [[ "${mode}" == candidate ]]; then
    test "$(go list -m -f '{{.Replace.Dir}}' "${module_path}")" = "${module_root}"
else
    test "$(go list -m -f '{{if .Replace}}replaced{{end}}' "${module_path}")" = ""
fi
if [[ "${compile_only}" == true ]]; then
    go test ./... -run '^$'
    if [[ "${mode}" == selected-public ]]; then
        go vet ./...
    fi
    printf '%s interoperability compile-only: runtime composition not exercised\n' "${mode}"
else
    if [[ "${mode}" == selected-public ]]; then
        go test -v ./... -count=1 -timeout=10m
    else
        go test ./... -count=1
    fi
    printf '%s interoperability composition passed\n' "${mode}"
fi
