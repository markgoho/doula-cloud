#!/usr/bin/env bash
# Forwards Stripe Sandbox events to the local BFF (see docs/environment.md).
# All three destinations are signed with this session's one secret, which is
# what STRIPE_WEBHOOK_SECRET, STRIPE_CONNECT_WEBHOOK_SECRET and
# STRIPE_ACCOUNT_WEBHOOK_SECRET all hold in app/.env.local. Deployed they
# are three different secrets, because they come from three separately
# created destinations.
#
# --forward-thin-connect-to is the Accounts v2 leg (#247): a v2 account
# emits thin events only, and `stripe listen` will not deliver them unless
# they are named in --thin-events.
exec stripe listen \
  --forward-to http://127.0.0.1:18080/api/stripe/webhook \
  --forward-connect-to http://127.0.0.1:18080/api/stripe/connect-webhook \
  --thin-events 'v2.core.account[configuration.merchant].capability_status_updated' \
  --forward-thin-connect-to http://127.0.0.1:18080/api/stripe/account-webhook
