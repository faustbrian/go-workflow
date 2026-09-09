#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
required=(
    README.md CHANGELOG.md COMPATIBILITY.md CONTRIBUTING.md DEPRECATION.md
    LICENSE SECURITY.md SUPPORT.md docs/README.md docs/architecture.md
    docs/operations.md docs/reference.md docs/troubleshooting.md
    docs/verification.md example_test.go durable_compensation_example_test.go
    postgres/durable_compensation_recipe_integration_test.go
)

cd "${root}"
quickstart="$(mktemp -d "${TMPDIR:-/tmp}/go-workflow-docs.XXXXXX")"
trap 'find "${quickstart}" -depth -delete' EXIT HUP INT TERM

for path in "${required[@]}"; do
    test -s "${path}" || {
        printf 'missing required workflow documentation: %s\n' "${path}" >&2
        exit 1
    }
done

awk '
    $0 == "## Quick start" { section = 1; next }
    section && $0 == "```go" { capture = 1; next }
    capture && $0 == "```" { exit }
    capture { print }
' README.md > "${quickstart}/main.go"
test -s "${quickstart}/main.go"
go run "${quickstart}/main.go"

for package in \
    github.com/faustbrian/go-workflow \
    github.com/faustbrian/go-workflow/postgres; do
    go doc "${package}" >/dev/null
done

go test -json . \
    -run '^Example_(durableOrchestration|durableCompensationRecovery)$' \
    -count=1 \
    > "${quickstart}/example-test.json"
for example in Example_durableOrchestration Example_durableCompensationRecovery; do
    jq -se --arg example "${example}" '
        any(.[]; .Action == "pass" and .Test == $example)
    ' "${quickstart}/example-test.json" >/dev/null
done

grep -Fq 'go get github.com/faustbrian/go-workflow@v1.0.0' README.md
grep -Fq 'docs/README.md' README.md
grep -Fq 'docs/troubleshooting.md' README.md
grep -Fq 'example_test.go' README.md

printf 'workflow documentation and executable examples are present\n'
