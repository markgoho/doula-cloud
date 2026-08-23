#!/usr/bin/env bash
# Forwards Stripe Sandbox events to the local BFF (see docs/environment.md).
# All three destinations are signed with this session's one secret, which is
# what STRIPE_WEBHOOK_SECRET, STRIPE_CONNECT_WEBHOOK_SECRET and
# STRIPE_ACCOUNT_WEBHOOK_SECRET all hold in app/.env.local. Deployed they
# are three different secrets, because they come from three separately
# created destinations.
#
# The Accounts v2 leg (#247) needs BOTH --thin-events (to subscribe) and
# --forward-thin-to (to deliver). Note --forward-thin-to, *not*
# --forward-thin-connect-to: a v2 Account is an object the platform owns,
# so its events are emitted on the platform, the same reason the deployed
# event destination is `events_from: ["@self"]`.
#
# Getting this wrong is silent in the worst way. `stripe listen` still
# prints a `-->` line for each thin event it receives, so the log looks
# healthy -- but with no matching --forward-thin-to there is no `<--` line
# and nothing is ever delivered. Read the log for `<--`, not `-->`.
exec stripe listen \
  --forward-to http://127.0.0.1:18080/api/stripe/webhook \
  --forward-connect-to http://127.0.0.1:18080/api/stripe/connect-webhook \
  --thin-events 'v2.core.account[configuration.merchant].capability_status_updated' \
  --forward-thin-to http://127.0.0.1:18080/api/stripe/account-webhook
