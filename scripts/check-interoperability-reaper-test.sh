#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pin_script="${WORKFLOW_REAPER_PIN_SCRIPT:-${root}/scripts/pin-interoperability-reaper.sh}"
task="$(mktemp -d "${TMPDIR:-/tmp}/workflow-reaper-pin-test.XXXXXX")"
trap 'rm -r "${task}"' EXIT
mkdir "${task}/bin"
cat >"${task}/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
set -euo pipefail
alias=testcontainers/ryuk:0.14.0
pinned="${alias}@sha256:7c1a8a9a47c780ed0f983770a662f80deb115d95cce3e2daa3d12115b8cd28f0"
if [[ "$*" == 'image ls --quiet --no-trunc' ]]; then
    cat "${RUNNER_TEMP}/images"
elif [[ "$*" == "image ls --quiet --no-trunc ${alias}" ]]; then
    if [[ "${FAKE_LIST_FAILURE:-0}" == 1 ]]; then exit 9; fi
    if [[ -f "${RUNNER_TEMP}/alias" ]]; then cat "${RUNNER_TEMP}/alias"; fi
elif [[ "$1" == pull && "$2" == "${pinned}" ]]; then
    if [[ "${FAKE_PULL_FAILURE:-0}" == 1 ]]; then exit 8; fi
    printf '%s\n' "${FAKE_PINNED_ID}" >>"${RUNNER_TEMP}/images"
    sort -u "${RUNNER_TEMP}/images" -o "${RUNNER_TEMP}/images"
elif [[ "$1 $2" == 'image inspect' && "$3" == --format && "$4" == '{{.Id}}' ]]; then
    if [[ "${FAKE_INSPECT_FAILURE:-0}" == 1 ]]; then exit 7; fi
    if [[ "$5" == "${pinned}" ]]; then
        printf '%s\n' "${FAKE_PINNED_ID}"
    elif [[ "$5" == "${alias}" ]]; then
        cat "${RUNNER_TEMP}/alias"
    else exit 6; fi
elif [[ "$*" == "image tag ${pinned} ${alias}" ]]; then
    printf '%s\n' "${FAKE_PINNED_ID}" >"${RUNNER_TEMP}/alias"
    printf 'tag\n' >>"${RUNNER_TEMP}/tags"
elif [[ "$1 $2" == 'container ls' ]]; then
    :
elif [[ "$1 $2" == 'image rm' ]]; then
    if grep -Fxq -- "$3" "${RUNNER_TEMP}/preexisting"; then
        printf 'attempted pre-existing image removal\n' >&2; exit 5
    fi
    printf '%s\n' "$3" >>"${RUNNER_TEMP}/removed"
    if [[ -f "${RUNNER_TEMP}/alias" && "$(cat "${RUNNER_TEMP}/alias")" == "$3" ]]; then
        rm "${RUNNER_TEMP}/alias"
    fi
else
    printf 'unexpected Docker operation\n' >&2; exit 4
fi
DOCKER
chmod +x "${task}/bin/docker"
export PATH="${task}/bin:${PATH}"
FAKE_PINNED_ID="sha256:$(printf 'a%.0s' {1..64})"
export FAKE_PINNED_ID
other_id="sha256:$(printf 'b%.0s' {1..64})"

prepare() {
    export RUNNER_TEMP="${task}/$1"
    mkdir "${RUNNER_TEMP}"
    printf '%s' "$2" >"${RUNNER_TEMP}/images"
    cp "${RUNNER_TEMP}/images" "${RUNNER_TEMP}/preexisting"
    : >"${RUNNER_TEMP}/tags"
    : >"${RUNNER_TEMP}/removed"
    bash "${root}/scripts/capture-interoperability-images.sh"
}

prepare new-image ''
bash "${pin_script}"
test "$(cat "${RUNNER_TEMP}/alias")" = "${FAKE_PINNED_ID}"
bash "${root}/scripts/cleanup-interoperability-images.sh" 1
test "$(cat "${RUNNER_TEMP}/removed")" = "${FAKE_PINNED_ID}"
test ! -e "${RUNNER_TEMP}/alias"
printf 'immutable pull, new alias and owned-image cleanup passed\n'

prepare matching-alias "${FAKE_PINNED_ID}"$'\n'
printf '%s\n' "${FAKE_PINNED_ID}" >"${RUNNER_TEMP}/alias"
bash "${pin_script}"
bash "${root}/scripts/cleanup-interoperability-images.sh" 1
test ! -s "${RUNNER_TEMP}/tags"
test ! -s "${RUNNER_TEMP}/removed"
test "$(cat "${RUNNER_TEMP}/alias")" = "${FAKE_PINNED_ID}"
printf 'matching pre-existing alias preserved passed\n'

for case_name in mismatched-alias preexisting-without-alias; do
    if [[ "${case_name}" == mismatched-alias ]]; then
        prepare "${case_name}" "${other_id}"$'\n'
        printf '%s\n' "${other_id}" >"${RUNNER_TEMP}/alias"
    else
        prepare "${case_name}" "${FAKE_PINNED_ID}"$'\n'
    fi
    if bash "${pin_script}" >"${task}/failure.log" 2>&1; then
        printf 'unsafe pre-existing Ryuk state accepted\n' >&2; exit 1
    fi
    test ! -s "${RUNNER_TEMP}/tags"
    bash "${root}/scripts/cleanup-interoperability-images.sh" 1
    if [[ "${case_name}" == mismatched-alias ]]; then
        test "$(cat "${RUNNER_TEMP}/alias")" = "${other_id}"
        test "$(cat "${RUNNER_TEMP}/removed")" = "${FAKE_PINNED_ID}"
    else
        test ! -e "${RUNNER_TEMP}/alias"
        test ! -s "${RUNNER_TEMP}/removed"
    fi
done
printf 'mismatched alias and pre-existing unaliased image fail closed passed\n'

for failure in LIST PULL INSPECT; do
    prepare "failure-${failure}" ''
    if env "FAKE_${failure}_FAILURE=1" bash "${pin_script}" >"${task}/failure.log" 2>&1; then
        printf 'failed Ryuk pin operation accepted\n' >&2; exit 1
    fi
    test ! -s "${RUNNER_TEMP}/tags"
    bash "${root}/scripts/cleanup-interoperability-images.sh" 1
    if [[ "${failure}" == INSPECT ]]; then
        test "$(cat "${RUNNER_TEMP}/removed")" = "${FAKE_PINNED_ID}"
    else
        test ! -s "${RUNNER_TEMP}/removed"
    fi
done
printf 'listing, immutable pull and identity-inspection failures cleaned safely passed\n'
