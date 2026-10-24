# The Hosting rewrite stays, and a Google-run caller on the direct path is an accepted residual

Status: accepted, 2026-10-24. Decided on [#1867](https://github.com/markgoho/doula-cloud/issues/1867).

[#1865](https://github.com/markgoho/doula-cloud/issues/1865) made `clientip.From` read `X-Forwarded-For` from the right and skip one entry when it is a Firebase Hosting edge, judged by address range. #1867 asked whether the reason to trust a range at all could be removed. It cannot be removed while the app reaches the BFF through the Hosting rewrite, and this records why, what is left, and what would change the answer.

**`doula-api`'s ingress stays `all`.** Cloud Run's ingress documentation (https://docs.cloud.google.com/run/docs/securing/ingress, read 2026-10-24) lists the sources that count as internal: Cloud Scheduler, Cloud Tasks, Dialogflow CX, Eventarc, Gemini Enterprise, Pub/Sub subscriptions, synthetic monitors, Workflows and BigQuery. Firebase Hosting is not one of them. The same page names Firebase Hosting among the services that call the default `run.app` URL, and says that turning that URL off "prevents these services or instance from working as expected". So `internal` and `internal-and-cloud-load-balancing` both refuse the rewrite, and so does turning off the default URL. Moving the Stripe and Mailgun webhooks to a service of their own would not change this: the rewrite alone needs `all`.

**Nothing Hosting adds can tell its requests from a direct caller's.** A Cloud Run rewrite in `firebase.json` takes a `serviceId`, a `region` and a `pinTag` and nothing else (https://firebase.google.com/docs/hosting/full-config, read 2026-10-24). The `headers` attribute on the same page sets response headers, not request headers, so no shared secret can ride the rewrite. Hosting's Cloud Run guide deploys the service to "allow unauthenticated invocations" (https://firebase.google.com/docs/hosting/cloud-run, read 2026-10-24) and documents no token that Hosting presents, and `doula-api` carries `allUsers` → `roles/run.invoker` for that reason, so there is no Google-signed token to check either, the way [ADR-0037](0037-the-internal-boundary-is-a-caller-identity-not-a-shared-secret.md) checks one for Cloud Scheduler. What is left is the address of the connection, which is what `hostingEdgeRanges` reads.

## The residual, and what it buys a caller

The ranges are Google's own egress, not Hosting's alone. Another Google-run service that connects from inside them, and that lets its user set a request header, can call the `run.app` URL with a made-up first `X-Forwarded-For` entry. `From` then skips that service's address as if it were Hosting and returns the made-up entry.

Whether such a service exists is not verified. Apps Script's `UrlFetchApp` was the example on #1867: its reference takes "a JavaScript key/value map of HTTP headers" and names no header it refuses (https://developers.google.com/apps-script/reference/url-fetch/url-fetch-app, read 2026-10-24), but whether it connects from inside the ranges, and whether a front end in its path replaces the header, is not documented. App Engine's URL Fetch lists `X-Forwarded-For` among the headers that "cannot be modified by the application" (https://docs.cloud.google.com/appengine/docs/legacy/standard/python/outbound-requests, read 2026-10-24). Cloud Scheduler, which does let any project set headers, connects from 34.116.39.x, 35.187.132.x, 107.178.194.x and similar addresses, none inside the ranges (doula-api's request log, read 2026-10-24).

If one does exist, a caller who uses it gets:

- a fresh `ratelimit.IPRule` bucket per request on the pre-account routes. Every other rule still holds: the JSON-field (email), path-value (Offer), bearer-token and session-cookie rules in `api/internal/ratelimit/rules.go` do not read `From`.
- a `signer_ip` or `terms_accepted` address of their own choice. Only the person signing or accepting can do that to their own record, since the request carries their own session or link.

## The decision

Keep the rewrite and the range check, and accept the residual. Make it visible instead: `.github/workflows/hosting-edge-check.yml` runs `api/cmd/hostingedge` every day against the last day of `doula-api`'s request log. It fails when an address from Google's own egress (Google's `goog.json` less `cloud.json`) is outside `hostingEdgeRanges`, which is either a Hosting edge the list is missing or the residual above in use, and when a range in the list stops being Google's own. A failed run opens a trunk-red issue. On its first read it found a Hosting edge the list was missing, 64.233.160.0/19, which had been connecting since at least 2026-09-27.

## Considered and rejected

- **An external Application Load Balancer in front of `doula-api`, with ingress `internal-and-cloud-load-balancing`.** This closes the residual: the load balancer is the only way in, it writes one known hop, and Cloud Armor can stand in front of it. It is not taken now, for three reasons. Hosting cannot rewrite to a load balancer, so either the app calls the API on a second origin (and the `__session` cookie and same-origin requests change with it) or the app itself moves off Firebase Hosting behind the same load balancer. It needs a domain and a managed certificate, and the `doula.cloud` cutover is the last step before launch. And a forwarding rule is billed by the hour whether or not anyone calls it, for a residual that is not shown to exist. If the daily check ever shows the residual in use, or the domain cutover is being planned anyway, this is the design to take.
- **A shared secret header from Hosting.** Not possible: the rewrite cannot set a request header.
- **A fixed one-hop skip.** Wrong on the direct path, which Stripe, Mailgun and Cloud Scheduler need: it would skip the real caller and return their made-up entry.
