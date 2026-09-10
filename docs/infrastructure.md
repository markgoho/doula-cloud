# Infrastructure: what Terraform owns, and what stays by hand

This is the specification [#797](https://github.com/markgoho/doula-cloud/issues/797) asked for. It names every resource in the `doula-cloud` GCP project, says which of them Terraform owns and which stay by hand with a reason, and settles the four mechanical questions — the Cloud Run image conflict, who may run `apply`, where state lives, and how the first apply is made safe. The decision itself is recorded in [ADR-0034](adr/0034-terraform-owns-the-shape-not-the-image-and-apply-stays-off-ci.md). The Terraform itself lives at `terraform/` at the repo root, split into `versions.tf` (the `required_version`/`required_providers`/GCS backend), `provider.tf`, and one file per resource kind — `cloud_run.tf` today, from [#1044](https://github.com/markgoho/doula-cloud/issues/1044); #1045 through #1048 add the rest to this same directory and share this state.

**If you are about to provision something by hand, this page is the boundary.** Anything in the "Terraform owns it" table below must be added to the configuration and applied, not created in a console. Anything in the "stays by hand" table is yours to create, and the reason it is not in code is written beside it.

## Why this exists

[#481](https://github.com/markgoho/doula-cloud/issues/481) is the case. `docs/environment.md` claimed ten Cloud Scheduler jobs existed and three did; four outboxes shipped after that ticket was filed and were never provisioned, two of them with no ADR-0013 nudge either, so they did not run at all for three days. Documentation cannot notice that. `terraform plan` can, and running it as a red CI job is the whole of what is being bought here.

**Repeatable provisioning is not the argument, and should not be made.** One project, one environment, provisioned once. The argument is drift detection, plus the one-time forcing function an import pass applies: every resource in the project has to be accounted for before it can be imported, which is how the two unaccounted Cloud Run services below finally got an answer.

## The inventory, read from `gcloud` on 2026-09-08

Read against the live project, not from documentation. This table supersedes the one on #797, which was incomplete — Artifact Registry, the Workload Identity pool, Cloud Run and Cloud Tasks IAM, and Firestore were all missing from it.

| Resource | What is there | Owned by Terraform? |
| --- | --- | --- |
| Cloud Run services | `doula-api` only — `redirecturl` and `shortenurl` (2nd-gen Cloud Functions) were deleted 2026-09-08 (#1043) | `doula-api` yes, minus its image |
| `doula-api` env vars and secret references | 19 — 9 plain values, 10 Secret Manager references | Yes |
| Cloud Run service IAM | `allUsers` → `roles/run.invoker` on `doula-api` | Yes |
| Cloud Scheduler jobs | `process-outbox-drain` (`*/5 * * * *`), `verify-practice-pages` (`*/15 * * * *`) | Yes |
| Cloud Tasks queues | `doula-cloud-notification-nudge` | Yes |
| Cloud Tasks queue IAM | `doula-api-runtime@` → `roles/cloudtasks.enqueuer` (`850855848778-compute@` until #1051) | Yes |
| Secret Manager secrets | 13 | 12 shells and their accessor grants yes, versions no; the 13th (Firebase's own OAuth token) no — see the by-hand table |
| Service accounts | `github-action-733741680@`, `terraform-plan@` (#1044), `doula-api-runtime@` (#1051), `firebase-adminsdk-rq3g0@`, `firebase-app-hosting-compute@`, `850855848778-compute@` (Google-created default) | `github-action-733741680@` and `doula-api-runtime@` yes; the three Google-created ones no; `terraform-plan@`'s project roles are owned but the account itself is not yet a resource — see the by-hand table |
| Project-level IAM bindings | 36 role bindings, 39 principal-role pairs, 23 distinct principals — #1043 removed `roles/cloudfunctions.developer` and `roles/cloudfunctions.admin` (each the sole binding for its role), #1044 then added `terraform-plan@`'s three (`roles/viewer`, `roles/iam.securityReviewer`, `roles/iam.workloadIdentityPoolViewer`), and #1051 traded `roles/editor` on `850855848778-compute@` for two grants on `doula-api-runtime@` (`roles/cloudsql.client`, already a role in the policy, and the new `doulaApiStaffAccounts` custom role). The principal count is unchanged because the default compute account left the policy entirely as the runtime account entered it, and `roles/editor` survives with one member, `850855848778@cloudservices`, a Google service agent | Yes, for the non-service-agent ones |
| Workload Identity pool and provider | pool `github-actions`, provider `github` | Yes |
| Artifact Registry repositories | `api`, `cloud-run-source-deploy`, `firebaseapphosting-images` — `gcf-artifacts` was deleted 2026-09-08 (#1043) | `api` yes; the other two no |
| Cloud SQL instance | `doula-cloud-pg`, POSTGRES_16, `db-f1-micro`, ZONAL, public IP, `sslMode: TRUSTED_CLIENT_CERTIFICATE_REQUIRED`, 10 GB, 7 backups retained, `deletionProtectionEnabled: true` (#1044 turned it on; #1048 imported the instance to match) | Instance settings and databases yes; users no |
| Cloud SQL databases and users | databases `doula_cloud`, `postgres`; users `app_runtime_login`, `site_builder_login`, `postgres` | Databases yes, users no |
| GCS buckets | `doula-cloud-attachments`; `doula-cloud-tfstate` (#1044); `doula-cloud.firebasestorage.app`, `run-sources-…` — `gcf-v2-sources-…` and `gcf-v2-uploads-…` were deleted 2026-09-08 (#1043) | `doula-cloud-attachments` yes; the rest no, `doula-cloud-tfstate` deliberately so — see [State](#state) |
| Firestore | one `(default)` FIRESTORE_NATIVE database, created by Firebase | No |

The paragraph below is #1047's reading, kept as it stood so the import it justifies can still be checked against it. #1051 moved it since: 36 roles and 39 pairs, because the default compute account's `roles/editor` went and `doula-api-runtime@` arrived holding two — `roles/cloudsql.client` and the `doulaApiStaffAccounts` custom role. The owned-pair count is 21, not 20.

#797's "34 project-level IAM bindings" is the role count as it stood before #1043; it was 35 after #1043 removed two roles and #1044 added `terraform-plan@` as a principal with three of its own. The number that matters for Terraform is the principal-role pairs — 37 before #1043, 38 now (#1043 removed two, #1044 added three) — because `google_project_iam_member` is one resource per pair. Read fresh from `gcloud projects get-iam-policy doula-cloud` for this ticket (#1047) rather than carried forward from an earlier count: 18 of the 38 pairs belong to Google's own service agents, which the boundary below leaves alone, and the other 20 are owned and imported here — the nine on `github-action-733741680@`, three each on `firebase-adminsdk-rq3g0@` and `firebase-app-hosting-compute@`, one on the default compute account, one on `markgoho@gmail.com`, and the three #1044 added on `terraform-plan@`. The 18/20 split does not simply carry the pre-#1044 count forward with three added: it is the direct result of categorizing all 38 live pairs against the boundary above, and it is what #1047 actually imported — 24 resources in total, of which 20 are these project IAM pairs (the other 4 are the service account, the two Workload Identity resources, and the Artifact Registry repository).

## What Terraform owns

Each of these is a resource in the configuration, and a difference between the configuration and the live project makes `plan` non-empty and the CI job red.

| Resource | Terraform resource | Why it is owned |
| --- | --- | --- |
| The two Cloud Scheduler jobs | `google_cloud_scheduler_job` | This is #481's failure verbatim. A missing job is invisible today and a red `plan` tomorrow. Both carry an `X-Internal-Secret` header whose value comes from a `google_secret_manager_secret_version` data source, never a literal — see [State](#state) for what that puts in the state file. |
| The Cloud Tasks queue and its IAM binding | `google_cloud_tasks_queue`, `google_cloud_tasks_queue_iam_member` | ADR-0013's nudge path. The queue existing without the enqueuer binding is a silent half-configuration, which is exactly the class of thing being caught. |
| `doula-api`'s service configuration | `google_cloud_run_v2_service`, minus the fields in [The Cloud Run image conflict](#the-cloud-run-image-conflict) | 19 environment variables and secret references set out of band by `gcloud run services update`, with nothing but `docs/environment.md` recording that they should be there. Each one is a `500` waiting to happen if a deploy ever drops it. |
| Cloud Run service IAM | `google_cloud_run_v2_service_iam_member` | `allUsers` on a public API is a deliberate choice today; it should be one that shows up in a diff if it ever changes. |
| Secret Manager secret shells | `google_secret_manager_secret` | The container, its replication policy, and its accessor grants. A secret that exists with the wrong accessor fails at container start, which is the failure #743 walked by hand. |
| `github-action-733741680@` and its grants | `google_service_account`, `google_project_iam_member` | The identity every deploy runs as. Its nine project roles (ten until #1043 removed `roles/cloudfunctions.developer`) are the highest-value thing in the project to have reviewable. |
| `doula-api-runtime@` and its grants | `google_service_account`, `google_project_iam_custom_role`, `google_project_iam_member`, `google_secret_manager_secret_iam_member`, `google_cloud_tasks_queue_iam_member`, `google_storage_bucket_iam_member`, `google_service_account_iam_member` | The identity the API container runs as, created by [#1051](https://github.com/markgoho/doula-cloud/issues/1051). Its whole point is that the set of things it can do is small and written down: a grant added to it by hand is a red `plan`, which is the only thing that keeps it small. The custom role is there because Identity Platform has no resource-level IAM — the only way to narrow a project-wide grant is to narrow the permissions, and `roles/firebaseauth.admin` would let the container rewrite ADR-0026's sign-in configuration. Its `roles/iam.serviceAccountUser` binding for the deploy identity is owned here too — the equivalent binding on the default compute account was made in a console and was never a resource at all, the same gap [#1091](https://github.com/markgoho/doula-cloud/issues/1091) closes for `terraform-plan@`. |
| The Workload Identity pool and provider | `google_iam_workload_identity_pool`, `google_iam_workload_identity_pool_provider` | The attribute condition on this provider — today `assertion.repository == 'markgoho/doula-cloud'` — is the single string that stops another GitHub repository from minting tokens for this project. It has never been reviewed in a diff because it has never been in a file. |
| Non-service-agent project IAM | `google_project_iam_member` | Google's own service agents (`service-…@gcp-sa-*`, `…@cloudservices`, `…@cloudbuild`, and the handful of other Google-managed accounts holding an equivalent platform role) are excluded: the platform creates and repairs them, and importing them means Terraform proposing to delete a binding Google will immediately recreate. Everything else — the GitHub Actions account, the Firebase accounts, the default compute account, `terraform-plan@`, and `markgoho@gmail.com`'s `roles/owner` — is owned. |
| Artifact Registry `api` | `google_artifact_registry_repository` | The repository, not the images in it. Same split as Cloud Run: Terraform owns the shape, CI owns the contents. |
| `doula-cloud-pg` instance settings and databases | `google_sql_database_instance`, `google_sql_database` | Tier, disk, backup schedule, retention, SSL mode and deletion protection. `docs/environment.md` describes none of these today, so nothing would notice if a backup schedule were turned off. |
| `doula-cloud-attachments`, and the runtime account's grant on it | `google_storage_bucket`, `google_storage_bucket_iam_member` | The one project-owned bucket. It carries uniform bucket-level access and enforced public-access prevention, and no lifecycle rule or versioning; nothing in the repository records any of that, so nothing would notice it being relaxed. #1051 added the grant: `roles/storage.objectUser` for `doula-api-runtime@`. The bucket's remaining bindings are the `projectEditor`/`projectOwner`/`projectViewer` legacy convenience roles GCS creates with every bucket, plus `terraform-plan@`'s own reader grant, and none of those is owned. |

## What stays by hand

Nothing here is an oversight. Each line is a decision, and the reason is the point of the line.

| Resource | Why it is not in Terraform |
| --- | --- |
| Secret **values** (every `google_secret_manager_secret_version`) | A version's payload is stored in Terraform state in plaintext. Putting the Stripe key, the Mailgun key and three database DSNs in state trades one console step for a file that is worth more than the console. The shells are owned; the values are added with `gcloud secrets versions add`, as `docs/environment.md` already describes. |
| `doula-api`'s container image | `ci.yml`'s `deploy-api` sets it on every merge to trunk. See the next section — this is the one conflict that decides whether any of this works. |
| Cloud SQL users and their passwords | `google_sql_user` writes the password to state. `app_runtime_login` and `site_builder_login` are created by hand and their DSNs live in Secret Manager. The instance and the databases are owned; the logins are not. |
| Identity Platform configuration | `signIn.email`, the MFA settings, the authorized domains. The Terraform provider covers `google_identity_platform_config` unevenly, and ADR-0026's two sign-in methods are a product decision walked by hand once. Revisit only if it drifts. |
| Firebase Hosting, Firestore, and the Firebase-created buckets | `firebase.json` and `.firebaserc` are already code and already deploy from CI; `doula-cloud.firebasestorage.app` and the `(default)` Firestore database were created by Firebase enabling itself and are managed by it. Adding a second owner would produce drift, not detect it. |
| `firebase-app-hosting-github-oauth-github-oauthtoken-a16322` | Created by Firebase App Hosting's Developer Connect integration for its own GitHub connection, not by Doula Cloud (#1046). Its only accessor is `service-850855848778@gcp-sa-devconnect.iam.gserviceaccount.com`, a Google service agent — already excluded from ownership by the row above, so importing the shell alone would own a resource whose one real grant Terraform can never manage, the same half-configuration this ticket exists to avoid on the twelve secrets it does own. Same reasoning as the Firebase-created buckets and Firestore: Developer Connect creates and rotates this secret for itself, and a second owner would read as drift, not detect it. |
| Google-created service accounts and service agents | `850855848778-compute@`, `firebase-adminsdk-rq3g0@`, `firebase-app-hosting-compute@`, and every `service-…@gcp-sa-*` principal. The platform creates, grants and repairs these; Terraform proposing to remove a binding Google recreates makes `plan` permanently red, which is the failure mode this whole effort is trying to avoid. Since #1051 the default compute account holds **no project role and no grant anywhere in this project** — nothing runs as it — so there is nothing left here for Terraform to own even in principle. It cannot be deleted: Google recreates a project's default compute account, so an empty policy is the end state, not removal. |
| Artifact Registry `cloud-run-source-deploy`, `firebaseapphosting-images` | Both are created by a tool for its own use — `gcloud run deploy --source` and Firebase App Hosting. `gcf-artifacts` held the two functions' build images below and was deleted with them, 2026-09-08 (#1043). |
| The `terraform-plan@` service account itself | Created by hand in #1044, before any Terraform existed, for the same reason the state bucket is: the plan identity has to exist before a plan can run as it. Its three project roles **are** owned, as `google_project_iam_member` resources (#1047), so a grant silently removed from it is still a red plan. The account shell is not yet a resource, which is a real gap rather than a decision — importing it is [#1091](https://github.com/markgoho/doula-cloud/issues/1091). |
| The Terraform state bucket | It has to exist before Terraform runs. Created once by hand; the commands are in the [State](#state) section so the creation is repeatable even though it is not applied. |
| The GCS buckets Google made for builds | `run-sources-…`. Build scratch space, recreated on demand. `gcf-v2-sources-…` and `gcf-v2-uploads-…` were deleted 2026-09-08 (#1043) along with the two Cloud Functions that used them. |
| Mailgun, Stripe, GitHub | Outside GCP entirely. `docs/environment.md` is where these are described, and this specification does not move them. |
| goose migrations | Already code, already run by `ci.yml`'s `migrate` job. Terraform owns the instance, never the schema. |

## The Cloud Run image conflict

This is the question that decides whether adopting Terraform is worth anything. `ci.yml`'s `deploy-api` runs `google-github-actions/deploy-cloudrun@v3` on every merge to trunk. If Terraform also owns what that action writes, `plan` is red after every merge, forever, nobody reads it, and the result is worse than not adopting Terraform at all.

**The mutation set is not a guess.** Two consecutive real CI deploys — `doula-api-00491-lrt` (trunk `26eab783`) and `doula-api-00492-b96` (trunk `c9097a3f`), eight minutes apart on 2026-09-09 — differ in exactly these places, and nowhere else:

- `spec.template.containers[0].image` — the tag, and with it the resolved digest
- the `commit-sha` label, at both the service and the revision-template level
- `client.knative.dev/nonce`, a per-revision label
- the revision's own name, `configurationGeneration`, `uid`, `resourceVersion` and timestamps

Everything else is byte-identical across the two: all 19 environment variables and secret references, the service account, `containerConcurrency`, the CPU and memory limits, the startup probe, `--min-instances=1`, the Cloud SQL instance attachment, and the `run.googleapis.com/client-name` and `client-version` annotations that record which tool wrote the revision. That is the useful result. Of the 19 values, `deploy-cloudrun` writes exactly two — `DATABASE_URL` through its `secrets:` input and `EXPECTED_ORIGINS` through `env_vars:` — and it is configured `env_vars_update_strategy: merge`, with `secrets:` merging by default, so the other seventeen are untouched by every deploy. Terraform can own all nineteen without ever contending with CI: the two the action writes are written back to the same values Terraform declares.

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

**Only two of the mutations reach a field Terraform declares**, and the list has four entries because one of those two is a pair. The image is one. The `commit-sha` label on the *revision template* is the other. Everything else needs no entry: the revision name, generation and timestamps are computed attributes the provider does not diff; `client.knative.dev/nonce` is a system label the v2 API does not expose at all; and the `commit-sha` label at the *service* level is not diffed either, even though `gcloud` shows it changing, because the v2 provider splits labels into `labels` (what the configuration declares, empty here), `terraform_labels` (also empty) and `effective_labels` (what is live, which does carry `commit-sha`) and compares only the first two — read out of the imported state rather than assumed.

`client` and `client_version` are the block's other two entries, and they are a precaution rather than observed drift: both deploys wrote `gcloud` / `568.0.0` and the plan was clean without them. They record which tool last wrote the service, so they move the day `setup-gcloud` picks up a new version — a difference that says nothing about the project's configuration and would make the drift job red for no reason. A narrower resource was considered and rejected — the v2 provider has no "service configuration without the image" resource, and splitting the service across two resources means two owners for one API object, which is the problem being solved rather than a solution to it.

**Verified, not asserted.** Terraform 1.16.1 with `hashicorp/google` v7, a throwaway configuration and local state, run against the live project:

1. `terraform plan -generate-config-out` against an `import` block for `projects/doula-cloud/locations/us-central1/services/doula-api` generated a 244-line resource from the live service and reported `1 to import, 0 to add, 0 to change, 0 to destroy`.
2. `terraform apply` performed the import — **import writes state only; no `apply` in this exercise ever changed the project.**
3. `terraform plan` against the imported state: *No changes. Your infrastructure matches the configuration.*
4. The configuration was then edited to hold a different image tag, a different `commit-sha` template label and a different `client_version` — the three declared fields the block covers — and planned twice. **Without** the `lifecycle` block: `Plan: 0 to add, 1 to change, 0 to destroy`, naming those three and nothing else. **With** it: *No changes.* That is both halves of the claim — the plan is genuinely sensitive to those fields, and the block is what makes it ignore them.
5. The same plan re-run after a real trunk deploy, which is the one step a hand-edited configuration cannot stand in for. This specification's own merge was that deploy — `362a1a17`, CI run 34299820218 green on all eight jobs, revision `doula-api-00495-s9f` — and the plan against the untouched spike configuration reported exactly two drifted attributes, the image and the `commit-sha` template label, and nothing else in the resource. With the `lifecycle` block: *No changes.* `client` and `client_version` did not move, which is the observation behind calling them a precaution.

Every command and every plan output is on #797 — [steps 1 through 4](https://github.com/markgoho/doula-cloud/issues/797#issuecomment-5594346220), including the two things the spike settled that were not obvious from the API surface, and [step 5](https://github.com/markgoho/doula-cloud/issues/797#issuecomment-5594502718). #1044 re-runs the whole sequence, step 5 included, against the committed configuration rather than trusting the spike.

One thing the generated configuration turned up that the boundary above now accounts for: the live service still carries a `build_config` block pointing at `gs://run-sources-doula-cloud-us-central1/services/doula-api/1786507208…zip`, left behind by a long-past `gcloud run deploy --source`. It is inert and does not drift today — every deploy since has been by image reference, and the plan above was empty with it declared as-is — but it is exactly the kind of residue an import pass exists to surface, and a single future `--source` deploy would turn it red. #1044 declared it as-is and added `build_config` to `ignore_changes` rather than clearing it: there is no `gcloud` flag to clear this field, only a raw Cloud Run Admin API `PATCH` with an explicit update mask, and making that call would be an out-of-band mutation of the live service outside the import-only-until-the-plan-is-empty mechanism this whole effort depends on. The committed `ignore_changes` list therefore has five entries, not the four shown in the illustrative block above: this one, for a reason unrelated to the deploy conflict that block explains.

## Who may apply, and what CI may read

**`plan` runs in CI. `apply` runs from a laptop, by the one person who has `roles/owner`.** The reason is not ceremony. Applying this configuration needs IAM-admin-class permission — `roles/iam.serviceAccountAdmin`, `roles/resourcemanager.projectIamAdmin`, `roles/iam.workloadIdentityPoolAdmin`, `roles/secretmanager.admin`, `roles/cloudsql.admin`. A principal holding those in GitHub Actions can grant itself anything in the project, and the thing it would authenticate through is the Workload Identity provider that the same configuration owns. For a business with one person in it, the convenience of `apply` on merge does not pay for that.

`apply` therefore runs interactively, from the main checkout, as `markgoho@gmail.com` over `gcloud auth application-default login`. No service account holds apply-class permission at all, so there is no credential to leak.

**What CI holds is read.** A second service account, `terraform-plan@doula-cloud.iam.gserviceaccount.com`, authenticates the same way `deploy-api` does, through the existing `github-actions` Workload Identity pool, and holds only enough to refresh state and diff:

| Role | Why `plan` needs it |
| --- | --- |
| `roles/viewer` | Reads almost everything the configuration covers: the Cloud Run service, the Scheduler jobs, the Tasks queue, the Cloud SQL instance and databases, the Artifact Registry repository, the secret shells, and the project IAM policy |
| `roles/iam.securityReviewer` | The `getIamPolicy` calls `roles/viewer` does not carry — confirmed missing for `cloudtasks.queues`, `storage.buckets` and `artifactregistry.repositories`, all three present here |
| `roles/iam.workloadIdentityPoolViewer` | `roles/viewer` carries no `iam.workloadIdentityPools.get`; without it the pool and provider resources cannot refresh |
| `roles/storage.legacyBucketReader` | `roles/viewer` carries no `storage.buckets.get` either, so `doula-cloud-attachments` cannot refresh without it. Granted on that bucket alone; the state bucket is a separate grant, in [State](#state) |
| `roles/secretmanager.secretAccessor`, **granted on one secret, not the project** | `doula-cloud-notification-worker-secret` only, so the Scheduler jobs' `X-Internal-Secret` header can come from a data source rather than a literal in the repository. Both jobs carry the same value, verified against the live jobs. Every other secret stays unreadable to CI |

That list is derived from `gcloud iam roles describe`, not from memory, but it is a starting point and not a finding: the bootstrap sub-issue (#1044) runs `plan` as that account and adds whatever a real refresh turns out to need, because a permission gap surfaces as a failed refresh rather than as a wrong answer. Against `doula-api`, the single resource #1044 imports, it held on the first attempt — no gap surfaced. `terraform-plan@` also carries `roles/storage.objectUser` on `doula-cloud-tfstate` alone (not part of this table; it is the [State](#state) section's separate "who can read state" grant, needed for the GCS backend itself rather than for refreshing any one resource), `roles/iam.workloadIdentityUser` for the same `github-actions` pool principal set `deploy-api` uses, and — for running `plan` from a laptop under impersonation rather than through CI — `roles/iam.serviceAccountTokenCreator` for `markgoho@gmail.com`, scoped to this one service account. This table will be revisited as #1045 through #1048 exercise the rest of it (the Scheduler jobs, the Tasks queue, the secret shells, project IAM, and the Cloud SQL instance).

**A red `plan` is a red required check on trunk, not an advisory annotation.** ADR-0034 has the argument. A drift job nobody has to act on is a drift job nobody reads, which is where documentation already failed. The job itself is `.github/workflows/terraform-plan.yml` (#1049): required on trunk, on every pull request, and on a daily schedule, since a console change produces no commit and therefore no push to trigger anything. A red run means the live project no longer matches this configuration — the job summary names which resource and which attributes drifted (never their values), and the fix is either a commit or a laptop `apply`, never a dismissal. A red run also blocks every other pull request until someone reconciles it or a maintainer relaxes the ruleset; that cost is accepted deliberately (see #1049) rather than solved by making the check advisory.

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

**The danger is not a wrong apply. It is a half-written configuration and a right-looking apply.** Terraform deletes what it manages and no longer sees. A configuration that covers eleven of the thirteen secrets, or the Cloud Run service without its Cloud SQL attachment, produces a plan that reads as reasonable and takes the live API down. There are no users yet, and the January 2027 target is what makes that survivable — but the database is real, and `doula-cloud-pg` had `deletionProtectionEnabled: false` before mechanism 3 below turned it on.

Four mechanisms, in this order:

1. **Import-only until the plan is empty.** No resource enters the configuration except by importing what already exists, and no `apply` runs that reports anything but `N to import, 0 to add, 0 to change, 0 to destroy`. An import-only apply cannot delete anything; it writes state. This is #1044's rule, and it is what the spike above already demonstrated for `doula-api` — generate, import, plan clean.
2. **`prevent_destroy` on everything that holds data or identity**, before the first apply, not after: the Cloud SQL instance, its databases, `doula-cloud-attachments`, every secret shell, and the Workload Identity provider. A `lifecycle { prevent_destroy = true }` block makes Terraform refuse the plan rather than execute it. Distinct from it, and easy to confuse with it: several resources carry their own `deletion_protection` **argument**, a Terraform-side refusal (not a GCP-level flag) that Terraform can also *turn off* in the same apply that then destroys the resource — the Cloud Run v2 resource defaults it to `true`, and `google_sql_database_instance` defaults it to `true` as well. `google_sql_database_instance` additionally carries a separate, nested `settings.deletion_protection_enabled` argument that does mirror the GCP-level flag — the one field `gcloud sql instances patch --deletion-protection` sets — and it is not the same field as the top-level one. All three fields — `deletion_protection`, `settings.deletion_protection_enabled`, and `lifecycle.prevent_destroy` — are set on `doula-cloud-pg` (#1048).
3. **Turn on Cloud SQL deletion protection.** `deletionProtectionEnabled` was `false` on the live instance before this step — a GCP-level protection independent of anything Terraform believes. Setting it is one `gcloud` command and it should not wait for the Terraform work; it is #1044's first step.
4. **One resource kind per sub-issue, each landing with an empty plan.** The configuration is never in a state where a plan covers half of a resource kind. Scheduler jobs land complete, then the Tasks queue lands complete, and so on.

`terraform destroy` is never run against this project, by anyone, for any reason.

## `redirecturl` and `shortenurl`

**They were deleted, not imported, on 2026-09-08 ([#1043](https://github.com/markgoho/doula-cloud/issues/1043)).** [#87](https://github.com/markgoho/doula-cloud/issues/87) is why this needed an answer rather than an assumption: infrastructure that looks unused is not safe to delete on that basis, and acting on appearance there caused a live incident. So here is the answer, written down.

**What they were.** Both were 2nd-generation Cloud Functions, not plain Cloud Run services — `goog-managed-by: cloudfunctions`, `run.googleapis.com/client-name: cli-firebase`, `cloudfunctions.googleapis.com/function-id: redirectUrl` and `shortenUrl`, both built on 2025-03-26 from a single Cloud Build (`fb1e636f-…`) producing `gcf-artifacts/doula--cloud__us--central1__shorten_url:version_1`. `gcloud functions list`, read before deletion, confirmed both as ACTIVE 2nd-gen HTTP-triggered functions. They were a Firebase Functions URL shortener deployed with `firebase deploy`.

**Whose they were.** Mark's account, [recorded on the ticket](https://github.com/markgoho/doula-cloud/issues/797#issuecomment-5594356337): a previous incarnation of this project, from before the current Svelte-and-Go stack existed. Nothing in this repository referenced either one — a full-tree search for `shortenurl`, `redirecturl`, `shorten_url`, `redirect_url` and `shortUrl` returned nothing outside `.git`.

**What depended on them: nothing.** Thirty days of Cloud Run request logs, re-read on 2026-09-08 immediately before deletion, against a 10,000-entry limit so nothing was truncated. An earlier reading of the same window undercounted — the figures below are the ones read fresh, immediately before deletion:

- `shortenurl`: **zero log entries of any kind.**
- `redirecturl`: 4,942 log entries — 4,575 HTTP requests plus 367 system/stderr lines — spanning eight days, 2026-08-10 through 2026-08-17, and nothing since. 4,566 of the 4,575 requests answered `404`; the other 9 answered `500`, all `GET /__debug__/` or `GET /wp-json/` from user agents impersonating a web crawler or identifying outright as a PHP-webshell scanner (`wp2shell`) — the function's own stderr log shows the 500s are a Firestore lookup crashing on a reserved document id, not a served response. Every requested path in the window is scanner fare: `/robots.txt`, then dozens of PHP web-shell filenames (`wp-login.php`, `admin.php`, `a.php`, and similar). There was no 2xx and no 3xx response anywhere in the window — a redirector that redirected nothing. [Full detail is on the ticket.](https://github.com/markgoho/doula-cloud/issues/1043#issuecomment-5594847923)
- Both carried `allUsers` → `roles/run.invoker`, so both were open to the internet. What the logs show is a public attack surface for a service nobody used.

**What went with them.** Deleting the functions made four things removable, each confirmed empty or unreferenced, also on 2026-09-08:

- The `gcf-artifacts` Artifact Registry repository — confirmed empty, then deleted.
- `gcf-v2-sources-850855848778-us-central1` and `gcf-v2-uploads-850855848778.us-central1.cloudfunctions.appspot.com` — confirmed to hold only build-scratch `function-source.zip` objects tied to the two deleted functions (plus one unrelated leftover from an already-gone function), then deleted.
- `roles/cloudfunctions.developer` on `github-action-733741680@` and `roles/cloudfunctions.admin` on `firebase-adminsdk-rq3g0@` — removed from the project's IAM policy before this change merged to trunk, so the trunk CI run the merge produced is itself the evidence the GitHub Actions deploy identity did not need the role.

The `(default)` Firestore database stayed: Firebase created it and the shortener's use of it, if any, was not what kept it there.

**Removal was `gcloud functions delete redirectUrl` and `gcloud functions delete shortenUrl`, in `us-central1`** — deleting the underlying Cloud Run service instead would have left the Cloud Functions resource behind. It was independent of every Terraform decision above and did not wait on any of it; it was #1043, first in the sequence, for that reason. Nothing else in the project was touched.

## What this specification found and did not fix

An import pass forces a question about every resource in the project, and asking those questions turned up five things that are not drift and are not this specification's job to fix. Each is filed rather than left here as prose.

- ~~**`doula-api` runs as `850855848778-compute@developer.gserviceaccount.com`, which holds `roles/editor` on the project.**~~ **Fixed by [#1051](https://github.com/markgoho/doula-cloud/issues/1051).** The service runs as `doula-api-runtime@doula-cloud.iam.gserviceaccount.com`. Its grants are each declared beside the resource they are granted on, so the list is `grep -rn doula_api_runtime terraform/` rather than a sentence that can go stale: `roles/cloudsql.client` and a three-permission custom role for Identity Platform accounts at the project level (neither service has resource-level IAM), a `secretAccessor` grant on each secret the service declares, `roles/cloudtasks.enqueuer` on `doula-cloud-notification-nudge`, and `roles/storage.objectUser` on `doula-cloud-attachments`. `ci.yml`'s `deploy-api` passes `--service-account`, so a deploy cannot fall back to the default account. See the "Two grants removed by hand" note below for what could not go through Terraform.
- **`doula-api` is `allUsers` → `roles/run.invoker`, and that includes every `/api/internal/**` endpoint.** The only thing standing between the public internet and `POST /api/internal/outboxes/drain` is the `X-Internal-Secret` header. That is a deliberate design (`docs/environment.md`), and it is also the whole of the boundary. → [Decide the boundary on `/api/internal/**`, and stop storing its secret in the clear on the Scheduler jobs #1052](https://github.com/markgoho/doula-cloud/issues/1052)
- **The Scheduler jobs carry that same secret as a plaintext header value in the job spec**, readable by anything with `cloudscheduler.jobs.get`, which `roles/viewer` grants. → covered by the same ticket as the line above.
- **`doula-cloud-pg` has `deletionProtectionEnabled: false`.** → the first step of the bootstrap sub-issue, not a separate ticket.
- **`doula-api` carries a stale `build_config` from a long-past `gcloud run deploy --source`.** Inert today; a single future `--source` deploy makes the drift job red for a reason that has nothing to do with drift. → handled in the bootstrap sub-issue: declared as-is and added to `ignore_changes`, not cleared — see the paragraph above.

### Two grants removed by hand (#1051)

Terraform removed twelve bindings on the default compute account — project `roles/editor`, its ten secret `secretAccessor` grants, and `roles/cloudtasks.enqueuer` on the nudge queue — because #1046 and #1047 had imported all twelve. Two more of that account's grants existed and were **not** in state, so `terraform destroy`-by-omission could not reach them and they were removed with `gcloud` instead:

- `roles/iam.serviceAccountUser` on `850855848778-compute@` itself, held by `github-action-733741680@`. A service-account-level binding, which no project-level resource covers. It is what let `deploy-api` deploy a service running as that account; the equivalent binding on `doula-api-runtime@` **is** a resource (`google_service_account_iam_member`), so the same gap does not reopen.
- `roles/storage.objectUser` on `gs://doula-cloud-attachments`, held by `850855848778-compute@`. #1048 imported the bucket but not its IAM, so this grant was invisible to `plan`. The replacement grants on `doula-api-runtime@` are both resources in `terraform/storage.tf`.

Both are the same shape as [#1091](https://github.com/markgoho/doula-cloud/issues/1091): a binding that exists, matters, and is watched by nothing. Neither was drift — each was live and correct for the configuration that preceded it — but neither would have shown up in a diff if someone had added it yesterday.

One unrelated thing, found while numbering ADR-0034: **two ADRs both number themselves 0033** — `0033-overdue-is-derived-and-notifies-nobody.md` and `0033-staff-login-deletion-is-immediate-and-redacts-the-person.md`. Renaming either one breaks existing links, so it is not done here. → [Two ADRs both number themselves 0033, so the citation names two decisions #1053](https://github.com/markgoho/doula-cloud/issues/1053)

## The build, as sub-issues

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
