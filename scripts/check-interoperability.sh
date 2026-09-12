#!/usr/bin/env bash
set -euo pipefail

module_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
task_root="$(mktemp -d)"
task_gocache="$(mktemp -d)"
task_modcache="$(mktemp -d)"
task_gotmpdir="$(mktemp -d)"

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

go mod init workflow-interoperability.invalid/test
go mod edit -go=1.27.0
go mod edit -require=github.com/faustbrian/go-workflow@v1.0.0
go mod edit -require=github.com/faustbrian/go-transactional-outbox@v1.0.0
go mod edit -require=github.com/faustbrian/go-kafka@v1.0.0
go mod edit -require=github.com/faustbrian/go-transactional-outbox/adapters/kafka@v1.0.0
go mod tidy
go test ./... -count=1
