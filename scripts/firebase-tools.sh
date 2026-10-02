#!/usr/bin/env bash
# The one way a workflow runs `firebase-tools` (#1624).
#
# It runs the copy that the root `package.json` devDependency and the root
# `bun.lock` hold, from the root `node_modules`. It never resolves a
# release from the registry: `bunx firebase-tools` with no root install
# downloaded the newest release on every run, so the version that deployed
# was in no file in this repository and in no line of the log. A job that
# did not run `bun install --frozen-lockfile` at the repository root fails
# here and says so.
#
# Usage:
#   scripts/firebase-tools.sh version          print the pinned version
#   scripts/firebase-tools.sh once  <args...>  one attempt, the command's
#                                              own output and exit status
#   scripts/firebase-tools.sh retry <args...>  the version line, then up to
#                                              three attempts
#
# `retry` carries the properties that #435 gave `.github/actions/gcp-auth`:
# three attempts, a 10 second then a 20 second backoff, a `::warning::`
# for each failed attempt and an `::error::` after the last. A hosting
# deploy and a channel deploy are safe to repeat: each one makes a new
# version and releases it. `once` is for a command whose non-zero exit is
# an ordinary answer (the preview cleanup's 404), which three attempts
# would only make slower.
#
# Environment:
#   FIREBASE_TOOLS_STDOUT_FILE  `retry` only. Each attempt's stdout goes to
#       this file (emptied before each attempt) and not to the log, so a
#       caller can read the `--json` result of the attempt that passed.
#       A failed attempt's file is printed to the log, because `--json`
#       sends the command's error to stdout too (#736).
#   FIREBASE_TOOLS_BIN, FIREBASE_TOOLS_BACKOFF_SECONDS  test seams: the
#       binary to run and the backoff unit. No workflow sets them.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${FIREBASE_TOOLS_BIN:-${root}/node_modules/.bin/firebase}"
backoff="${FIREBASE_TOOLS_BACKOFF_SECONDS:-10}"
attempts=3

usage() {
  echo "usage: scripts/firebase-tools.sh version | once <args...> | retry <args...>" >&2
  exit 2
}

require_install() {
  if [ ! -x "${bin}" ]; then
    echo "::error::firebase-tools is not installed at ${bin}. Run \`bun install --frozen-lockfile\` at the repository root before this step; this script does not download a release."
    exit 1
  fi
}

print_version() {
  local version
  if ! version="$("${bin}" --version)"; then
    echo "::error::firebase-tools did not print its version"
    exit 1
  fi
  echo "firebase-tools version: ${version} (the root bun.lock pin)"
}

run_attempt() {
  if [ -n "${FIREBASE_TOOLS_STDOUT_FILE:-}" ]; then
    "${bin}" "$@" > "${FIREBASE_TOOLS_STDOUT_FILE}"
  else
    "${bin}" "$@"
  fi
}

show_failure() {
  if [ -n "${FIREBASE_TOOLS_STDOUT_FILE:-}" ] && [ -s "${FIREBASE_TOOLS_STDOUT_FILE}" ]; then
    echo "The command's output follows."
    cat "${FIREBASE_TOOLS_STDOUT_FILE}"
    echo
  fi
}

retry() {
  local label="firebase-tools $1"
  local attempt status wait
  print_version
  for attempt in $(seq 1 "${attempts}"); do
    echo "${label}: attempt ${attempt} of ${attempts}"
    run_attempt "$@"
    status=$?
    if [ "${status}" -eq 0 ]; then
      echo "${label} passed on attempt ${attempt} of ${attempts}"
      return 0
    fi
    show_failure
    if [ "${attempt}" -lt "${attempts}" ]; then
      wait=$((backoff * attempt))
      echo "::warning::${label} attempt ${attempt} of ${attempts} failed (exit ${status}); retrying in ${wait}s"
      sleep "${wait}"
    else
      echo "::warning::${label} attempt ${attempt} of ${attempts} failed (exit ${status})"
    fi
  done
  echo "::error::${label} failed after ${attempts} attempts"
  return 1
}

[ "$#" -ge 1 ] || usage
mode="$1"
shift

case "${mode}" in
  version)
    require_install
    print_version
    ;;
  once)
    [ "$#" -ge 1 ] || usage
    require_install
    "${bin}" "$@"
    ;;
  retry)
    [ "$#" -ge 1 ] || usage
    require_install
    retry "$@"
    ;;
  *)
    usage
    ;;
esac
