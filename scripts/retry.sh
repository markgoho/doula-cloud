#!/usr/bin/env bash
# Runs a command up to three times, for a step whose only failure is a
# download that can hit a 5xx (#2819), such as `terraform init` fetching a
# provider. It is not for a command whose failure is an answer.
#
# Usage: scripts/retry.sh <command> [args...]
#
# Environment, a test seam that no workflow sets:
#   RETRY_BACKOFF_SECONDS  the backoff unit; the wait after attempt N is N times this (default 20)
set -uo pipefail

[ "$#" -ge 1 ] || {
  echo "usage: scripts/retry.sh <command> [args...]" >&2
  exit 2
}
unit="${RETRY_BACKOFF_SECONDS:-20}"
attempts=3

for attempt in $(seq 1 "${attempts}"); do
  "$@"
  status=$?
  [ "${status}" -eq 0 ] && exit 0
  if [ "${attempt}" -eq "${attempts}" ]; then
    echo "::error::$1 failed after ${attempts} attempts"
    exit "${status}"
  fi
  delay=$((unit * attempt))
  echo "::warning::$1 attempt ${attempt} of ${attempts} failed (exit ${status}); retrying in ${delay}s"
  sleep "${delay}"
done
