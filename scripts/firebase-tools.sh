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
#   FIREBASE_TOOLS_BIN, FIREBASE_TOOLS_LOCKFILE, FIREBASE_TOOLS_BACKOFF_SECONDS
#       test seams: the binary to run, the lockfile that holds the pin,
#       and the backoff unit. No workflow sets them.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${FIREBASE_TOOLS_BIN:-${root}/node_modules/.bin/firebase}"
lockfile="${FIREBASE_TOOLS_LOCKFILE:-${root}/bun.lock}"
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

# The version line, checked against the root bun.lock and not only
# labeled with it: an install that holds any other version (a stale
# cache, an install without the frozen lockfile) fails here.
print_version() {
  local version pinned
  if ! version="$("${bin}" --version)"; then
    echo "::error::firebase-tools did not print its version"
    exit 1
  fi
  pinned="$(grep -o '"firebase-tools@[^"]*"' "${lockfile}" | head -n 1 | tr -d '"')"
  if [ "firebase-tools@${version}" != "${pinned}" ]; then
    echo "::error::firebase-tools ${version} is installed, but the root bun.lock holds ${pinned:-no firebase-tools}. Run \`bun install --frozen-lockfile\` at the repository root."
    exit 1
  fi
  echo "firebase-tools version: ${version} (the root bun.lock pin)"
}

# What a failed attempt shows, and why it takes a hard link.
#
# `firebase-tools` hides the cause of an authentication failure: it
# prints `Failed to authenticate, have you run firebase login?` for every
# one, and writes the error it caught (`error.original.stack`) at debug
# level only. Debug lines go to `firebase-debug.log` in the working
# directory on every run, but the CLI deletes that file when it exits
# with a status below 2, and its own errors exit with 1. So the one
# record of the cause is gone before the step ends. (Read in the 15.30.2
# source: lib/requireAuth.js, lib/logError.js, lib/error.js, lib/logger.js
# and lib/bin/cli.js.)
#
# The CLI opens a `firebase-debug.log` that is already there and appends
# to it, and its delete removes that one name. So this script makes the
# file before each attempt and gives it a second name; the second name
# keeps the content. `--debug` would keep the file too, but it also sends
# every debug line of a passing deploy to a public log.
debug_log="${PWD}/firebase-debug.log"
kept_log="${PWD}/firebase-debug.kept.log"
kept_lines=80

drop_debug_log() {
  rm -f "${debug_log}" "${kept_log}"
}

keep_debug_log() {
  drop_debug_log
  : > "${debug_log}" && ln "${debug_log}" "${kept_log}" 2> /dev/null
}

# The repository is public and so are its logs. The debug log holds no
# credential on the hosting path: the CLI logs no Authorization header,
# and the token exchange is google-auth-library's own request, of which
# only the error's stack reaches the log. This filter is the boundary
# that enforces it whatever a later release writes.
redact() {
  sed -E \
    -e 's/ya29\.[A-Za-z0-9._-]+/[redacted]/g' \
    -e 's/eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]*)?/[redacted]/g' \
    -e 's/([Bb]earer|[Bb]asic) +[A-Za-z0-9._~+\/=-]+/\1 [redacted]/g' \
    -e 's/("?[A-Za-z_-]*([Tt]oken|[Ss]ecret|[Pp]assword|private_key|[Aa]pi[_-]?[Kk]ey)[A-Za-z_-]*"?[:=] *"?)[^",} ]+/\1[redacted]/g'
}

kept_count() {
  if [ -f "${kept_log}" ]; then
    wc -l < "${kept_log}" | tr -d ' '
  else
    echo 0
  fi
}

show_debug_log() {
  if [ -s "${kept_log}" ]; then
    echo "::group::firebase-tools debug log of the failed attempt (last ${kept_lines} lines, credentials redacted)"
    tail -n "${kept_lines}" "${kept_log}" | cut -c1-2000 | redact
    echo "::endgroup::"
  else
    echo "firebase-tools left no debug log for this attempt."
  fi
}

run_attempt() {
  keep_debug_log
  if [ -n "${FIREBASE_TOOLS_STDOUT_FILE:-}" ]; then
    "${bin}" "$@" > "${FIREBASE_TOOLS_STDOUT_FILE}"
  else
    "${bin}" "$@"
  fi
}

show_failure() {
  if [ -n "${FIREBASE_TOOLS_STDOUT_FILE:-}" ] && [ -s "${FIREBASE_TOOLS_STDOUT_FILE}" ]; then
    echo "The command's output follows (credentials redacted)."
    redact < "${FIREBASE_TOOLS_STDOUT_FILE}"
    echo
  fi
  show_debug_log
}

retry() {
  local label="firebase-tools $1"
  local attempt status delay
  print_version
  for attempt in $(seq 1 "${attempts}"); do
    echo "${label}: attempt ${attempt} of ${attempts}"
    run_attempt "$@"
    status=$?
    if [ "${status}" -eq 0 ]; then
      # Evidence in every passing log that the kept name still works: a
      # release that changes how the CLI opens its log shows 0 here.
      echo "firebase-tools debug log kept for this attempt, line count: $(kept_count) (printed only when an attempt fails)"
      drop_debug_log
      echo "${label} passed on attempt ${attempt} of ${attempts}"
      return 0
    fi
    show_failure
    drop_debug_log
    if [ "${attempt}" -lt "${attempts}" ]; then
      delay=$((backoff * attempt))
      echo "::warning::${label} attempt ${attempt} of ${attempts} failed (exit ${status}); retrying in ${delay}s"
      sleep "${delay}"
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
