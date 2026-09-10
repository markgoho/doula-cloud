# Imported from live state (docs/infrastructure.md, #1047). #797's boundary
# owns the deploy identity, the non-service-agent project IAM, the Workload
# Identity federation that authenticates it, and the Artifact Registry
# repository `api`.
#
# `google_project_iam_policy` appears nowhere in this configuration, and
# never should: it is authoritative, replacing the whole project IAM policy
# with exactly what it declares. Google's own service agents — every
# `service-…@gcp-sa-*` principal, `…@cloudservices`, `…@cloudbuild`, and the
# handful of other Google-managed accounts holding a `*ServiceAgent` or
# equivalent platform role (`gcf-admin-robot`, `containerregistry`,
# `serverless-robot-prod`, `firebase-rules`, `firebase-sa-management`,
# `gcp-sa-devconnect`'s `secretmanager.admin`) — are created, granted and
# repaired by the platform, not by anyone here. A `google_project_iam_policy`
# would propose to delete every one of those bindings on the next `plan`,
# Google would recreate them immediately, and the drift check would be red
# forever — the exact failure ADR-0034 refuses. `google_project_iam_member`
# is one resource per principal-role pair, so each pair below is owned or
# excluded on its own, and Google's bindings are simply never mentioned.
#
# The import pass (#1047) brought in two grants that were over-broad and
# declared them exactly as they stood, on the argument that making a grant
# reviewable is not the same act as narrowing it. Both are now gone. #1051
# removed project `roles/editor` from the default compute account and gave
# `doula-api` the dedicated runtime identity below, so the account that used
# to run the container now holds no role in this project at all. #1078
# removed project `roles/secretmanager.secretAccessor` from the deploy
# identity: it reached all thirteen secrets in the project, and CI reads two
# of them, so the two became `google_secret_manager_secret_iam_member` grants
# in `secrets.tf`, next to the secrets they are granted on. That role no
# longer appears in this project's IAM policy at all — the deploy identity
# was its only project-level member.
#
# #1078's apply had to be staged, and a future one of the same shape does
# too. Deleting a `google_project_iam_member` and adding the narrower grants
# elsewhere gives Terraform no ordering to respect: the removed resource is
# no longer in the configuration, so neither `depends_on` nor
# `create_before_destroy` can express "grant first." A single apply that
# happened to destroy first would leave a window in which a trunk push could
# not read its migration DSN. It was applied as `-target` on the two
# `*_deploy_accessor` resources, then a full apply for the destroy.

# `github-action-733741680@`: the identity every deploy in `ci.yml` runs as.
resource "google_service_account" "github_action" {
  account_id                   = "github-action-733741680"
  create_ignore_already_exists = null
  deletion_policy              = "DELETE"
  description                  = "A service account with permission to deploy to Firebase Hosting and Cloud Functions for the GitHub repository markgoho/doula-cloud"
  disabled                     = false
  display_name                 = "GitHub Actions (markgoho/doula-cloud)"
  project                      = "doula-cloud"

  lifecycle {
    prevent_destroy = true
  }
}

# `github-action-733741680@`'s eight project roles (ten until #1043 removed
# `roles/cloudfunctions.developer`, nine until #1078 removed
# `roles/secretmanager.secretAccessor`). Whether any of the eight is itself
# wider than a deploy needs is a question #1078 deliberately did not open;
# it answered only the one role that had a per-resource scope to move to and
# a knowable, two-item list of resources to move to it.
resource "google_project_iam_member" "github_action_artifactregistry_writer" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/artifactregistry.writer"
}

resource "google_project_iam_member" "github_action_cloudsql_client" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/cloudsql.client"
}

resource "google_project_iam_member" "github_action_firebaseauth_admin" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/firebaseauth.admin"
}

resource "google_project_iam_member" "github_action_firebasehosting_admin" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/firebasehosting.admin"
}

resource "google_project_iam_member" "github_action_run_admin" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/run.admin"
}

resource "google_project_iam_member" "github_action_run_viewer" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/run.viewer"
}

resource "google_project_iam_member" "github_action_serviceusage_api_keys_viewer" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/serviceusage.apiKeysViewer"
}

resource "google_project_iam_member" "github_action_serviceusage_service_usage_consumer" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/serviceusage.serviceUsageConsumer"
}

