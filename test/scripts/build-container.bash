#!/usr/bin/env bash
TAG="${1:-${IMAGE_TAG:-latest}}"
IMAGE="ghcr.io/nblair2/opendnp3-demo:${TAG}"

# source top-level .env (resolve repo root relative to this script)
scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$scriptdir/../.." && pwd)"
if [ -f "$repo_root/.env" ]; then
	. "$repo_root/.env"
fi

echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GITHUB_USER" --password-stdin || true
# Pass any additional arguments to docker build
shift 1
docker build "$@" -t "${IMAGE}" -f test/docker/Dockerfile test/docker
docker push "${IMAGE}"
