# Infrastructure: what Terraform owns, and what stays by hand

This is the specification [#797](https://github.com/markgoho/doula-cloud/issues/797) asked for. It names every resource in the `doula-cloud` GCP project, says which of them Terraform owns and which stay by hand with a reason, and settles the four mechanical questions — the Cloud Run image conflict, who may run `apply`, where state lives, and how the first apply is made safe. The decision itself is recorded in [ADR-0034](adr/0034-terraform-owns-the-shape-not-the-image-and-apply-stays-off-ci.md). The Terraform itself is not here; it is this ticket's children.

**If you are about to provision something by hand, this page is the boundary.** Anything in the "Terraform owns it" table below must be added to the configuration and applied, not created in a console. Anything in the "stays by hand" table is yours to create, and the reason it is not in code is written beside it.

## Why this exists

[#481](https://github.com/markgoho/doula-cloud/issues/481) is the case. `docs/environment.md` claimed ten Cloud Scheduler jobs existed and three did; four outboxes shipped after that ticket was filed and were never provisioned, two of them with no ADR-0013 nudge either, so they did not run at all for three days. Documentation cannot notice that. `terraform plan` can, and running it as a red CI job is the whole of what is being bought here.

**Repeatable provisioning is not the argument, and should not be made.** One project, one environment, provisioned once. The argument is drift detection, plus the one-time forcing function an import pass applies: every resource in the project has to be accounted for before it can be imported, which is how the two unaccounted Cloud Run services below finally got an answer.

## The inventory, read from `gcloud` on 2026-09-08

Read against the live project, not from documentation. This table supersedes the one on #797, which was incomplete — Artifact Registry, the Workload Identity pool, Cloud Run and Cloud Tasks IAM, and Firestore were all missing from it.

| Resource | What is there | Owned by Terraform? |
| --- | --- | --- |
| Cloud Run services | `doula-api`; `redirecturl` and `shortenurl` (2nd-gen Cloud Functions, being deleted) | `doula-api` yes, minus its image; the other two are deleted rather than imported |
| `doula-api` env vars and secret references | 19 — 8 plain values, 11 Secret Manager references | Yes |
| Cloud Run service IAM | `allUsers` → `roles/run.invoker` on all three services | `doula-api`'s yes; the other two go with their services |
| Cloud Scheduler jobs | `process-outbox-drain` (`*/5 * * * *`), `verify-practice-pages` (`*/15 * * * *`) | Yes |
| Cloud Tasks queues | `doula-cloud-notification-nudge` | Yes |
| Cloud Tasks queue IAM | `850855848778-compute@` → `roles/cloudtasks.enqueuer` | Yes |
| Secret Manager secrets | 13 | Shells yes, versions no |
| Service accounts | `github-action-733741680@`, `firebase-adminsdk-rq3g0@`, `firebase-app-hosting-compute@`, `850855848778-compute@` (Google-created default) | The first yes; the three Google-created ones no |
| Project-level IAM bindings | 34 role bindings, 37 principal-role pairs, 22 distinct principals | Yes, for the non-service-agent ones |
| Workload Identity pool and provider | pool `github-actions`, provider `github` | Yes |
| Artifact Registry repositories | `api`, `cloud-run-source-deploy`, `gcf-artifacts`, `firebaseapphosting-images` | `api` yes; the other three no |
| Cloud SQL instance | `doula-cloud-pg`, POSTGRES_16, `db-f1-micro`, ZONAL, public IP, `sslMode: TRUSTED_CLIENT_CERTIFICATE_REQUIRED`, 10 GB, 7 backups retained, **`deletionProtectionEnabled: false`** | Instance settings and databases yes; users no |
| Cloud SQL databases and users | databases `doula_cloud`, `postgres`; users `app_runtime_login`, `site_builder_login`, `postgres` | Databases yes, users no |
| GCS buckets | `doula-cloud-attachments`; `doula-cloud.firebasestorage.app`, `run-sources-…`, `gcf-v2-sources-…`, `gcf-v2-uploads-…` | `doula-cloud-attachments` yes; the rest no |
| Firestore | one `(default)` FIRESTORE_NATIVE database, created by Firebase | No |

#797's "34 project-level IAM bindings" is the role count. The number that matters for Terraform is 37, the principal-role pairs, because `google_project_iam_member` is one resource per pair. Seventeen of those 37 belong to Google's own service agents, which the boundary below leaves alone; the other twenty are owned.

## What Terraform owns

Each of these is a resource in the configuration, and a difference between the configuration and the live project makes `plan` non-empty and the CI job red.

| Resource | Terraform resource | Why it is owned |
| --- | --- | --- |
| The two Cloud Scheduler jobs | `google_cloud_scheduler_job` | This is #481's failure verbatim. A missing job is invisible today and a red `plan` tomorrow. Both carry an `X-Internal-Secret` header whose value comes from a `google_secret_manager_secret_version` data source, never a literal — see [State](#state) for what that puts in the state file. |
| The Cloud Tasks queue and its IAM binding | `google_cloud_tasks_queue`, `google_cloud_tasks_queue_iam_member` | ADR-0013's nudge path. The queue existing without the enqueuer binding is a silent half-configuration, which is exactly the class of thing being caught. |
| `doula-api`'s service configuration | `google_cloud_run_v2_service`, minus the fields in [The Cloud Run image conflict](#the-cloud-run-image-conflict) | 19 environment variables and secret references set out of band by `gcloud run services update`, with nothing but `docs/environment.md` recording that they should be there. Each one is a `500` waiting to happen if a deploy ever drops it. |
| Cloud Run service IAM | `google_cloud_run_v2_service_iam_member` | `allUsers` on a public API is a deliberate choice today; it should be one that shows up in a diff if it ever changes. |
| Secret Manager secret shells | `google_secret_manager_secret` | The container, its replication policy, and its accessor grants. A secret that exists with the wrong accessor fails at container start, which is the failure #743 walked by hand. |
| `github-action-733741680@` and its grants | `google_service_account`, `google_project_iam_member` | The identity every deploy runs as. Its ten project roles are the highest-value thing in the project to have reviewable. |
| The Workload Identity pool and provider | `google_iam_workload_identity_pool`, `google_iam_workload_identity_pool_provider` | The attribute condition on this provider — today `assertion.repository == 'markgoho/doula-cloud'` — is the single string that stops another GitHub repository from minting tokens for this project. It has never been reviewed in a diff because it has never been in a file. |
| Non-service-agent project IAM | `google_project_iam_member` | Google's own service agents (`service-…@gcp-sa-*`, `…@cloudservices`, `…@cloudbuild`) are excluded: the platform creates and repairs them, and importing them means Terraform proposing to delete a binding Google will immediately recreate. Everything else — the GitHub Actions account, the Firebase accounts, the default compute account, and `markgoho@gmail.com`'s `roles/owner` — is owned. |
| Artifact Registry `api` | `google_artifact_registry_repository` | The repository, not the images in it. Same split as Cloud Run: Terraform owns the shape, CI owns the contents. |
| `doula-cloud-pg` instance settings and databases | `google_sql_database_instance`, `google_sql_database` | Tier, disk, backup schedule, retention, SSL mode and deletion protection. `docs/environment.md` describes none of these today, so nothing would notice if a backup schedule were turned off. |
| `doula-cloud-attachments` | `google_storage_bucket` | The one project-owned bucket. It carries uniform bucket-level access and enforced public-access prevention, and no lifecycle rule or versioning; nothing in the repository records any of that, so nothing would notice it being relaxed. |

## What stays by hand

Nothing here is an oversight. Each line is a decision, and the reason is the point of the line.

| Resource | Why it is not in Terraform |
| --- | --- |
| Secret **values** (every `google_secret_manager_secret_version`) | A version's payload is stored in Terraform state in plaintext. Putting the Stripe key, the Mailgun key and three database DSNs in state trades one console step for a file that is worth more than the console. The shells are owned; the values are added with `gcloud secrets versions add`, as `docs/environment.md` already describes. |
| `doula-api`'s container image | `ci.yml`'s `deploy-api` sets it on every merge to trunk. See the next section — this is the one conflict that decides whether any of this works. |
| Cloud SQL users and their passwords | `google_sql_user` writes the password to state. `app_runtime_login` and `site_builder_login` are created by hand and their DSNs live in Secret Manager. The instance and the databases are owned; the logins are not. |
| Identity Platform configuration | `signIn.email`, the MFA settings, the authorized domains. The Terraform provider covers `google_identity_platform_config` unevenly, and ADR-0026's two sign-in methods are a product decision walked by hand once. Revisit only if it drifts. |
| Firebase Hosting, Firestore, and the Firebase-created buckets | `firebase.json` and `.firebaserc` are already code and already deploy from CI; `doula-cloud.firebasestorage.app` and the `(default)` Firestore database were created by Firebase enabling itself and are managed by it. Adding a second owner would produce drift, not detect it. |
| Google-created service accounts and service agents | `850855848778-compute@`, `firebase-adminsdk-rq3g0@`, `firebase-app-hosting-compute@`, and every `service-…@gcp-sa-*` principal. The platform creates, grants and repairs these; Terraform proposing to remove a binding Google recreates makes `plan` permanently red, which is the failure mode this whole effort is trying to avoid. |
| Artifact Registry `cloud-run-source-deploy`, `gcf-artifacts`, `firebaseapphosting-images` | All three are created by a tool for its own use — `gcloud run deploy --source`, Cloud Functions, and Firebase App Hosting. `gcf-artifacts` goes away with the two functions below. |
| The Terraform state bucket | It has to exist before Terraform runs. Created once by hand; the commands are in the [State](#state) section so the creation is repeatable even though it is not applied. |
| The GCS buckets Google made for builds | `run-sources-…`, `gcf-v2-sources-…`, `gcf-v2-uploads-…`. Build scratch space, recreated on demand. |
| Mailgun, Stripe, GitHub | Outside GCP entirely. `docs/environment.md` is where these are described, and this specification does not move them. |
| goose migrations | Already code, already run by `ci.yml`'s `migrate` job. Terraform owns the instance, never the schema. |

## The Cloud Run image conflict

This is the question that decides whether adopting Terraform is worth anything. `ci.yml`'s `deploy-api` runs `google-github-actions/deploy-cloudrun@v3` on every merge to trunk. If Terraform also owns what that action writes, `plan` is red after every merge, forever, nobody reads it, and the result is worse than not adopting Terraform at all.

**The mutation set is not a guess.** Two consecutive real CI deploys — `doula-api-00491-lrt` (trunk `26eab783`) and `doula-api-00492-b96` (trunk `c9097a3f`), eight minutes apart on 2026-09-09 — differ in exactly these places, and nowhere else:

- `spec.template.containers[0].image` — the tag, and with it the resolved digest
- the `commit-sha` label, at both the service and the revision-template level
- `client.knative.dev/nonce`, and the `run.googleapis.com/client-name` / `client-version` annotations that record which tool wrote the revision
- the revision's own name, `configurationGeneration`, `uid`, `resourceVersion` and timestamps

Everything else is byte-identical across the two: all 19 environment variables and secret references, the service account, `containerConcurrency`, the CPU and memory limits, the startup probe, `--min-instances=1`, and the Cloud SQL instance attachment. That is the useful result. `deploy-cloudrun` is configured with `env_vars_update_strategy: merge`, and `secrets:` merges by default, so the action re-asserts three settings (the image, `EXPECTED_ORIGINS`, and `DATABASE_URL`) and leaves the other sixteen alone rather than replacing them. Terraform can own those sixteen without ever contending with CI for them.

**So the split is: Terraform owns the shape, CI owns the image.** One `google_cloud_run_v2_service` resource, with a `lifecycle` block:

```hcl
lifecycle {
  ignore_changes = [
    client,
    client_version,
    template[0].containers[0].image,
    template[0].labels["commit-sha"],
  ]
}
```

Four of the six mutated fields need no entry. The revision name, generation and timestamps are computed attributes the provider does not diff. The `commit-sha` label at the *service* level is not one either, even though `gcloud` shows it: the v2 provider splits labels into `labels` (what the configuration declares, empty here), `terraform_labels`, and `effective_labels` (what is live, which does carry `commit-sha`), and diffs only the first two. Verified in the imported state. The `commit-sha` label on the *revision template* is a declared field and does drift, which is why it is the one label in the list. `client.knative.dev/nonce` is a system label the v2 API does not expose at all. A narrower resource was considered and rejected — the v2 provider has no "service configuration without the image" resource, and splitting the service across two resources means two owners for one API object, which is the problem being solved rather than a solution to it.

**Verified, not asserted.** Terraform 1.16.1 with `hashicorp/google` v7, a throwaway configuration and local state, run against the live project:

1. `terraform plan -generate-config-out` against an `import` block for `projects/doula-cloud/locations/us-central1/services/doula-api` generated a 244-line resource from the live service and reported `1 to import, 0 to add, 0 to change, 0 to destroy`.
2. `terraform apply` performed the import — **import writes state only; no `apply` in this exercise ever changed the project.**
3. `terraform plan` against the imported state: *No changes. Your infrastructure matches the configuration.*
4. The configuration was then edited to hold a different image tag, a different `commit-sha` template label and a different `client_version` — the three declared fields a deploy moves — and planned twice. **Without** the `lifecycle` block: `Plan: 0 to add, 1 to change, 0 to destroy`, naming those three and nothing else. **With** it: *No changes.* That is both halves of the claim — the plan is genuinely sensitive to those fields, and the block is what makes it ignore them.
5. The same plan re-run after a real trunk deploy. This ticket's own merge is that deploy.

Every command and every plan output above is on [#797, as a comment](https://github.com/markgoho/doula-cloud/issues/797#issuecomment-5594346220), including the two things the spike settled that were not obvious from the API surface. The first build sub-issue re-runs the whole sequence against the committed configuration rather than trusting the spike.

One thing the generated configuration turned up that the boundary above now accounts for: the live service still carries a `build_config` block pointing at `gs://run-sources-doula-cloud-us-central1/services/doula-api/1786507208…zip`, left behind by a long-past `gcloud run deploy --source`. It is inert and does not drift today — every deploy since has been by image reference, and the plan above was empty with it declared as-is — but it is exactly the kind of residue an import pass exists to surface, and a single future `--source` deploy would turn it red. Clear it, or add `build_config` to `ignore_changes`, before the first apply.

## Who may apply, and what CI may read

**`plan` runs in CI. `apply` runs from a laptop, by the one person who has `roles/owner`.** The reason is not ceremony. Applying this configuration needs IAM-admin-class permission — `roles/iam.serviceAccountAdmin`, `roles/resourcemanager.projectIamAdmin`, `roles/iam.workloadIdentityPoolAdmin`, `roles/secretmanager.admin`, `roles/cloudsql.admin`. A principal holding those in GitHub Actions can grant itself anything in the project, and the thing it would authenticate through is the Workload Identity provider that the same configuration owns. For a business with one person in it, the convenience of `apply` on merge does not pay for that.

`apply` therefore runs interactively, from the main checkout, as `markgoho@gmail.com` over `gcloud auth application-default login`. No service account holds apply-class permission at all, so there is no credential to leak.

**What CI holds is read.** A second service account — call it `terraform-plan@doula-cloud.iam.gserviceaccount.com` — authenticates the same way `deploy-api` does, through the existing `github-actions` Workload Identity pool, and holds only enough to refresh state and diff:

| Role | Why `plan` needs it |
| --- | --- |
| `roles/viewer` | Reads almost everything the configuration covers: the Cloud Run service, the Scheduler jobs, the Tasks queue, the Cloud SQL instance and databases, the Artifact Registry repository, the secret shells, and the project IAM policy |
| `roles/iam.securityReviewer` | The `getIamPolicy` calls `roles/viewer` does not carry — confirmed missing for `cloudtasks.queues`, `storage.buckets` and `artifactregistry.repositories`, all three present here |
| `roles/iam.workloadIdentityPoolViewer` | `roles/viewer` carries no `iam.workloadIdentityPools.get`; without it the pool and provider resources cannot refresh |
| `roles/storage.legacyBucketReader` | `roles/viewer` carries no `storage.buckets.get` either, so `doula-cloud-attachments` cannot refresh, and neither can the state bucket |
| `roles/secretmanager.secretAccessor`, **granted on two secrets, not the project** | Only so the Scheduler jobs' `X-Internal-Secret` header can come from a data source rather than a literal in the repository. Every other secret stays unreadable to CI |

That list is derived from `gcloud iam roles describe`, not from memory, but it is a starting point and not a finding: the first build sub-issue runs `plan` as that account and adds whatever a real refresh turns out to need, because a permission gap surfaces as a failed refresh rather than as a wrong answer.

**A red `plan` is a red required check on trunk, not an advisory annotation.** ADR-0034 has the argument. A drift job nobody has to act on is a drift job nobody reads, which is where documentation already failed.

## State

**State lives in a GCS bucket in this same project, `doula-cloud-tfstate`, versioned, uniform access, public access prevention enforced, and created by hand because it has to exist before Terraform can run.** Terraform's GCS backend takes a lock object in the same bucket, which is enough for one person and one CI job.

```sh
gcloud storage buckets create gs://doula-cloud-tfstate \
  --project doula-cloud --location us-central1 \
  --uniform-bucket-level-access --public-access-prevention
gcloud storage buckets update gs://doula-cloud-tfstate --versioning
```

**Who can read it: `markgoho@gmail.com` through `roles/owner`, and the plan account through `roles/storage.objectUser` on this bucket alone.** Nothing else. The bucket is not owned by Terraform — a configuration that owns its own state bucket can propose to delete it.

**What is in it, and why the boundary is drawn where it is.** Terraform state holds every attribute of every managed resource in plaintext, so the state file is as sensitive as the most sensitive thing in it:

- The two Scheduler jobs' `X-Internal-Secret` header value. Unavoidable if the jobs are owned at all — it is a field on the job — and owning the jobs is the entire #481 case. It is a shared secret guarding internal endpoints, rotatable in one `gcloud secrets versions add` plus one `terraform apply`, which is why this one is accepted.
- **Not** any Secret Manager payload, because only the shells are owned.
- **Not** any Cloud SQL password, because `google_sql_user` is not owned. This is the specific reason for that line in the by-hand table.
- The 8 plain `doula-api` environment variables, none of which is a secret; the 11 secret references are references, not values.

Anyone with read on the state bucket therefore has the internal worker secret. That is one principal and one service account, and it is the price of catching the next #481.

## Making the first apply safe

**The danger is not a wrong apply. It is a half-written configuration and a right-looking apply.** Terraform deletes what it manages and no longer sees. A configuration that covers eleven of the thirteen secrets, or the Cloud Run service without its Cloud SQL attachment, produces a plan that reads as reasonable and takes the live API down. There are no users yet, and the January 2027 target is what makes that survivable — but the database is real, and `doula-cloud-pg` has `deletionProtectionEnabled: false` today.

Four mechanisms, in this order:

1. **Import-only until the plan is empty.** No resource enters the configuration except by importing what already exists, and no `apply` runs that reports anything but `N to import, 0 to add, 0 to change, 0 to destroy`. An import-only apply cannot delete anything; it writes state. This is the first sub-issue's rule, and it is what the spike above already demonstrated for `doula-api` — generate, import, plan clean.
2. **`prevent_destroy` on everything that holds data or identity**, before the first apply, not after: the Cloud SQL instance, its databases, `doula-cloud-attachments`, every secret shell, and the Workload Identity provider. A `lifecycle { prevent_destroy = true }` block makes Terraform refuse the plan rather than execute it.
3. **Turn on Cloud SQL deletion protection.** `deletionProtectionEnabled` is `false` on the live instance right now — a GCP-level protection independent of anything Terraform believes. Setting it is one `gcloud` command and it should not wait for the Terraform work; it is the first sub-issue's first step.
4. **One resource kind per sub-issue, each landing with an empty plan.** The configuration is never in a state where a plan covers half of a resource kind. Scheduler jobs land complete, then the Tasks queue lands complete, and so on.

`terraform destroy` is never run against this project, by anyone, for any reason.

## `redirecturl` and `shortenurl`

**They are deleted, not imported.** [#87](https://github.com/markgoho/doula-cloud/issues/87) is why this needed an answer rather than an assumption: infrastructure that looks unused is not safe to delete on that basis, and acting on appearance there caused a live incident. So here is the answer, written down.

**What they are.** Both are 2nd-generation Cloud Functions, not plain Cloud Run services — `goog-managed-by: cloudfunctions`, `run.googleapis.com/client-name: cli-firebase`, `cloudfunctions.googleapis.com/function-id: redirectUrl` and `shortenUrl`, both built on 2025-03-26 from a single Cloud Build (`fb1e636f-…`) producing `gcf-artifacts/doula--cloud__us--central1__shorten_url:version_1`. `gcloud functions list` confirms both as ACTIVE 2nd-gen HTTP-triggered functions. They are a Firebase Functions URL shortener deployed with `firebase deploy`.

**Whose they are.** Mark's account, on this ticket: they are a previous incarnation of this project, from before the current Svelte-and-Go stack existed. Nothing in this repository references either one — a full-tree search for `shortenurl`, `redirecturl`, `shorten_url`, `redirect_url` and `shortUrl` returns nothing outside `.git`.

**What depends on them: nothing.** Thirty days of Cloud Run request logs, read on 2026-09-08:

- `shortenurl`: **zero requests**, of any kind.
- `redirecturl`: traffic on one day only, 2026-08-17, and every request is a hostile scan — `POST /xmlrpc.php`, and roughly fifty `GET`s for PHP web-shell filenames (`wp-load.php`, `root.php`, `usr.php`, `a.php`). Every one answered `404`. There is not a single successful request in the window.
- Both carry `allUsers` → `roles/run.invoker`, so both are open to the internet. What the logs show is a public attack surface for a service nobody uses.

**What goes with them.** Deleting the functions makes four things removable that exist only to support them, and each should be confirmed empty rather than deleted on sight: the `gcf-artifacts` Artifact Registry repository, the `gcf-v2-sources-850855848778-us-central1` and `gcf-v2-uploads-…` buckets, and two project IAM bindings — `roles/cloudfunctions.developer` on `github-action-733741680@` and `roles/cloudfunctions.admin` on `firebase-adminsdk-rq3g0@`. The `(default)` Firestore database stays: Firebase created it and the shortener's use of it, if any, is not what keeps it there.

**Removal is `gcloud functions delete redirectUrl` and `gcloud functions delete shortenUrl`, in `us-central1`** — deleting the underlying Cloud Run service instead leaves the Cloud Functions resource behind. It is independent of every Terraform decision above and does not wait on any of it; it is the first sub-issue in the sequence for that reason. Nothing else in the project is touched.

## What this specification found and did not fix

An import pass forces a question about every resource in the project, and asking those questions turned up five things that are not drift and are not this specification's job to fix. Each is filed rather than left here as prose.

- **`doula-api` runs as `850855848778-compute@developer.gserviceaccount.com`, which holds `roles/editor` on the project.** The API container can therefore do very nearly anything in GCP, including read every secret and modify every service. A dedicated runtime service account with the four grants the API actually uses — Cloud SQL client, the specific secret accessors, Cloud Tasks enqueuer, and object access on the attachments bucket — is what this should be. Owning IAM in Terraform is what makes the current state reviewable; it does not fix it. → [Give `doula-api` its own runtime service account instead of the default compute editor #1051](https://github.com/markgoho/doula-cloud/issues/1051)
- **`doula-api` is `allUsers` → `roles/run.invoker`, and that includes every `/api/internal/**` endpoint.** The only thing standing between the public internet and `POST /api/internal/outboxes/drain` is the `X-Internal-Secret` header. That is a deliberate design (`docs/environment.md`), and it is also the whole of the boundary. → [Decide the boundary on `/api/internal/**`, and stop storing its secret in the clear on the Scheduler jobs #1052](https://github.com/markgoho/doula-cloud/issues/1052)
- **The Scheduler jobs carry that same secret as a plaintext header value in the job spec**, readable by anything with `cloudscheduler.jobs.get`, which `roles/viewer` grants. → covered by the same ticket as the line above.
- **`doula-cloud-pg` has `deletionProtectionEnabled: false`.** → the first step of the bootstrap sub-issue, not a separate ticket.
- **`doula-api` carries a stale `build_config` from a long-past `gcloud run deploy --source`.** Inert today; a single future `--source` deploy makes the drift job red for a reason that has nothing to do with drift. → handled in the bootstrap sub-issue, which either clears it or ignores it explicitly.

One unrelated thing, found while numbering ADR-0034: **two ADRs both number themselves 0033** — `0033-overdue-is-derived-and-notifies-nobody.md` and `0033-staff-login-deletion-is-immediate-and-redacts-the-person.md`. Renaming either one breaks existing links, so it is not done here. → [Two ADRs both number themselves 0033, so the citation names two decisions #1053](https://github.com/markgoho/doula-cloud/issues/1053)

## The build

Seven sub-issues of [#797](https://github.com/markgoho/doula-cloud/issues/797), sequenced. The order is dependency, not priority — nothing here is ranked by impact, because there is nothing yet to impact.

| # | Work | Depends on |
| --- | --- | --- |
| 1 | [Delete `redirecturl` and `shortenurl`, the URL shortener from a previous incarnation #1043](https://github.com/markgoho/doula-cloud/issues/1043) | — |
| 2 | [Terraform bootstrap: state bucket, plan identity, and `doula-api` imported behind an empty plan #1044](https://github.com/markgoho/doula-cloud/issues/1044) | — |
| 3 | [Import the Cloud Scheduler jobs and the Cloud Tasks queue, so a missing job is a red plan #1045](https://github.com/markgoho/doula-cloud/issues/1045) | 2 |
| 4 | [Import the Secret Manager shells and their accessor grants, never the versions #1046](https://github.com/markgoho/doula-cloud/issues/1046) | 2 |
| 5 | [Import the deploy identity, project IAM, Workload Identity federation and the `api` registry #1047](https://github.com/markgoho/doula-cloud/issues/1047) | 2 |
| 6 | [Import the Cloud SQL instance, its databases and the attachments bucket, but not the logins #1048](https://github.com/markgoho/doula-cloud/issues/1048) | 2 |
| 7 | [Make `terraform plan` a required, scheduled CI check, so drift is reported instead of discovered #1049](https://github.com/markgoho/doula-cloud/issues/1049) | 3, 4, 5, 6 |

Steps 1 and 2 are independent of each other and can run in either order. Steps 3 through 6 each land one resource kind, complete, with an empty plan, and can run in any order once 2 is done. Step 7 is last on purpose: a drift check that goes red for a resource kind still half-imported teaches people to ignore it.

**The first apply's safety mechanism is named in step 2, and it is import-only-until-the-plan-is-empty**, with `prevent_destroy` on everything that holds data or identity and Cloud SQL deletion protection turned on before any Terraform runs at all. Every later step inherits it: no `apply` in this project ever reports a change or a destroy until its plan has first reported nothing.