# `firebase-adminsdk-rq3g0@` and `firebase-app-hosting-compute@`: Firebase's
# own project-scoped service accounts (docs/infrastructure.md, "Service
# accounts" row) — their project IAM is owned, but neither is a
# `google_service_account` resource here: both are created by Firebase
# enabling itself, not by anyone in this project.
resource "google_project_iam_member" "firebase_adminsdk_sdk_admin_agent" {
  member  = "serviceAccount:firebase-adminsdk-rq3g0@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/firebase.sdkAdminServiceAgent"
}

resource "google_project_iam_member" "firebase_adminsdk_service_account_token_creator" {
  member  = "serviceAccount:firebase-adminsdk-rq3g0@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/iam.serviceAccountTokenCreator"
}

resource "google_project_iam_member" "firebase_adminsdk_storage_admin" {
  member  = "serviceAccount:firebase-adminsdk-rq3g0@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/storage.admin"
}

resource "google_project_iam_member" "firebase_app_hosting_compute_sdk_admin_agent" {
  member  = "serviceAccount:firebase-app-hosting-compute@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/firebase.sdkAdminServiceAgent"
}

resource "google_project_iam_member" "firebase_app_hosting_compute_developerconnect_read_token_accessor" {
  member  = "serviceAccount:firebase-app-hosting-compute@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/developerconnect.readTokenAccessor"
}

resource "google_project_iam_member" "firebase_app_hosting_compute_apphosting_compute_runner" {
  member  = "serviceAccount:firebase-app-hosting-compute@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/firebaseapphosting.computeRunner"
}

# `doula-api-runtime@`: the identity the `doula-api` container runs as
# (cloud_run.tf), created by #1051. Before it, `doula-api` ran as the
# Google-created default compute service account, which held project
# `roles/editor` — so a remote-code-execution bug in the BFF was a
# whole-project compromise: every Cloud Run service and the Cloud SQL
# instance modifiable, every bucket writable, and every one of those
# reachable without touching this configuration. Secret *payloads* were the
# one thing it could not reach — `roles/editor` excludes
# `secretmanager.versions.access` — which is why the per-secret grants in
# secrets.tf mattered even then. That account now holds no role in this
# project at all.
#
# Every grant this account holds is derived from what `api/main.go` and the
# packages it constructs actually call. Each grant is written next to the
# resource it is granted on, so the authoritative list is the set of
# resources naming `google_service_account.doula_api_runtime`, not this
# comment — `grep -rn doula_api_runtime terraform/` is the honest inventory:
#
#   - `roles/cloudsql.client`, below. Project level because Cloud SQL
#     exposes no instance-level IAM. It carries `cloudsql.instances.connect`
#     and nothing that can change the instance.
#   - The Identity Platform custom role, below. Also project level, because
#     Identity Platform has no per-resource IAM at all.
#   - `roles/secretmanager.secretAccessor` on the secrets the service
#     declares as `value_source.secret_key_ref` (secrets.tf), one grant per
#     secret. The Cloud Run runtime resolves those references as this
#     account, so a missing grant is a container that will not start.
#   - `roles/cloudtasks.enqueuer` on `doula-cloud-notification-nudge`
#     (scheduler.tf) — ADR-0013's nudge path. Since #1183 the task carries
#     an OIDC token minted for this same account rather than a plain header,
#     which is why `roles/iam.serviceAccountUser` on itself is below: Cloud
#     Tasks checks that the creating identity may act as the account the
#     token names, and a service account has no implicit `actAs` on itself.
#   - `roles/storage.objectUser` on `doula-cloud-attachments` (storage.tf).
#
# Nothing for logging or monitoring: Cloud Run collects a container's
# stdout/stderr and its request logs at the platform level, not as the
# runtime identity, and `api/` constructs no logging or monitoring client.
# Nothing for Artifact Registry either: Cloud Run pulls the image as its own
# service agent, not as this account.

