#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

CONTAINER_NAME="${CONTAINER_NAME:-octopus}"
HOST_PORT="${HOST_PORT:-8080}"
CONTAINER_PORT="${CONTAINER_PORT:-8080}"
DATA_DIR="${DATA_DIR:-octopus-data}"

mkdir -p archives "${DATA_DIR}"

shopt -s nullglob
for tar_file in ./*.tar; do
  mv -f "${tar_file}" archives/
done
shopt -u nullglob

latest_gz="$(
  find . -maxdepth 1 -type f -name 'Docker-jialtang-octopus-*.tar.gz' \
    -printf '%T@ %p\n' \
    | sort -nr \
    | awk 'NR == 1 { print $2 }'
)"

if [[ -z "${latest_gz}" ]]; then
  echo "No Docker-jialtang-octopus-*.tar.gz file found in $(pwd)" >&2
  exit 1
fi

echo "Deploying ${latest_gz}"
gunzip -f "${latest_gz}"

image_tar="${latest_gz%.gz}"
load_output="$(docker load -i "${image_tar}")"
echo "${load_output}"

image_name="$(
  echo "${load_output}" \
    | awk -F': ' '/Loaded image:/ { image=$2 } END { print image }'
)"

if [[ -z "${image_name}" ]]; then
  echo "Could not determine loaded Docker image name" >&2
  exit 1
fi

docker stop "${CONTAINER_NAME}" >/dev/null 2>&1 || true
docker rm "${CONTAINER_NAME}" >/dev/null 2>&1 || true

docker run -d \
  --name "${CONTAINER_NAME}" \
  --restart unless-stopped \
  -v "$(pwd)/${DATA_DIR}:/app/data" \
  -p "${HOST_PORT}:${CONTAINER_PORT}" \
  "${image_name}"

docker ps --filter "name=^/${CONTAINER_NAME}$"
mv -f "${image_tar}" archives/
