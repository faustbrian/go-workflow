#!/usr/bin/env bash
set -euo pipefail

# Testcontainers 0.44.0 has no Ryuk-image override: reaper.go uses this default
# directly, with AlwaysPullImage false. docker.go first inspects the local tag.
# Populate that alias from an immutable pull, never from the mutable tag.
alias=testcontainers/ryuk:0.14.0
pinned="${alias}@sha256:7c1a8a9a47c780ed0f983770a662f80deb115d95cce3e2daa3d12115b8cd28f0"
initial="${RUNNER_TEMP}/workflow-selected-initial-images"
if [[ ! -f "${initial}" ]]; then
    printf 'Ryuk pin requires a successful initial image snapshot\n' >&2
    exit 1
fi
alias_id="$(docker image ls --quiet --no-trunc "${alias}" | sort -u)"
docker pull "${pinned}"
pinned_id="$(docker image inspect --format '{{.Id}}' "${pinned}")"
if [[ ! "${pinned_id}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    printf 'invalid immutable Ryuk image identity\n' >&2
    exit 1
fi
if [[ -n "${alias_id}" ]]; then
    if [[ "${alias_id}" != "${pinned_id}" ]]; then
        printf 'pre-existing Ryuk default alias differs from pinned image; refusing to overwrite\n' >&2
        exit 1
    fi
else
    if grep -Fxq -- "${pinned_id}" "${initial}"; then
        # Untagging its last tag could delete a pre-existing digest-only image.
        # Reject this runner state instead of adding a tag we cannot safely own.
        printf 'pinned Ryuk image pre-exists without default alias; refusing to modify it\n' >&2
        exit 1
    else
        status=$?
        if (( status != 1 )); then exit "${status}"; fi
    fi
    docker image tag "${pinned}" "${alias}"
fi
if [[ "$(docker image inspect --format '{{.Id}}' "${alias}")" != "${pinned_id}" ]]; then
    printf 'Ryuk default alias does not resolve to pinned image\n' >&2
    exit 1
fi
printf 'Ryuk default alias verified against %s\n' "${pinned}"
