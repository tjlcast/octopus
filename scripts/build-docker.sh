#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-octopus}"
IMAGE_NAME="${IMAGE_NAME:-jialtang/octopus}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
TARGET_PLATFORM="${TARGET_PLATFORM:-linux/amd64}"
EXPORT_IMAGE="${EXPORT_IMAGE:-false}"
OUTPUT_DIR="${OUTPUT_DIR:-build}"
DOCKERFILE="${DOCKERFILE:-scripts/dockerfiles/Dockerfile.alpine}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

case "${TARGET_PLATFORM}" in
  linux/amd64) GOOS=linux; GOARCH=amd64 ;;
  linux/arm64) GOOS=linux; GOARCH=arm64 ;;
  linux/386) GOOS=linux; GOARCH=386 ;;
  linux/arm/v7) GOOS=linux; GOARCH=arm; GOARM=7 ;;
  *) echo "Unsupported TARGET_PLATFORM: ${TARGET_PLATFORM}" >&2; exit 1 ;;
esac

VERSION="${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null || echo dev)}"
COMMIT_ID="${COMMIT_ID:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME="${BUILD_TIME:-$(TZ=Asia/Shanghai date '+%F %T %z')}"
AUTHOR="${AUTHOR:-bestrui}"
IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"
SAFE_IMAGE_NAME="${IMAGE_NAME//\//-}"
ASSET_NAME="${ASSET_NAME:-Docker-${SAFE_IMAGE_NAME}-${IMAGE_TAG}.tar.gz}"

LDFLAGS="-X 'github.com/bestruirui/octopus/internal/conf.Version=${VERSION}' \
-X 'github.com/bestruirui/octopus/internal/conf.BuildTime=${BUILD_TIME}' \
-X 'github.com/bestruirui/octopus/internal/conf.Author=${AUTHOR}' \
-X 'github.com/bestruirui/octopus/internal/conf.Commit=${COMMIT_ID}' \
-s -w"

echo "Building frontend..."
(
  cd web
  pnpm install --frozen-lockfile
  NEXT_PUBLIC_APP_VERSION="${VERSION}" pnpm run build
)

rm -rf static/out
mv web/out static/out

echo "Updating price data..."
python3 scripts/updatePrice.py

echo "Building Go backend for ${TARGET_PLATFORM}..."
mkdir -p "${OUTPUT_DIR}/docker/${TARGET_PLATFORM}"
if [[ -n "${GOARM:-}" ]]; then
  GOOS="${GOOS}" GOARCH="${GOARCH}" GOARM="${GOARM}" CGO_ENABLED=0 \
    go build -o "${OUTPUT_DIR}/docker/${TARGET_PLATFORM}/${APP_NAME}" \
      -ldflags="${LDFLAGS}" -tags=jsoniter .
else
  GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED=0 \
    go build -o "${OUTPUT_DIR}/docker/${TARGET_PLATFORM}/${APP_NAME}" \
      -ldflags="${LDFLAGS}" -tags=jsoniter .
fi

echo "Building Docker image ${IMAGE_REF}..."
docker build \
  --pull \
  --build-arg "TARGETPLATFORM=${TARGET_PLATFORM}" \
  -f "${DOCKERFILE}" \
  -t "${IMAGE_REF}" \
  .

if [[ "${EXPORT_IMAGE}" == "true" ]]; then
  echo "Exporting Docker image to ${ASSET_NAME}..."
  docker save "${IMAGE_REF}" | gzip -9 > "${ASSET_NAME}"
  gzip -t "${ASSET_NAME}"
  ls -lh "${ASSET_NAME}"
fi

echo "Docker image ready: ${IMAGE_REF}"
