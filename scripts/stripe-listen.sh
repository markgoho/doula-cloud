#!/usr/bin/env bash
# Forwards Stripe test events to the local BFF (see docs/environment.md).
# Both destinations are signed with this session's one secret, which is
# what STRIPE_WEBHOOK_SECRET and STRIPE_CONNECT_WEBHOOK_SECRET both hold
# in app/.env.local. Run it beside `bun run dev:full`.
exec stripe listen \
  --forward-to http://127.0.0.1:18080/api/stripe/webhook \
  --forward-connect-to http://127.0.0.1:18080/api/stripe/connect-webhook
