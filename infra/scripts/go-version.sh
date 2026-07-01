#!/usr/bin/env bash
set -euo pipefail

image="${GO_FMT_IMAGE:-ghcr.io/oullin/go-fmt:v0.4.2-full}"

docker run --rm --entrypoint go "${image}" version | awk '{ sub(/^go/, "", $3); print $3 }'