# Identity Platform has no resource-level IAM — a grant is project-wide or it
# does not exist — so the only way to narrow it is to narrow the permissions
# themselves. `roles/firebaseauth.admin`, the obvious predefined choice and
# the one `github-action-733741680@` holds, also carries
# `firebaseauth.configs.*`: the power to change sign-in providers, MFA
# enforcement and authorized domains, which is ADR-0026's product decision
# and not something the container should be able to rewrite.
#
# These three permissions are exactly what `authn.FirebaseVerifier`'s
# `*auth.Client` calls: `GetUser`, `GetUserByEmail` and `GetUsers` (get);
# `UpdateUser` for the email-verified flag, a password set, an address
# change and an MFA reset (update); and `DeleteUser` (delete). Adding a
# fourth admin call to `api/` means adding its permission here, which is the
# point. `VerifyIDToken` itself is not in this list and needs nothing: it
# fetches Google's public signing certificates over plain HTTPS.
#
# This gap was the reason the switch below could not have been made blind.
# It was invisible while the container held `roles/editor`, and it is
# invisible in the Cloud Run configuration too — no environment variable, no
# secret reference, nothing but a Go client constructed at startup that does
# not fail until a person tries to change their own email address.
resource "google_project_iam_custom_role" "doula_api_staff_accounts" {
  description = "What doula-api's runtime identity does to Identity Platform accounts: read them, update them, delete them. Never the Identity Platform configuration itself (#1051)."
  permissions = [
    "firebaseauth.users.delete",
    "firebaseauth.users.get",
    "firebaseauth.users.update",
  ]
  project = "doula-cloud"
  role_id = "doulaApiStaffAccounts"
  stage   = "GA"
  title   = "doula-api staff accounts"
}
resource "google_service_account" "doula_api_runtime" {
  account_id                   = "doula-api-runtime"
  create_ignore_already_exists = null
  deletion_policy              = "DELETE"
  description                  = "Runtime identity for the doula-api Cloud Run service (#1051). Holds only what api/main.go uses; never roles/editor."
  disabled                     = false
  display_name                 = "doula-api runtime"
  project                      = "doula-cloud"

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_project_iam_member" "doula_api_runtime_cloudsql_client" {
  member  = google_service_account.doula_api_runtime.member
  project = "doula-cloud"
  role    = "roles/cloudsql.client"
}

resource "google_project_iam_member" "doula_api_runtime_staff_accounts" {
  member  = google_service_account.doula_api_runtime.member
  project = "doula-cloud"
  role    = google_project_iam_custom_role.doula_api_staff_accounts.id
}

# `deploy-api` in ci.yml passes `--service-account` on every deploy, and
# Cloud Run refuses a deploy whose caller cannot act as the runtime identity
# it names. The equivalent binding for the default compute account was made
# by hand and was never a Terraform resource at all — the same unowned-shell
# gap #1091 closes for `terraform-plan@` — which is why this one is declared
# here rather than left to a console.
resource "google_service_account_iam_member" "github_action_doula_api_runtime_service_account_user" {
  member             = google_service_account.github_action.member
  role               = "roles/iam.serviceAccountUser"
  service_account_id = google_service_account.doula_api_runtime.name
}

# `internal-caller@`: the identity the two Cloud Scheduler jobs present at
# `/api/internal/**` (#1183, ADR-0037). It holds **no project role**, and that
# is not an omission — the boundary is the `email` claim on the token it
# presents, checked against `INTERNAL_OIDC_CALLERS` inside the process, so the
# account needs nothing but to exist and to be mintable. A project role here
# would be power the boundary does not read.
#
# It is a separate account from `doula-api-runtime@` deliberately. The two
# callers arrive by different routes — a schedule Google runs, and this
# service enqueueing to itself — and one account per route is what makes a
# Cloud Run log line say which. Cloud Scheduler mints the token through its
# own service agent, `service-850855848778@gcp-sa-cloudscheduler…`, which
# holds `iam.serviceAccounts.getOpenIdToken` project-wide through
# `roles/cloudscheduler.serviceAgent`; the same is true of Cloud Tasks'
# agent. Read from `gcloud iam roles describe`, so neither agent needs a
# `roles/iam.serviceAccountTokenCreator` grant here.
resource "google_service_account" "internal_caller" {
  account_id                   = "internal-caller"
  create_ignore_already_exists = null
  deletion_policy              = "DELETE"
  description                  = "The identity the two Cloud Scheduler jobs present at /api/internal/** (#1183, ADR-0037). Holds no project role: the boundary is the token's email claim."
  disabled                     = false
  display_name                 = "Internal caller"
  project                      = "doula-cloud"

  lifecycle {
    prevent_destroy = true
  }
}

