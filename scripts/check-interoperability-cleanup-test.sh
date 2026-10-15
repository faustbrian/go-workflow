#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cleanup_script="${WORKFLOW_IMAGE_CLEANUP_SCRIPT:-${root}/scripts/cleanup-interoperability-images.sh}"
capture_script="${WORKFLOW_IMAGE_CAPTURE_SCRIPT:-${root}/scripts/capture-interoperability-images.sh}"
task="$(mktemp -d "${TMPDIR:-/tmp}/workflow-image-cleanup-test.XXXXXX")"
delayed_pid=''
cleanup() {
    if [[ -n "${delayed_pid}" ]]; then
        kill "${delayed_pid}" 2>/dev/null || true
        wait "${delayed_pid}" 2>/dev/null || true
    fi
    rm -r "${task}"
}
trap cleanup EXIT
mkdir "${task}/bin"
cat >"${task}/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == 'image ls --quiet --no-trunc' ]]; then
    case "${FAKE_IMAGE_LIST_FAILURE:-}" in
        empty) exit 9 ;;
        partial) printf 'fixture-image\n'; exit 9 ;;
    esac
    printf '%s' "${FAKE_IMAGES-fixture-image
reaper-image
shared-image
}"
elif [[ "$1 $2" == 'container ls' ]]; then
    if [[ "${FAKE_LIST_FAILURE:-0}" == 1 ]]; then
        printf 'container inspection failed\n' >&2
        exit 9
    fi
    if [[ "$*" == *ancestor=reaper-image* && -f "${FAKE_STATE}/reaper-alive" ]]; then
        printf 'task-owned-ryuk\n'
    fi
elif [[ "$1 $2" == 'image rm' ]]; then
    shift 2
    for image in "$@"; do
        if [[ "${image}" == shared-image && "${FAKE_ALLOW_SHARED_REMOVAL:-0}" != 1 ]]; then
            printf 'attempted pre-existing image removal\n' >&2
            exit 8
        fi
        if [[ "${image}" == reaper-image && -f "${FAKE_STATE}/reaper-alive" ]]; then
            printf 'conflict: image is being used by running container task-owned-ryuk\n' >&2
            exit 1
        fi
        if [[ "${image}" == "${FAKE_REMOVE_FAILURE:-}" ]]; then
            printf 'image removal failed\n' >&2
            exit 7
        fi
        printf '%s\n' "${image}" >>"${FAKE_STATE}/removed"
    done
else
    printf 'unexpected Docker operation\n' >&2
    exit 10
fi
DOCKER
chmod +x "${task}/bin/docker"
export PATH="${task}/bin:${PATH}"

prepare() {
    export RUNNER_TEMP="${task}/$1" FAKE_STATE="${task}/$1"
    mkdir "${RUNNER_TEMP}"
    printf 'shared-image\n' >"${RUNNER_TEMP}/workflow-selected-initial-images"
    : >"${FAKE_STATE}/removed"
}

for failure in empty partial; do
    prepare "capture-failure-${failure}"
    rm "${RUNNER_TEMP}/workflow-selected-initial-images"
    if FAKE_IMAGE_LIST_FAILURE="${failure}" bash "${capture_script}" >"${task}/capture.log" 2>&1; then
        printf 'failed image listing was accepted\n' >&2
        exit 1
    fi
    # The daemon recovers before always-cleanup runs. A failed listing must
    # not turn the now-visible pre-existing images into task-owned images.
    FAKE_ALLOW_SHARED_REMOVAL=1 bash "${cleanup_script}" 1
    if [[ -s "${FAKE_STATE}/removed" ]]; then
        printf 'failed snapshot caused image deletion after daemon recovery\n' >&2
        exit 1
    fi
    test ! -e "${RUNNER_TEMP}/workflow-selected-initial-images"
    test -z "$(find "${RUNNER_TEMP}" -name '*.pending.*' -print)"
done
printf 'failed and partial snapshot capture preserve pre-existing images passed\n'

prepare empty-baseline
rm "${RUNNER_TEMP}/workflow-selected-initial-images"
FAKE_IMAGES='' bash "${capture_script}"
test -f "${RUNNER_TEMP}/workflow-selected-initial-images"
test ! -s "${RUNNER_TEMP}/workflow-selected-initial-images"
FAKE_IMAGES="fixture-image
reaper-image
" bash "${cleanup_script}" 1
printf '%s\n' fixture-image reaper-image >"${task}/expected"
cmp "${task}/expected" "${FAKE_STATE}/removed"
printf 'successful empty baseline cleanup passed\n'

prepare delayed-reaper
rm "${RUNNER_TEMP}/workflow-selected-initial-images"
FAKE_IMAGES='shared-image
' bash "${capture_script}"
touch "${FAKE_STATE}/reaper-alive"
(sleep 0.2; rm "${FAKE_STATE}/reaper-alive") &
delayed_pid=$!
bash "${cleanup_script}" 5
wait "${delayed_pid}"
delayed_pid=''
printf '%s\n' fixture-image reaper-image >"${task}/expected"
cmp "${task}/expected" "${FAKE_STATE}/removed"
test ! -e "${RUNNER_TEMP}/workflow-selected-initial-images"
printf 'delayed reaper exit and pre-existing image preservation passed\n'

prepare deadline
touch "${FAKE_STATE}/reaper-alive"
if bash "${cleanup_script}" 1 >"${task}/deadline.log" 2>&1; then
    printf 'cleanup accepted a permanently referenced image\n' >&2
    exit 1
fi
test ! -s "${FAKE_STATE}/removed"
test -f "${RUNNER_TEMP}/workflow-selected-initial-images"
grep -F 'timed out waiting for task-owned image references' "${task}/deadline.log" >/dev/null
printf 'cleanup deadline fails closed passed\n'

prepare removal-failure
if FAKE_REMOVE_FAILURE=fixture-image bash "${cleanup_script}" 1 >"${task}/remove.log" 2>&1; then
    printf 'cleanup accepted image removal failure\n' >&2
    exit 1
fi
test -f "${RUNNER_TEMP}/workflow-selected-initial-images"
grep -F 'image removal failed' "${task}/remove.log" >/dev/null
printf 'image removal failure remains fatal passed\n'

prepare inspection-failure
if FAKE_LIST_FAILURE=1 bash "${cleanup_script}" 1 >"${task}/inspect.log" 2>&1; then
    printf 'cleanup accepted container inspection failure\n' >&2
    exit 1
fi
test ! -s "${FAKE_STATE}/removed"
test -f "${RUNNER_TEMP}/workflow-selected-initial-images"
printf 'container inspection failure remains fatal passed\n'
