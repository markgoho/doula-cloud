# gcp-dashboard

A personal, local-only tool that shows actual GCP spend and usage for the `doula-cloud` project, paired together so the developer does not need to navigate multiple GCP consoles. It is never deployed anywhere.

Unlike `app/` (a static SPA on `adapter-static`), this project runs on `@sveltejs/adapter-node` because its `+server.ts` routes call the BigQuery and Cloud Monitoring APIs server-side.

Styling here is plain and self-contained — it does not reuse or imitate `app/`'s design-system components.

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
