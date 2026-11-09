#!/usr/bin/env bash
# Downloads the pinned Bun release into a directory (#2819).
#
# `oven-sh/setup-bun` retries a 5xx for about 35 seconds and then fails the
# job. A release host that is down for longer than that went red on trunk
# with no code at fault. This tries five times with a backoff that adds up
# to more than 60 seconds, and only a persistent outage fails the step.
#
# Usage: scripts/install-bun.sh <version> <destination>
#   The binary lands in <destination>/bin/bun, with `bunx` beside it.
#
# Environment, test seams that no workflow sets:
#   INSTALL_BUN_BACKOFF_SECONDS  the backoff unit; the wait after attempt N is N times this
#   INSTALL_BUN_CURL, INSTALL_BUN_UNZIP  the binaries to run
set -uo pipefail

[ "$#" -eq 2 ] || {
  echo "usage: scripts/install-bun.sh <version> <destination>" >&2
  exit 2
}
version="$1"
dest="$2"
unit="${INSTALL_BUN_BACKOFF_SECONDS:-15}"
curl_bin="${INSTALL_BUN_CURL:-curl}"
unzip_bin="${INSTALL_BUN_UNZIP:-unzip}"
attempts=5

case "$(uname -m)" in
  x86_64) arch=x64 ;;
  aarch64 | arm64) arch=aarch64 ;;
  *)
    echo "::error::no Bun release for $(uname -m)"
    exit 1
    ;;
esac

url="https://github.com/oven-sh/bun/releases/download/bun-v${version}/bun-linux-${arch}.zip"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

for attempt in $(seq 1 "${attempts}"); do
  echo "Downloading Bun ${version}: attempt ${attempt} of ${attempts}"
  if "${curl_bin}" --fail --silent --show-error --location --max-time 120 --output "${work}/bun.zip" "${url}"; then
    break
  fi
  if [ "${attempt}" -eq "${attempts}" ]; then
    echo "::error::Bun ${version} did not download after ${attempts} attempts: ${url}"
    exit 1
  fi
  delay=$((unit * attempt))
  echo "::warning::Bun download attempt ${attempt} of ${attempts} failed; retrying in ${delay}s"
  sleep "${delay}"
done

"${unzip_bin}" -q -o "${work}/bun.zip" -d "${work}" || exit 1
mkdir -p "${dest}/bin"
install -m 755 "${work}"/bun-linux-*/bun "${dest}/bin/bun" || exit 1
ln -sf bun "${dest}/bin/bunx"
echo "Bun ${version} installed to ${dest}/bin"