# `roles/owner` carries `iam.serviceAccounts.actAs` and not
# `iam.serviceAccounts.getOpenIdToken` — read from `gcloud iam roles describe
# roles/owner` — so the one human principal in this project cannot mint a
# token for this account without this grant, and every operator endpoint in
# `docs/runbooks/` is reached by minting one.
resource "google_service_account_iam_member" "markgoho_internal_caller_token_creator" {
  member             = "user:markgoho@gmail.com"
  role               = "roles/iam.serviceAccountTokenCreator"
  service_account_id = google_service_account.internal_caller.name
}

# Cloud Tasks refuses to create a task whose `oidc_token` names a service
# account the *creating* identity cannot act as, and a service account holds
# no implicit `actAs` on itself. `doula-api-runtime@` both creates the nudge
# and is named in its token, so it needs this binding on itself — without it
# every enqueue fails with a permission error rather than a 401, and ADR-0013's
# nudge silently stops while the five-minute drain keeps covering for it.
resource "google_service_account_iam_member" "doula_api_runtime_acts_as_itself" {
  member             = google_service_account.doula_api_runtime.member
  role               = "roles/iam.serviceAccountUser"
  service_account_id = google_service_account.doula_api_runtime.name
}

# The one human principal in the project.
resource "google_project_iam_member" "markgoho_owner" {
  member  = "user:markgoho@gmail.com"
  project = "doula-cloud"
  role    = "roles/owner"
}

# `terraform-plan@`, added in #1044: itself a service account with
# project-level roles, so it is in scope on the same argument as every other
# non-service-agent principal above — the account that runs the drift check
# is not an exception to the review it exists to provide.
resource "google_project_iam_member" "terraform_plan_viewer" {
  member  = "serviceAccount:terraform-plan@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/viewer"
}

resource "google_project_iam_member" "terraform_plan_security_reviewer" {
  member  = "serviceAccount:terraform-plan@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/iam.securityReviewer"
}

resource "google_project_iam_member" "terraform_plan_workload_identity_pool_viewer" {
  member  = "serviceAccount:terraform-plan@doula-cloud.iam.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/iam.workloadIdentityPoolViewer"
}

# The pool and provider `ci.yml`'s `deploy-api` and `terraform-plan@` both
# authenticate through. `prevent_destroy` is on both: losing either breaks
# every deploy and every scheduled `plan`.
resource "google_iam_workload_identity_pool" "github_actions" {
  deletion_policy           = "DELETE"
  description               = null
  disabled                  = false
  display_name              = "GitHub Actions"
  project                   = "doula-cloud"
  workload_identity_pool_id = "github-actions"

  lifecycle {
    prevent_destroy = true
  }
}

# `attribute_condition` is the highest-value string in this configuration:
# it is the only thing stopping a GitHub repository other than
# `markgoho/doula-cloud` from minting a token this pool will exchange for
# GCP credentials. It has never appeared in a diff before this import,
# because it has never been in a file.
resource "google_iam_workload_identity_pool_provider" "github" {
  attribute_condition = "assertion.repository == 'markgoho/doula-cloud'"
  attribute_mapping = {
    "attribute.ref"        = "assertion.ref"
    "attribute.repository" = "assertion.repository"
    "google.subject"       = "assertion.sub"
  }
  deletion_policy                    = "DELETE"
  description                        = null
  disabled                           = false
  display_name                       = "GitHub OIDC"
  project                            = "doula-cloud"
  workload_identity_pool_id          = "github-actions"
  workload_identity_pool_provider_id = "github"

  oidc {
    allowed_audiences = []
    issuer_uri        = "https://token.actions.githubusercontent.com"
    jwks_json         = null
  }

  lifecycle {
    prevent_destroy = true
  }
}

# The repository, never the images in it — the same split ADR-0034 draws for
# `doula-api` in cloud_run.tf. `cloud-run-source-deploy` and
# `firebaseapphosting-images` are not here: both are created by a tool for
# its own use (docs/infrastructure.md's by-hand table).
resource "google_artifact_registry_repository" "api" {
  cleanup_policy_dry_run = false
  deletion_policy        = "DELETE"
  description            = "doula-cloud api/ Docker images"
  format                 = "DOCKER"
  kms_key_name           = null
  labels                 = {}
  location               = "us-central1"
  mode                   = "STANDARD_REPOSITORY"
  project                = "doula-cloud"
  repository_id          = "api"

  vulnerability_scanning_config {
    enablement_config = null
  }
}
