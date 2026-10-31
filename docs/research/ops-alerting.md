# What Cloud Monitoring can alert on for webhook silence and refusals

Research for [Research: what Cloud Monitoring can alert on for webhook silence and refusals (#2412)](https://github.com/markgoho/doula-cloud/issues/2412), part of map [Wayfinder: the Platform operator app at ops.doula.cloud (#2409)](https://github.com/markgoho/doula-cloud/issues/2409). It feeds the alerting grilling, #2417, which makes the decision. This file records facts and labels its few recommendations as such.

Repo facts are cited by path and line on trunk at `cf27d249`. Outside facts are cited to Google's and Stripe's own documentation, read on 2026-10-31. Nothing here was read from the live GCP project; where a claim rests on a page that did not say something, the file says so rather than filling the gap.

## Summary

- **No log line marks a successful webhook today.** The BFF logs only refusals and drops, with the stdlib `log` package as plain text. A success leaves a `stripe_webhook_events` row and a Cloud Run request-log entry, nothing else.
- **Silence per webhook route** can be caught without code changes: a user-defined log-based counter on the Cloud Run request log, filtered by path and 2xx, plus a metric-absence condition. **Silence per event type** cannot; the request log does not carry the event type, so it needs a new structured log line or a database query.
- **Metric-absence windows max out at 23.5 hours** (the quotas page says one day). At pilot volume, a single Stripe event type can go longer than that without anything being wrong, so a Cloud Monitoring absence alert either fires falsely or has to watch something busier than one event type.
- **Counting refusals** needs a log-based *metric* with a threshold condition. A log-match (log-based) alert cannot count, and is capped at 20 notifications a day.
- **Cost is near zero either way.** Alerting is free until no sooner than 2027-09-01, then $0.35 per condition per month. A log-based counter costs 8 bytes a point against a 150 MiB monthly free allotment. A Cloud Scheduler job costs $0.10 per 31 days past three free per billing account, and the repo already has two.
- **#1403 is not built.** No uptime check, alert policy, notification channel or monitoring resource exists in `terraform/` today.

## 1. What the BFF logs today

### Logging setup

- The BFF uses Go's stdlib `log` package (`log.Printf`) throughout; no `slog`, no JSON lines, no logging client. `terraform/iam.tf:199-201`: "Cloud Run collects a container's stdout/stderr and its request logs at the platform level, not as the runtime identity, and `api/` constructs no logging or monitoring client."
- Go's `log` writes to stderr by default. Cloud Run puts a plain-text line in `textPayload` and parses a single-line JSON object into `jsonPayload`, lifting `severity` and `message` out of it ([Cloud Run logging](https://docs.cloud.google.com/run/docs/logging); [Structured logging](https://docs.cloud.google.com/logging/docs/structured-logging)). **Unverified:** what severity a plain-text stderr line gets on Cloud Run. Neither page states it, so a filter on `severity>=ERROR` should not be assumed to catch these lines; a filter on `textPayload` text will.
- `apierr.WriteError` writes the response and logs nothing (`api/internal/apierr/apierr.go:342-344`). Every 4xx or 5xx a handler returns through it is visible only in the Cloud Run request log.
- Cloud Run writes a request log for every request, log name `run.googleapis.com%2Frequests` ([Cloud Run logging](https://docs.cloud.google.com/run/docs/logging)). Its `httpRequest` field is the standard `LogEntry.HttpRequest`, which carries `requestUrl` and `status` ([LogEntry reference](https://docs.cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest)).
- The built-in metric `run.googleapis.com/request_count` has labels `response_code` and `response_code_class`, and `route` is "Always empty" ([GCP metrics, P–Z](https://docs.cloud.google.com/monitoring/api/metrics_gcp_p_z)). So the built-in metric cannot tell the webhook path from any other path; a per-path signal has to come from the request log through a log-based metric.

### Webhook routes

Four webhook routes, all registered in `api/routes_webhook.go:17-26`:

| Route | Handler | Carries |
| --- | --- | --- |
| `POST /api/stripe/webhook` | `billing.PostPurchaseWebhookHandler` | Credit purchases (Checkout) |
| `POST /api/stripe/connect-webhook` | `payments.PostConnectWebhookHandler` | Invoice and payment snapshot events from connected accounts |
| `POST /api/stripe/account-webhook` | `payments.PostAccountWebhookHandler` | v2 account thin events (capability status) |
| `POST /api/mailgun/webhook` | `portalinvite.PostBounceWebhookHandler` | Mailgun bounces and complaints |

### What each outcome leaves behind

| Outcome | App log line | Request log | Database |
| --- | --- | --- | --- |
| **Bad signature** | None. `billing/webhook.go:44-47`, `payments/webhook.go:128-131`, `:729-732`, `portalinvite/bounce_webhook.go:96` return 400 through `apierr.WriteError`. | 400 | Nothing |
| **Mode mismatch** (live event, test keys or the reverse) | `billing: purchase webhook: refusing event %s: livemode %v, keys are in %s mode` (`billing/webhook.go:50`); same shape for `payments: connect webhook:` (`payments/webhook.go:134`) and `payments: account webhook:` (`:735`) | 400 | Nothing |
| **Unhandled event type** | `payments: connect webhook: dropping unhandled event type %q (id %s)` (`payments/webhook.go:153`), and the account webhook's at `:744`. The purchase webhook acknowledges other types silently (`billing/webhook.go:60-63`). | 200 | Nothing |
| **Event for an unknown invoice or account** | `dropping invoice.paid for unresolved invoice …` and siblings (`payments/webhook.go:336`, `:460`, `:548`, `:653`, `:810`) | 200 | Nothing |
| **Handled successfully** | None | 200 | A `stripe_webhook_events` row: `event_id`, `processed_at` only, no type and no route (`api/db/migrations/00022_stripe_purchase_webhook.sql:11-14`) |
| **Internal error** (database, Stripe call) | Mostly none; `apierr.WriteError` with 500 | 500 | Rolled back |

Two consequences:

1. **A signature failure is invisible except in the request log.** A rotated or wrong signing secret makes every delivery 400 with no app log line. A log-based metric on the request log filtered to the webhook path and `httpRequest.status=400` catches it; a filter on app log text does not.
2. **Silence is measurable per route from the request log, not per event type.** Nothing logs the type of a handled event, and `stripe_webhook_events` stores no type. Per-type silence needs either a new log line at the point of success (a JSON line with `event_type` lets a log-based metric extract it as a label; 10 labels per metric, 30,000 active series per metric, per [Logging quotas](https://docs.cloud.google.com/logging/quotas)) or a `type` column on `stripe_webhook_events` for an app-side query.

### Outbox worker

- One Cloud Scheduler job, `process-outbox-drain`, calls `POST /api/internal/outboxes/drain` every five minutes with `retry_count = 0` (`terraform/scheduler.tf:21-56`). A second job, `verify-practice-pages`, runs every fifteen minutes (`terraform/scheduler.tf:58-88`).
- The drain runs every registered outbox (22 registrations in `api/outboxes.go`), each in its own transaction. A failing outbox logs `outbox: drain %s: %v` with the outbox's path (`api/internal/outbox/drain.go:111`) and the response is 500 naming the failed paths (`drain.go:114-117`); otherwise 200. Route registered at `api/internal/outbox/registry.go:86`.
- **Dead-lettering logs nothing.** A row that exhausts its retries, or is dead-lettered on the spot for a suppressed address, gets `status = 'dead_lettered'` and `last_error` on the row (`api/internal/outbox/outbox.go:98-135`). The only place it is visible is the outbox table.
- So the outbox has two signals available today: the drain's 500 in the request log (and its log line), and dead-lettered rows in the database. The first suits a log-based metric; the second needs a query.
- **Unverified:** whether Cloud Scheduler exposes a built-in Monitoring metric for job attempts. The GCP metrics list for services starting with C did not show a `cloudscheduler.googleapis.com` metric when searched, and the Scheduler logging page fetched returned 404. The drain's own 500 in the Cloud Run request log does not depend on this.

### Error rates

The built-in `run.googleapis.com/request_count` with `response_code_class` gives a service-wide 5xx count with no setup, but not per path (`route` is always empty, above). A per-path error rate needs a log-based counter on the request log.

### Monitoring resources in Terraform today

None. `terraform/` holds `cloud_run.tf`, `cloud_sql.tf`, `iam.tf`, `provider.tf`, `scheduler.tf`, `secrets.tf`, `storage.tf`, `versions.tf`; a search for `monitoring`, `logging_metric` and `alert` finds only the IAM comment above. #1403's uptime checks, alert policies and phone channel are not built (#1403 is open, every box unticked).

## 2. Cloud Monitoring: absence and counts

### Log-based metrics

- A user-defined counter "count[s] the number of log entries that match a specified filter"; label values are extracted from fields of matching entries, one time series per label combination ([Log-based metrics](https://docs.cloud.google.com/logging/docs/logs-based-metrics)).
- **Gaps, not zeros.** "A value of zero is written to the time series when an adjacent interval has a non-zero value." Otherwise "the time series isn't updated and there is a data gap for the interval" (same page). This is what a metric-absence condition detects.
- Data comes "only from log entries received after the metric is created"; no backfill (same page).
- Limits: 500 user-defined metric descriptors per project, 10 labels per metric, 30,000 active time series per metric ([Logging quotas](https://docs.cloud.google.com/logging/quotas)).

### Metric-absence conditions

- Fires "when a monitored time series has no data for a period of time that you specify", up to "a maximum of 23.5 hours" ([Metric-absence policies](https://docs.cloud.google.com/monitoring/alerts/metric-absence)). The quotas page states the limit as one day ([Monitoring quotas](https://docs.cloud.google.com/monitoring/quotas)).
- **Arms only after data arrives.** The condition is met only once the series has written a point after the policy was created or modified (same page). A policy edited during a quiet spell stays silent until the next matching log line. For a low-volume webhook route this is a real gap: change the policy, and it watches nothing until Stripe next sends.
- Absence is checked in 15-minute intervals; with the default retest window, "there must be no data for 20 minutes before the condition is met" ([Alerting in depth](https://docs.cloud.google.com/monitoring/alerts/concepts-indepth)).
- For a log-based metric the page recommends a rolling window of at least 10 minutes.

### Threshold conditions on a count

- A refusal count over N in a window is a metric-threshold condition on a log-based counter. Up to six conditions per metric policy ([Monitoring quotas](https://docs.cloud.google.com/monitoring/quotas)).
- `evaluationMissingData` decides what a gap does: `ACTIVE` lets a gap open an alert, `INACTIVE` closes open alerts, unspecified holds them ([Alerting in depth](https://docs.cloud.google.com/monitoring/alerts/concepts-indepth)). A refusal counter that is usually empty wants this set deliberately.

### Log-match (log-based) alerts

- An alert straight off a log filter, no metric. "You can't use log-based alerting policies to derive counts from your logs" ([Log-based alerts](https://docs.cloud.google.com/logging/docs/alerting/log-based-alerts)).
- Rate limits: 20 new alerts a day, 20 notifications a day, 2 new alerts a minute, at least 5 minutes between notifications for an open alert ([Monitoring quotas](https://docs.cloud.google.com/monitoring/quotas)).
- Suits a rare, always-bad line ("refusing event … livemode") where any one occurrence matters. Does not suit a count or an absence.

### Timing

Monitoring may take "up to 180 seconds after the alerting policy condition evaluates to true" to create an alert, and computations "take an additional delay of up to 5 minutes and 30 seconds" ([Alerting in depth](https://docs.cloud.google.com/monitoring/alerts/concepts-indepth)). Log-based counters update every minute per stream ([Log-based metrics](https://docs.cloud.google.com/logging/docs/logs-based-metrics)).

### Limits that bear

2,000 alert policies per metrics scope, 16 notification channels per policy ([Monitoring quotas](https://docs.cloud.google.com/monitoring/quotas)). None of these is near what ops would need.

## 3. Price

All from [Google Cloud Observability pricing](https://cloud.google.com/stackdriver/pricing), read 2026-10-31:

| Item | Price | Free |
| --- | --- | --- |
| Alerting policies | $0.35 a month per metric reference (each condition counts as one; a log-match policy has one condition), plus $0.50 per million points returned | Not charged before **September 1, 2027** at the earliest; 90- and 30-day notices first. Policies on uptime, billing or quota metrics are never charged. |
| User-defined log-based metric | Billed as custom metric ingestion; a counter is 8 bytes per point, $0.2580/MiB past the allotment | First 150 MiB per billing account per month |
| Logs ingestion | $0.50/GiB | First 50 GiB per project per month |
| Uptime-check executions | $0.30 per 1,000 | 1 million per project per month |
| Cloud Scheduler job ([pricing](https://cloud.google.com/scheduler/pricing)) | $0.10 per job per 31 days; a paused job still counts | 3 jobs per billing account |

Worked figures, not quotes:

- A counter per webhook route writes at most one point a minute: about 43,200 points, about 0.33 MiB a month. Four such counters use under 1% of the free 150 MiB.
- After September 2027, five conditions (say four route-silence conditions and one refusal threshold) are $1.75 a month, plus points returned. #1403's two uptime policies cost nothing.
- #1403's uptime checks: two checks, every minute, from at least three checkers ([Uptime checks](https://docs.cloud.google.com/monitoring/uptime-checks)) is about 259,000 executions a month, inside the free million.
- Scheduler: two jobs today. One more is free. #1403's enable/disable pair makes four, one billed: $0.10 per 31 days.

## 4. Getting it to the phone, waking hours only

### Channels

From [Notification options](https://docs.cloud.google.com/monitoring/support/notification-options) and [Monitoring quotas](https://docs.cloud.google.com/monitoring/quotas):

- **Mobile app** (Google Cloud app). Set up by installing the app and choosing the project; the channel appears under Mobile Devices after a few minutes. It goes to one device or all the user's devices, and "you can't select or change the scope." The page groups it with PagerDuty, webhooks and Slack as sharing "a single point of failure" and advises pairing with email or Pub/Sub.
- **SMS.** "SMS isn't a fully reliable notification channel type, and it might not be available in certain regions." 2,500 messages per rolling 24 hours per project, 200 per phone number. Verification needs a code sent to the phone, which only the founder can enter (as #1403's comment says).
- **Email.** One address per channel.
- Pricing pages list no charge for any channel.

### Waking hours

Cloud Monitoring has no schedule on a policy or a channel. #1403 names two ways; the docs add a third.

1. **Disable at night, enable in the morning** (#1403's first way, a Cloud Scheduler pair). "Monitoring doesn't … create alerts or send notifications for disabled alerting policies," but it keeps evaluating; alerts already open "remain open until the policy's auto-close duration expires"; and on re-enable it reads the latest retest window, which "might include data taken before, during, and after the policy was enabled" ([Alerting in depth](https://docs.cloud.google.com/monitoring/alerts/concepts-indepth)). [Manage policies](https://docs.cloud.google.com/monitoring/alerts/manage-alerts) says "A re-enabled policy might trigger immediately." That is what makes a night failure show at the start of the day. For an absence condition, though, re-enabling counts as a modification only if the API treats an enable as an edit; the docs read do not say, so it is **unverified** whether a re-enabled absence policy needs a fresh data point before it can fire again. A Scheduler job can call the Monitoring API directly; that it can do so with an OAuth token for `alertPolicies.patch` is the standard Scheduler HTTP-target pattern, not something read on a page today.
2. **The phone silences the app overnight** (#1403's second way). The policy is always on; the phone's own do-not-disturb setting hides the push. Nothing in the repo. Alerts still open and show in the app in the morning.
3. **A snooze.** Snoozes stop alerts and notifications for up to 16 policies over a set window, with label filters. There is no recurring snooze: "To create snoozes that happen over recurring intervals, you can write a script that creates a snooze," run from cron or Cloud Scheduler. Expired snoozes are kept 13 months and cannot be deleted ([Manage snoozes](https://docs.cloud.google.com/monitoring/alerts/manage-snooze)). The page does not say whether a condition that held during a snooze notifies after the snooze ends, and does not cover Terraform. A daily snooze is one Scheduler job (creating tomorrow night's snooze) rather than two.

## 5. Stripe's side of silence

From [Receive Stripe events](https://docs.stripe.com/webhooks) and [Manage webhook endpoints](https://docs.stripe.com/events/manage-webhook-endpoints):

- In live mode Stripe retries a failed delivery "for up to three days with an exponential back off"; in a sandbox, "three times over the course of a few hours."
- Workbench's Event deliveries tab lists each event as Delivered, Pending or Failed, with the status code.
- `GET /v1/events?delivery_success=false` returns events that failed delivery to at least one endpoint, for events created in the last 30 days.
- Neither page read says whether Stripe emails the account or disables an endpoint after sustained failure. Not found; not assumed.

This matters to the silence question: Stripe knows what it sent. A check that asks Stripe "anything failed delivery?" distinguishes "nothing happened" from "something happened and we did not get it", which no count of our own arrivals can do.

## 6. The two routes, side by side

### Cloud Monitoring

What it takes, per signal:

| Signal | Mechanism | Code change |
| --- | --- | --- |
| No delivery on a webhook route for N hours (N ≤ 23.5) | Log-based counter on `run.googleapis.com/requests`, path = route, 2xx; metric-absence condition | None |
| Signature failures | Same counter shape, status 400 on the webhook paths; threshold condition | None |
| Mode-mismatch refusal | Log-match alert on `textPayload:"refusing event"`, or a counter | None |
| No event of a given type for N hours | Counter with an `event_type` label | A JSON log line on success |
| Outbox drain failing | Counter on the drain path with status 500, or a log-match on `outbox: drain` | None |
| Dead-lettered outbox rows | Not visible to Logging | A log line on dead-letter |
| Service-wide 5xx | Built-in `request_count`, `response_code_class=5xx` | None |

Strengths: lives in `terraform/` beside #1403's uptime checks and uses the same channel; no app code for most signals; runs even when the BFF is down (an absence alert fires precisely because nothing is logged).

Limits: 23.5-hour cap on absence; absence arms only after the next data point following any edit; per-type and dead-letter signals need code anyway; log-match alerts cannot count and stop at 20 a day.

### App-side check

A Cloud Scheduler job calls a new internal route (OIDC as `internal-caller@`, like the two existing jobs, `terraform/scheduler.tf:6-10`). The route queries the database (and optionally Stripe) and answers 500 when something is wrong. Something then has to turn the 500 into a phone notification, and that is still Cloud Monitoring: a log-match or counter on the route's 500 in the request log. An uptime check cannot simply call it instead, because uptime checks authenticate with Basic auth or a Monitoring service-agent identity token, not `internal-caller@`, and the route's guard would need to accept that identity ([Uptime checks](https://docs.cloud.google.com/monitoring/uptime-checks)).

Strengths: any window, not capped at 23.5 hours; can read things Logging never sees (dead-lettered rows, `stripe_webhook_events.processed_at`, Credits); can ask Stripe for `delivery_success=false`; the same query could feed the ops app's System section (#2409's notes, #1996).

Costs: one Scheduler job ($0.10 per 31 days if it is the fourth; free if the third); an internal route with tests to 100% coverage; a `type` column on `stripe_webhook_events` if silence is per type; and it fires nothing if the BFF or the database is down, which #1403's uptime check covers separately.

### Recommendations (labeled; #2417 decides)

- **Recommendation:** use Cloud Monitoring for what Logging already sees with no code (signature 400s, mode refusals, drain 500s, service 5xx), and do not put a per-event-type absence alert in Cloud Monitoring. At pilot volume one event type can legitimately be quiet for days, past the 23.5-hour cap.
- **Recommendation:** for "is Stripe reaching us", ask Stripe (`delivery_success=false`) from an app-side check rather than infer it from our own silence. It separates a quiet week from a broken endpoint.
- **Recommendation:** whatever is chosen, add a log line on dead-letter, since today that state is visible only in the table.

## Open questions for #2417

- Which signals alert at all, and which only show on the ops app's System section.
- Waking-hours mechanism and hours (shared with #1403).
- Unverified above: plain-text stderr severity on Cloud Run; whether a re-enabled absence policy needs a fresh point to arm; whether Cloud Scheduler has a built-in attempt metric; whether Stripe emails or disables a failing endpoint.
