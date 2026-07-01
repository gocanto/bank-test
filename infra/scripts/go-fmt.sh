#!/usr/bin/env bash
set -euo pipefail

image="${GO_FMT_IMAGE:-ghcr.io/oullin/go-fmt:v0.4.2-full}"
project_dir="${GO_FMT_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cache_volume="${GO_FMT_CACHE_VOLUME:-go-fmt-cache}"

if [[ $# -eq 0 ]]; then
	printf 'usage: %s <format|check|sources|version|help> [paths...]\n' "${0##*/}" >&2
	exit 2
fi

docker_args=(
	--rm
	-v "${project_dir}:/work"
	-v "${cache_volume}:/cache"
	-w /work
	-e "HOST_PROJECT_PATH=${project_dir}"
	-e GOCACHE=/cache/go-build
	-e GOPATH=/cache/gopath
	-e GOMODCACHE=/cache/gopath/pkg/mod
)

mode="$1"

if [[ "$mode" == "check" ]]; then
	shift
	paths=("$@")
	if [[ ${#paths[@]} -eq 0 ]]; then
		paths=(.)
	fi

	docker run "${docker_args[@]}" "${image}" check "${paths[@]}"
	docker run "${docker_args[@]}" --entrypoint fmt-lint "${image}" "${paths[@]}"
	exit 0
fi

exec docker run "${docker_args[@]}" "${image}" "$@"
