# What a second app and backend service beside `doula-api` takes

Research for [Research: what a second app and backend service beside doula-api takes (#2411)](https://github.com/markgoho/doula-cloud/issues/2411), part of [Wayfinder: the Platform operator app at ops.doula.cloud (#2409)](https://github.com/markgoho/doula-cloud/issues/2409). It gathers facts for the service-shape grilling, [#2415](https://github.com/markgoho/doula-cloud/issues/2415). It decides nothing. Where a line says **Recommendation**, it is labeled as one and is the researcher's, not a decision.

Repo facts are cited by path, read on trunk at `cf27d249` (2026-10-31). Outside facts are cited to Google's, HashiCorp's and PostgreSQL's own documentation, read 2026-10-31.

## Summary

- **Hosting.** A third Firebase Hosting site in the same project, with `ops.doula.cloud` connected and a `/api/**` rewrite to a second Cloud Run service, is supported and has no listed charge. It is a `firebase.json` entry, a `.firebaserc` target, a site created once with the CLI, and a TXT plus A records at the DNS host. Terraform can only manage the site and the domain through the `google-beta` provider; the repo pins `hashicorp/google` alone and keeps Hosting by hand on purpose.
- **Second Cloud Run service.** Supported by the provider the repo already uses (`google_cloud_run_v2_service`). With `min-instances=0` an operator's traffic sits inside the free tier and pays a cold start; with `min-instances=1` at `doula-api`'s size, the idle cost works out to about $10 a month (arithmetic below).
- **Database role.** The repo already has the pattern twice: a `NOLOGIN` group role made in a migration (`app_runtime` in 00002, `site_builder` in 00046), a login granted that role and made by hand, and either `TO role USING (true)` policies (00046) or `SECURITY DEFINER` functions with `REVOKE ... FROM PUBLIC` then `GRANT ... TO role` (00010 and nine others). The thing that keeps an operator function away from `app_runtime` is that `REVOKE`, written every time; nothing in the repo checks it today.
- **IAM database auth** is off on the live instance. Turning it on removes the password (and with it the reason `docs/infrastructure.md` keeps logins out of Terraform), but the documented way to use it from Go is the `cloudsqlconn` connector, not the `/cloudsql` socket `doula-api` dials today.
- **Go shape.** A second binary in `api/` (`api/cmd/ops`) lands under the same 100% coverage gate with no CI change to the gate, but `Deps`, the route registration and the outbox list are `package main` at `api/` root and cannot be imported by a second `main`. Routes on `doula-api` behind a separate host cost nothing in CI but run as `app_runtime`, which defeats the reason #2409 gives for a separate app.

## 1. Today's wiring, from the repo

### Hosting

`firebase.json` declares two Hosting targets:

- `site` → `site/build`, no rewrites.
- `app` → `app/build`, with two rewrites: `/api/**` to Cloud Run `{ "serviceId": "doula-api", "region": "us-central1" }`, then `**` to `/200.html`.

`.firebaserc` maps target `site` to the Hosting site `doula-cloud` and `app` to `doula-cloud-app`. The app is still served from `https://doula-cloud-app.web.app`, which is the value of `EXPECTED_ORIGINS` and `APP_BASE_URL` on Cloud Run (`docs/environment.md`, the `EXPECTED_ORIGINS` and `APP_BASE_URL` rows). No custom domain is on the app site yet.

Hosting is not in Terraform. `docs/infrastructure.md`, "What stays by hand": "`firebase.json` and `.firebaserc` are already code and already deploy from CI ... Adding a second owner would produce drift, not detect it."

### How `app/` and `site/` deploy

- `app/`: `ci.yml` job `deploy-app` runs on every trunk push, after `api`, `app` and `migrate` pass, and runs `scripts/firebase-tools.sh retry deploy --only hosting:app`. Pull requests get a preview channel from `.github/workflows/firebase-hosting-app-pull-request.yml` (`hosting:channel:deploy "pr-N" --only app`).
- `site/`: its own workflow, `.github/workflows/firebase-hosting-merge.yml`, path-filtered to `site/**` and a few shared files, plus a `repository_dispatch` and a daily schedule. It deploys `--only hosting:site`. Previews come from `.github/workflows/firebase-hosting-pull-request.yml`.
- `api/`: `ci.yml` job `api-image` builds `api/Dockerfile` (`go build -o /app .`, one binary from the module root) and boot-tests it against a throwaway Postgres; `deploy-api` deploys it with `google-github-actions/deploy-cloudrun@v3` to service `doula-api`, `secrets: DATABASE_URL=doula-cloud-pg-app-runtime-dsn:latest`, and flags `--min-instances=1 --add-cloudsql-instances=doula-cloud:us-central1:doula-cloud-pg --service-account=doula-api-runtime@...`. `migrate` runs goose before either deploy.

Each preview workflow deploys one target, so a third site needs a third preview workflow or goes without previews.

### Cloud Run `doula-api`

`terraform/cloud_run.tf`: `ingress = "INGRESS_TRAFFIC_ALL"`, 1 vCPU / 512Mi, `cpu_idle = true` (request-based billing), `startup_cpu_boost = true`, template `min_instance_count = 1`, `max_instance_count = 20`, concurrency 80, the `/cloudsql` volume, and runtime identity `doula-api-runtime@`. `lifecycle.ignore_changes` leaves the image and the `commit-sha` label to `deploy-api` (ADR-0034's split). `allUsers` holds `roles/run.invoker` (`google_cloud_run_v2_service_iam_member.doula_api_allusers_invoker`), because the Hosting rewrite forwards an anonymous request.

[ADR-0056](../adr/0056-the-hosting-rewrite-stays-and-a-google-run-caller-on-the-direct-path-is-an-accepted-residual.md) keeps that shape: ingress must stay `all` for the rewrite to reach the service, nothing Hosting adds can tell its requests from a direct caller's, and the client address is read from `X-Forwarded-For` by Hosting edge range (`api/internal/clientip`). A second service behind its own rewrite inherits the same three facts: `allUsers` invoker, ingress `all`, and no Google-signed token from Hosting to check.

[ADR-0037](../adr/0037-the-internal-boundary-is-a-caller-identity-not-a-shared-secret.md) already weighed "two Cloud Run services from the same image, one public and one `run.invoker` for named service accounts only" and rejected it because it "doubles the deployment surface — two services, two revisions per merge, two sets of environment variables and secret references to keep in step, a second Terraform resource with the same image-ownership conflict ADR-0034 had to solve once — to buy what one token check buys, on a boundary with three callers." That rejection was about separating callers by invoker IAM. #2409's reason is a different one, a database role the Practice BFF does not hold. Whether that difference changes the answer is #2415's call; the grilling should see the earlier rejection.

### Operator endpoints that already exist

`api/routes_internal.go` registers the operator acts on `doula-api`, under `/api/internal/**`, behind `internalauth.Guard` (ADR-0037, a Google-signed OIDC token checked against `INTERNAL_OIDC_CALLERS`):

- `POST /api/internal/billing/refunds`
- `GET /api/internal/billing/dormant-practices`
- `GET /api/internal/billing/monthly-credits`
- `POST /api/internal/billing/founding-grants`
- `POST /api/internal/staffauth/mfa-recovery/support-clear`
- `POST /api/internal/staffauth/pilot-allowlist`, `POST /api/internal/staffauth/pilot-gate/open`
- `POST /api/internal/legal/publish`

All of them run as `app_runtime`. The founder's feedback read page is the one operator read in the app itself, gated by `FOUNDER_STAFF_ID` (`api/main.go`, `staffauth.FounderOnly`) and migration 00122's `feedback_reader()` session variable.

### Cloud SQL, roles and RLS

`terraform/cloud_sql.tf`: one instance, `doula-cloud-pg`, Postgres 16, `db-f1-micro`, Enterprise edition, zonal, public IP with `TRUSTED_CLIENT_CERTIFICATE_REQUIRED`. No `database_flags` block, and the live instance carries no database flags (`gcloud sql instances describe doula-cloud-pg --format='value(settings.databaseFlags)'` returned empty, 2026-10-31), so `cloudsql.iam_authentication` is off. Logins are not Terraform resources: "`google_sql_user` ... writes the generated or supplied password to Terraform state in plaintext" (`terraform/cloud_sql.tf`, header comment; `docs/infrastructure.md`, by-hand table). The two logins are `app_runtime_login` and `site_builder_login`, each a password user whose DSN lives in Secret Manager (`terraform/secrets.tf`: `pg_app_runtime_dsn`, read by `doula-api-runtime@`; `pg_site_builder_dsn` and `pg_migrate_dsn`, read by the deploy identity in CI).

The roles, from `api/db/migrations/`:

- **`app_runtime`** — `00002_practice_staff_tenancy.sql`. `CREATE ROLE app_runtime NOLOGIN`, guarded by `IF NOT EXISTS` because roles are cluster-wide. Granted table privileges table by table in later migrations. RLS reads `app.current_practice_id` (and `app.current_identity_uid` and others) set per transaction with `set_config(...)`; unset means zero rows.
- **`site_builder`** — `00046_practice_page_slug.sql`. Same guarded `CREATE ROLE ... NOLOGIN`, `GRANT SELECT` on five tables, and five permissive policies `FOR SELECT TO site_builder USING (true)` that sit beside the per-Practice ones. The comment explains why: "the existing policies name no role and so apply to everyone, including this one, which is why a grant alone would have returned zero rows." This is the repo's one existing cross-Practice reader role, and the closest precedent for an operator role.
- Login roles in tests: `api/internal/testdb/setup.go` creates a `LOGIN` role `IN ROLE app_runtime` once; `app/e2e/stack.ts` creates `app_e2e ... IN ROLE app_runtime`.

Facts about RLS that bear on an operator role:

- RLS is enabled on 44 tables. **`practices` has none** (no `ALTER TABLE practices ENABLE ROW LEVEL SECURITY` in any migration). `app_runtime` can list every Practice today. Postgres applies a table's policies only once row security is enabled on it, so 00046's `practices_site_builder` policy has no effect today; `site_builder` reads `practices` through its grant alone.
- No table uses `FORCE ROW LEVEL SECURITY`, so the table owner (the migration login) bypasses every policy, which is what makes a `SECURITY DEFINER` function work.
- The BFF itself chooses `app.current_practice_id`. `api/internal/billing/monthly.go` and `dormancy.go` read every Practice's ledger as `app_runtime` by looping over Practices and calling `set_config('app.current_practice_id', id, true)` for each. So RLS protects one Practice from a request acting for another; it does not stop the BFF process from reading any Practice it decides to. What a separate role can add is a set of **functions and policies the Practice BFF's role is not granted at all**.

`SECURITY DEFINER` functions on trunk (17 migrations mention it) take two shapes:

- **Handler-called, granted to one role.** 00010 (`push_subscriptions_for_message_recipient`), 00067, 00081, 00084, 00108, 00121, 00122 (`feedback_sender`), 00125 (`pilot_admits`), 00130, 00134. Each does `REVOKE EXECUTE ON FUNCTION ... FROM PUBLIC; GRANT EXECUTE ON FUNCTION ... TO app_runtime;`. 00010's comment: "Postgres grants EXECUTE to PUBLIC by default on CREATE FUNCTION, which would undercut this function's own 'narrow, engagement-scoped escape hatch' premise".
- **Policy-called, left on `PUBLIC`.** `current_staff_id()` (00003), `staff_is_sole_owner` (00106), `staff_was_ever_a_member_at_practice` (00116). 00111's comment explains: "a function a policy calls has to be executable by every role the policy applies to", and "00010's revoke is for a function a *handler* calls with arguments of its choosing, which is the case where the grant is the boundary."

There is no `ALTER DEFAULT PRIVILEGES` in any migration, and no test reads function ACLs (`has_function_privilege`, `proacl`) anywhere in `api/`.

### Database connections

`api/main.go` opens one `database/sql` pool on `DATABASE_URL` with the `pgx` driver and sets no `SetMaxOpenConns`. `doula-api` can scale to 20 instances. Cloud SQL's flags page gives a default `max_connections` of 25 for "tiny (~0.5)" and 50 for "small (~1.7)" and does not say which row `db-f1-micro` (about 0.6 GB) falls in ([Cloud SQL flags](https://docs.cloud.google.com/sql/docs/postgres/flags)). A second service opens a second pool against the same limit.

### The 100% coverage gate

`docs/testing/gates.md`, "Coverage": `ci.yml` job `api` runs `go test -coverprofile=coverage.out ./...` across the whole `doula-cloud/api` module, then `go run ./tools/covcheck -profile=coverage.out -module=doula-cloud/api -skip=doula-cloud/api/tools/`. Any package under `api/` other than `tools/` is gated, including every `api/cmd/*` (`hostingedge`, `migrate`, `simclock` each have a `main_test.go`). `app/` is gated on `src/lib/**` only, through `app/vite.config.ts`.

## 2. A second Hosting site on `ops.doula.cloud`, rewriting to a second Cloud Run service

### What the primary docs say

- **Multiple sites.** "The multisite feature supports a maximum of 36 sites per Firebase project." "Each site has its own hosting configuration", one `firebase.json` entry per site, tied to a site by a deploy target in `.firebaserc` ([Share project resources across multiple sites](https://firebase.google.com/docs/hosting/multisites)).
- **Custom domain.** A subdomain such as `app.example.com` is connected directly. Ownership is a TXT record, then A/AAAA records; "Each custom domain can only be connected to one Hosting site"; "Each custom domain is limited to having 20 subdomains per apex domain"; the certificate "may take up to 24 hours after you point your DNS to Firebase Hosting" ([Connect a custom domain](https://firebase.google.com/docs/hosting/custom-domain)).
- **Rewrite to Cloud Run.** A `run` rewrite takes `serviceId`, `region` (default `us-central1`) and optional `pinTag`. `us-central1` is on the list of regions to "colocate your Cloud Run service with Hosting". The service is deployed to "allow unauthenticated invocations". "Firebase Hosting is subject to a 60-second request timeout." `pinTag` depends on Cloud Run tags, limited to 1000 per service ([Hosting full config](https://firebase.google.com/docs/hosting/full-config); [Serve dynamic content with Cloud Run](https://firebase.google.com/docs/hosting/cloud-run)). Neither page says whether the service must be in the same project; here it would be, so the question does not arise.
- **Cookies.** "cookies are generally stripped from incoming requests"; "Only the specially-named `__session` cookie is permitted to pass through" ([Manage cache behavior](https://firebase.google.com/docs/hosting/manage-cache)). An ops site on its own subdomain gets its own `__session` cookie, scoped to that host, separate from the app's.
- **Cost.** Hosting: storage "No-cost up to 10 GB", transfer "No-cost up to 360 MB/day"; custom domain and SSL and multiple sites are listed with no price; "No-cost usage quotas apply at the project-level" ([Firebase pricing](https://firebase.google.com/pricing)). So a third site shares the quota the other two already use.

### Terraform

- `google_cloud_run_v2_service` and `google_cloud_run_v2_service_iam_member` are in the `hashicorp/google` provider the repo already pins (`terraform/versions.tf`: `hashicorp/google ~> 7.0`); `doula-api` uses both.
- `google_firebase_hosting_site` and `google_firebase_hosting_custom_domain` both carry: "This resource is in beta, and should be used with the terraform-provider-google-beta provider" (provider docs, `website/docs/r/firebase_hosting_site.html.markdown` and `firebase_hosting_custom_domain.html.markdown` in `hashicorp/terraform-provider-google`). The custom-domain resource exposes `required_dns_updates` and a `wait_dns_verification` argument.
- So Terraform-owning the site or the domain means adding a second provider, `google-beta`, against `docs/infrastructure.md`'s current decision to keep Hosting by hand. Leaving them by hand means `firebase hosting:sites:create` once and the domain connected in the console, the way `doula-cloud` and `doula-cloud-app` were.

### Cold starts and cost of the second service

From [Cloud Run pricing](https://cloud.google.com/run/pricing), request-based billing, `us-central1`, default prices:

| Item | Price |
| --- | --- |
| CPU, active | $0.000024 per vCPU-second |
| CPU, idle min instance | $0.0000025 per vCPU-second |
| Memory, active and idle min instance | $0.0000025 per GiB-second |
| Requests | $0.40 per million |
| Free tier (aggregated per billing account) | 180,000 vCPU-seconds, 360,000 GiB-seconds, 2 million requests a month |

"Idle instances that are not minimum instances are not charged." [Min instances](https://docs.cloud.google.com/run/docs/configuring/min-instances): min instances keep containers warm "to avoid slow container start times and reduce service latency", and Google "recommends that you apply minimum instances at the service level."

Arithmetic (the researcher's, not Google's): one idle min instance at `doula-api`'s size, 1 vCPU and 0.5 GiB, for a 30-day month (2,592,000 seconds) is 2,592,000 × $0.0000025 = $6.48 of CPU plus 2,592,000 × 0.5 × $0.0000025 = $3.24 of memory, about **$9.72 a month** before any free-tier offset. At `min-instances=0`, an operator's few dozen requests a day are well inside the free tier and each first request after idle pays a cold start. The free tier is a spending discount shared across the billing account, which `doula-api` already draws on.

Two repo-specific costs on top: a second Secret Manager secret for the ops DSN if it uses a password, and the 60-second Hosting timeout, which matters for any operator read that loops over every Practice the way `billing.MonthlyCredits` does.

## 3. Postgres roles on Cloud SQL for a second service

### Two ways to log in

**Password login (what the repo does today).** A migration creates `ops_runtime NOLOGIN` (guarded like 00002); by hand, someone creates `ops_runtime_login` with a password and `GRANT ops_runtime TO ops_runtime_login`; the DSN goes into a new Secret Manager secret readable only by the ops service's runtime service account; the service mounts `/cloudsql` like `doula-api`. No new connection code. Same by-hand step as `app_runtime_login` and `site_builder_login`.

**IAM database authentication.** From Google's docs:

- Enabled per instance with the `cloudsql.iam_authentication` flag, default off, no restart required ([flags](https://docs.cloud.google.com/sql/docs/postgres/flags)). Off on `doula-cloud-pg` today (live read above).
- Tokens "are short-lived and valid for only one hour". Automatic IAM authentication "is supported by the Cloud SQL Auth Proxy, the Go connector, the Java connector, and the Python connector" ([IAM authentication](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication)). The Go sample uses `cloudsqlconn.WithIAMAuthN()` with `WithLazyRefresh()`, "recommended for serverless environments" ([Log in using IAM](https://docs.cloud.google.com/sql/docs/postgres/iam-logins)).
- Neither the IAM pages nor [Connect from Cloud Run](https://docs.cloud.google.com/run/docs/configuring/connect-cloudsql) say that IAM authentication works through Cloud Run's built-in `/cloudsql` Unix socket, which is how `doula-api` connects. The documented path is the connector in-process. So IAM auth means different connection code in the ops service from `doula-api`'s plain `sql.Open("pgx", DSN)`.
- The database user for a service account is its email without `.gserviceaccount.com`, for example `ops-runtime@doula-cloud.iam`. A new IAM user is "granted no privileges on any databases, by default"; privileges come from `GRANT`. Principals need `roles/cloudsql.instanceUser` to log in, plus `roles/cloudsql.client` to connect ([Add and manage IAM users](https://docs.cloud.google.com/sql/docs/postgres/add-manage-iam-users)).
- Terraform: `google_sql_user` with `type = "CLOUD_IAM_SERVICE_ACCOUNT"` takes no password ("Don't set this field for CLOUD_IAM_USER and CLOUD_IAM_SERVICE_ACCOUNT user types"), and the provider's own example names it `trimsuffix(google_service_account.x.email, ".gserviceaccount.com")` (`website/docs/r/sql_user.html.markdown`). That removes the reason `docs/infrastructure.md` gives for keeping logins out of Terraform: no password reaches state. `GRANT ops_runtime TO "ops-runtime@doula-cloud.iam"` is still a SQL statement someone runs, by hand or in a migration that knows the name.

### Keeping operator functions away from `app_runtime`

PostgreSQL's own guidance ([CREATE FUNCTION, "Writing SECURITY DEFINER Functions Safely"](https://www.postgresql.org/docs/16/sql-createfunction.html)): "by default, execute privilege is granted to PUBLIC for newly created functions ... you must revoke the default PUBLIC privileges and then grant execute privilege selectively. To avoid having a window where the new function is accessible to all, create it and set the privileges within a single transaction." It also says to set `search_path` with `pg_temp` last. goose runs "All statements within a single migration file ... within the same transaction" unless the file carries `-- +goose no transaction` ([goose annotations](https://pressly.github.io/goose/documentation/annotations/)), so a migration that creates and revokes together meets the "single transaction" part.

[Row security](https://www.postgresql.org/docs/16/ddl-rowsecurity.html): permissive policies "are combined using OR"; "If no role is specified ... the policy applies to all users"; "Table owners normally bypass row security"; superusers and `BYPASSRLS` roles always do.

So, for an `ops_runtime` role, two shapes the repo already uses:

1. **`TO ops_runtime` policies** (00046's `site_builder` shape). `CREATE POLICY ... FOR SELECT TO ops_runtime USING (true)` beside each table's per-Practice policy. `app_runtime` is not in `ops_runtime` and the policy names only `ops_runtime`, so `app_runtime` gains nothing. Cost: one policy per table per command, and a door onto every row of that table for that role. 00122's comment rejected this shape for the founder's feedback read for that reason ("A policy on each of the three would be a door onto every Staff member ... evaluated on every query those tables ever serve").
2. **Handler-called `SECURITY DEFINER` functions** (00010 shape) with `REVOKE ... FROM PUBLIC; GRANT EXECUTE ... TO ops_runtime`, never to `app_runtime`. Each returns only the columns the operator page prints. Per 00111, such a function must be called by a handler, never by a policy, or it has to stay executable by everyone the policy covers.

Either way, three things keep the boundary true, and none exists today:

- No `GRANT ops_runtime TO app_runtime` or to `app_runtime_login`, ever. A role-membership check (`pg_has_role('app_runtime', 'ops_runtime', 'MEMBER')` is false) is a cheap test.
- Every new function revokes `PUBLIC`. A guard can be either `ALTER DEFAULT PRIVILEGES FOR ROLE <migration login> REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` (then policy-called functions must be granted explicitly to each role a policy covers) or a Go test over `pg_proc` that fails on any `prosecdef` function `app_runtime` can execute that is not on a named list. Neither is on trunk.
- Writes by the operator (founding grant, refunds, pilot allowlist) go through the same functions or grants, with `granted_by`-style actor columns as today, so the audit trail does not depend on which service wrote the row.

Today's operator endpoints (section 1) run as `app_runtime`. Moving them to an ops service under `ops_runtime` means the grants they rely on move to `ops_runtime` and are revoked from `app_runtime`, or `app_runtime` keeps them and the boundary covers only new functions.

## 4. Shape options for the Go code

| | A. Second binary in `api/` (`api/cmd/ops`), second Cloud Run service | B. Routes on `doula-api`, served only on the ops host | C. Separate Go module in `ops/` |
| --- | --- | --- | --- |
| Database role | Its own (`ops_runtime`) | `app_runtime`; the process holds every grant | Its own |
| Shares `api/internal/*` | Yes, same module | Yes | No; `internal/` is not importable across modules |
| `Deps`, `routes*.go`, `outboxes.go` | `package main` at `api/` root (`api/routes.go:63`), so not importable; shared wiring moves into an `internal/` package or is written again | Reused as is | Written again |
| Coverage gate | Covered automatically by `go test ./...` and `covcheck` over the module | Unchanged | Needs its own `covcheck` run and CI job |
| CI deltas | A second image (Dockerfile build arg or second Dockerfile; `api/Dockerfile` builds `.` today), a second `api-image` smoke test, a second `deploy-cloudrun` step and Artifact Registry image name, a third Hosting deploy target and preview workflow | Third Hosting site and deploy only | All of A, plus module setup, lint and toolchain guardrails (`api/toolchain_guardrail_test.go`) twice |
| Terraform | Second `google_cloud_run_v2_service` with ADR-0034's `ignore_changes` split, a runtime service account, its `cloudsql.client` grant and secret grants, `allUsers` invoker | None for the service | Same as A |
| Boundary from the Practice BFF | A role the Practice BFF never holds | A host check inside one process; Hosting's rewrite cannot add a header, and Cloud Run's `run.app` URL answers any path (ADR-0056) | Same as A |

How B would know which host a request came through is not documented on the Hosting pages read here, and nothing in `api/` reads a forwarded host today. Whatever it reads, per ADR-0056 the `run.app` URL is reachable directly with `allUsers` and a caller sets its own headers, so a host check is a routing convenience, not a boundary. B does not meet #2409's stated reason for a separate app.

A variant of A from ADR-0037's rejected design, **one image deployed as two services with a mode flag**, avoids a second Dockerfile and image but keeps two services, two env sets and two Terraform resources, which is most of A's cost; and the ops service would carry the Practice routes in its binary, unused.

**Recommendation (researcher's, for #2415 to accept or not):** A, with `ops_runtime` as a `NOLOGIN` role from a migration, handler-called `SECURITY DEFINER` functions granted to it alone, and a Go test over `pg_proc` that names every function `app_runtime` may execute. Password login first, matching `app_runtime_login`, since IAM auth adds a connector and a by-hand instance flag for one service; IAM auth is worth its own decision when both services can move together. `min-instances=0` for ops, since the operator can wait out a cold start and the alerting path (Cloud Scheduler calling the ops service) is not latency-bound. Hosting site and `ops.doula.cloud` by hand, matching `docs/infrastructure.md`, rather than adding `google-beta`.

## Open facts not settled here

- The live `max_connections` on `doula-cloud-pg`. Read it with `SELECT setting FROM pg_settings WHERE name = 'max_connections'` before sizing a second pool.
- Whether the migration login owns every table (so `SECURITY DEFINER` functions it creates bypass RLS on all of them). The repo assumes so (00010's comment); a one-line `pg_tables.tableowner` query confirms it.
- Local development (`dev:full`, an operator seed) is listed under #2409's "Not yet specified" and is not researched here.
