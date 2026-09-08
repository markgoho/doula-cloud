# gcp-dashboard

A personal, local-only tool that shows actual GCP spend and usage for the `doula-cloud` project, paired together so the developer does not need to navigate multiple GCP consoles. It is never deployed anywhere.

Unlike `app/` (a static SPA on `adapter-static`), this project runs on `@sveltejs/adapter-node` because its `+server.ts` routes call the BigQuery and Cloud Monitoring APIs server-side.

Styling here is plain and self-contained — it does not reuse or imitate `app/`'s design-system components.

## What it shows

The one screen has a sidebar and a cost breakdown. The sidebar carries total spend for the current billing period and the sync action; the main area lists every service the billing export returned for the period, largest first, each expanding to its SKUs.

`GET /api/cost` is the whole data path: it queries the `doula-cloud:billing_export` BigQuery dataset (`us-central1`), grouped by `service.description` and `sku.description` and filtered to this project. No service is named in the query, so a service enabled tomorrow appears with no code change. Three of them — Artifact Registry, Secret Manager and Identity Platform (which is how Firebase Authentication bills) — publish no Cloud Monitoring metric that can be paired with the bill, so they carry an inline "usage detail not available" note where a usage panel would go.

Sync is stateless: each press is a fresh server-side read that replaces what is on screen. Nothing polls, nothing auto-refreshes, and no history is kept. The billing export is written about once a day, so the total is always about 24 hours behind actual usage — the sidebar says so, and never implies a same-day figure.

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
