# gcp-dashboard

A personal, local-only tool that shows actual GCP spend and usage for the `doula-cloud` project, paired together so the developer does not need to navigate multiple GCP consoles. It is never deployed anywhere.

Unlike `app/` (a static SPA on `adapter-static`), this project runs on `@sveltejs/adapter-node` because its `+server.ts` routes call the BigQuery and Cloud Monitoring APIs server-side.

Styling here is plain and self-contained — it does not reuse or imitate `app/`'s design-system components.

## What it shows

The one screen has a sidebar and a set of panels. The sidebar carries total spend for the current billing period and the sync action; the main area lists every service the billing export returned for the period, largest first, each expanding to its SKUs, and then pairs the Cloud Run, Cloud SQL, Cloud Storage, Firestore and Firebase Hosting bills with the usage that produced them. The panels lay themselves out two-up wherever the column can hold two and stack where it cannot, which is the column's own width talking rather than the viewport's.

`GET /api/cost` queries the `doula-cloud:billing_export` BigQuery dataset (`us-central1`), grouped by `service.description` and `sku.description` and filtered to this project. No service is named in the query, so a service enabled tomorrow appears with no code change. Three of them — Artifact Registry, Secret Manager and Identity Platform (which is how Firebase Authentication bills) — publish no Cloud Monitoring metric that can be paired with the bill, so they carry an inline "usage detail not available" note where a usage panel would go.

`GET /api/usage` is the other half: it asks Cloud Monitoring's `listTimeSeries` for four Cloud Run metrics — billable instance time, CPU allocation time, memory allocation time, and request count — scoped to the `doula-api` service, each collapsed to a single figure for the same billing period the cost query groups by. All four are DELTA counters, so all four are summed; the aligner is derived from the metric kind rather than written per metric, so a GAUGE added later cannot be summed by accident. Unlike cost, these figures are current to the second the sync ran, and the panel says so.

The same route pulls the one Cloud SQL figure the bill moves with: `database/disk/quota`, the provisioned disk, scoped to the `doula-cloud-pg` instance — which `cloudsql_database` identifies as `database_id="doula-cloud:doula-cloud-pg"`, since the resource carries no `instance_id` label. It is a GAUGE of bytes, and it is read with `ALIGN_MAX` rather than the mean its kind defaults to: a quota is a step function that only rises, so the mean would understate the size being paid for if the disk grew mid-period. A GAUGE may ask for that peak; a DELTA counter may not, because the maximum of a counter's samples reports the busiest slice of the period rather than the period. Cloud SQL CPU and memory utilization are deliberately absent — compute bills flat per instance-tier-hour, so a utilization percentage is a sizing signal and showing it in a cost panel would imply the bill moves with it.

The same route pulls three more services. Cloud Storage reports what is held and what leaves — `storage/total_bytes` and `network/sent_bytes_count`, over every bucket in the project, since every bucket produces the bill. The stored figure is a GAUGE averaged within each bucket, as the GAUGE default is, but added across them rather than averaged: read live, the three buckets hold 567,149 bytes between them and the default cross-series mean would have reported 189,050, which is one average bucket and not what is billed. Egress is the DELTA counter `network/sent_bytes_count`; its counterpart `received_bytes_count` is ingress, which is not charged, so it is absent.

Firestore is the three document counters Firestore charges per operation — `document/read_count`, `write_count` and `delete_count`, all DELTA. This project keeps its data in Postgres, so they usually report no series at all; the panel then shows a dash, which means "Cloud Monitoring reported nothing", not "zero".

Firebase Hosting is `network/sent_bytes_count`, the bytes served, over every domain in the project — a DELTA counter summed within each domain and across them, exactly as Cloud Storage's egress is. Hosting also publishes `network/monthly_sent`, a GAUGE holding a month-to-date total, and the panel deliberately does not read it: that total's month is a Pacific one. Traced live, `dou.la` read 137,092,104 at 07:07:59Z on the first of the month and 17,720 by 07:16:59Z — a reset at midnight `America/Los_Angeles`, the same instant the dashboard's own billing period now opens. Any read of the GAUGE inside that window answers with the previous month's total, because that is the only sample there is. The DELTA has no such edge at any instant.

The billing period itself is the calendar month in `America/Los_Angeles`, not UTC — that is the zone GCP assigns `invoice.month` in, verified against the project's own billing export. Both `/api/cost` and `/api/usage` derive their window from the same conversion, so a usage figure and a cost figure always cover the same month.

Cloud Monitoring rejects an alignment period under a minute, so `/api/usage` cannot ask for one in the first minute of a billing period — the shortest period it could ask for would reach back into the previous one. Rather than send that request and answer wrong, every usage figure is absent for that first minute, the same dash Firestore already shows when Monitoring has nothing to report.

Sync is stateless: one read pulls both routes at once and replaces what is on screen. Either half failing fails the sync, and each names its own upstream, so the banner says which half broke. The page reads once as it mounts, so opening it shows figures rather than an empty screen; every read after that is a press of the button. Nothing polls, nothing refreshes on a timer, and no history is kept. The billing export is written about once a day, so the total is always about 24 hours behind actual usage — the sidebar says so, and never implies a same-day figure.

Future work, deliberately not built: a history or trend view over past syncs.

## Local setup

1. `bun install`
2. Authenticate to Google Cloud with Application Default Credentials — this is the **only** auth step required, no service account or key file:

   ```sh
   gcloud auth application-default login
   ```

   On macOS this writes `~/.config/gcloud/application_default_credentials.json`, which the GCP client libraries this project uses discover automatically.
3. `bun run dev` to start the dev server, or `bun run dev -- --open` to also open it in a browser tab.

## Building

```sh
bun run build
```

Preview the production build with `bun run preview`.

## Testing

```sh
bun run check            # typecheck
bun run lint             # eslint
bun run test:unit:coverage   # unit tests with the 100%-line-coverage gate
```

CI (`.github/workflows/gcp-dashboard-ci.yml`) runs all three on every change under `gcp-dashboard/**`. This job never deploys — the tool has no production target.

`test:unit:coverage`'s 100%-line-coverage gate is scoped to `src/lib/**` only (see `vite.config.ts`'s `coverage.include`) — the same convention `docs/testing.md`'s Coverage section documents for `app/`. `src/routes/**` (the dashboard page and its `+server.ts` routes) is exercised by real specs in the `client` (a rendered `dashboard-page.svelte.spec.ts`, browser-mode via vitest-browser-svelte) and `server` (`cost-route.spec.ts`) projects, it just isn't folded into the 100% requirement — a deliberate exclusion, not an oversight.
