#!/usr/bin/env bash
set -euo pipefail

initial="${RUNNER_TEMP}/workflow-selected-initial-images"
pending="$(mktemp "${initial}.pending.XXXXXX")"
trap 'rm -f "${pending}"' EXIT

# File existence is cleanup's ownership boundary. Publish only a complete,
# successful listing; a failed pipeline must not leave an empty/partial baseline.
docker image ls --quiet --no-trunc | sort -u >"${pending}"
mv "${pending}" "${initial}"
