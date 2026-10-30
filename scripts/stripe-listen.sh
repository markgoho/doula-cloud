#!/usr/bin/env bash
# Forwards Stripe Sandbox events to the local BFF (see docs/environment.md).
# All three destinations are signed with the CLI's one secret, which is
# what STRIPE_WEBHOOK_SECRET, STRIPE_CONNECT_WEBHOOK_SECRET and
# STRIPE_ACCOUNT_WEBHOOK_SECRET all hold in app/.env.local. Deployed they
# are three different secrets, because they come from three separately
# created destinations.
#
# The BFF this forwards to is the worktree's own (#2201): `dev:full`
# listens on 18080 + 100 x the offset in the nearest `.port-offset` above
# this script, the same walk and the same reading `app/e2e/ports.ts`
# makes. No file, as in the main checkout, is offset 0, and so is a file
# that does not hold a whole number.
#
# Two listeners, because the CLI (1.55) no longer lets one session send
# snapshot and thin events to different places: "Snapshot and thin events
# cannot share one destination", and `--forward-thin-to` is deprecated
# for `--forward-to` with `--events` naming the thin types; a
# `--forward-to` with no `--events`, `--all-snapshot` or `--all-thin` is
# refused outright. The first listener takes every snapshot event
# (`--all-snapshot`), the platform's to /webhook and the
# connected accounts' to /connect-webhook. The second takes the one
# Accounts v2 event the BFF handles (#247) to /account-webhook. A v2
# Account is an object the platform owns, so its events are emitted on the
# platform, the same reason the deployed event destination is
# `events_from: ["@self"]`; no `--events-from` is needed. Both listeners
# print the same secret: it belongs to the CLI's login, not the session.
#
# Read the log for `<--` lines, not `-->`: a `-->` with no `<--` means the
# event arrived and was never delivered.
#
# Environment:
#   STRIPE_BIN  the `stripe` to run; a test's stand-in (default `stripe`).
set -euo pipefail

stripe_bin="${STRIPE_BIN:-stripe}"

offset=0
directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
while :; do
  if [[ -f "$directory/.port-offset" ]]; then
    value="$(tr -d '[:space:]' < "$directory/.port-offset")"
    if [[ "$value" =~ ^[0-9]+$ ]]; then offset=$((10#$value)); fi
    break
  fi
  [[ "$directory" == / ]] && break
  directory="$(dirname "$directory")"
done

bff="http://127.0.0.1:$((18080 + 100 * offset))"
echo "Forwarding Stripe events to the BFF at $bff (port offset $offset)" >&2

"$stripe_bin" listen \
  --events 'v2.core.account[configuration.merchant].capability_status_updated' \
  --forward-to "$bff/api/stripe/account-webhook" &
thin=$!
trap 'kill "$thin" 2>/dev/null || true' EXIT INT TERM

"$stripe_bin" listen \
  --all-snapshot \
  --forward-to "$bff/api/stripe/webhook" \
  --forward-connect-to "$bff/api/stripe/connect-webhook"
