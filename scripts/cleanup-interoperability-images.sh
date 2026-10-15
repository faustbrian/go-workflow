#!/usr/bin/env bash
set -euo pipefail

timeout_seconds="${1:-60}"
if [[ ! "${timeout_seconds}" =~ ^[0-9]+$ ]] || (( timeout_seconds < 1 || timeout_seconds > 120 )); then
    printf 'cleanup timeout must be between 1 and 120 seconds\n' >&2
    exit 2
fi
initial="${RUNNER_TEMP}/workflow-selected-initial-images"
final="${RUNNER_TEMP}/workflow-selected-final-images"
owned="${RUNNER_TEMP}/workflow-selected-owned-images"
if [[ ! -f "${initial}" ]]; then
    exit 0
fi
docker image ls --quiet --no-trunc | sort -u >"${final}"
comm -13 "${initial}" "${final}" >"${owned}"

# Ryuk auto-removes itself only after its reconnection grace and cleanup pass.
# Wait for all image references to disappear; never stop a container or force
# image removal, including when an unexpected reference outlives the deadline.
deadline=$((SECONDS + timeout_seconds))
while :; do
    referenced=false
    while IFS= read -r image; do
        containers="$(docker container ls --all --quiet --filter "ancestor=${image}")"
        if [[ -n "${containers}" ]]; then
            referenced=true
        fi
    done <"${owned}"
    if [[ "${referenced}" == false ]]; then
        break
    fi
    if (( SECONDS >= deadline )); then
        printf 'timed out waiting for task-owned image references to disappear\n' >&2
        exit 1
    fi
    sleep 1
done
while IFS= read -r image; do
    docker image rm "${image}"
done <"${owned}"
rm "${initial}" "${final}" "${owned}"
