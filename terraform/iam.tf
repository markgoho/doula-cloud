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
# reviewable is not the same act as narrowing it. One of the two is gone:
# #1051 removed project `roles/editor` from the default compute account and
# gave `doula-api` the dedicated runtime identity below, so the account that
# used to run the container now holds no role in this project at all. The
# other, `github_action_secretmanager_secret_accessor`, is still here — the
# deploy identity can read all thirteen secrets instead of the one `ci.yml`
# reads, which is #1078.

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

# `github-action-733741680@`'s nine project roles (ten until #1043 removed
# `roles/cloudfunctions.developer`).
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

# #1078: project-wide read access to all thirteen secrets, not the one
# `ci.yml` reads. Imported as it stands; narrowing it is #1078's job.
resource "google_project_iam_member" "github_action_secretmanager_secret_accessor" {
  member  = google_service_account.github_action.member
  project = "doula-cloud"
  role    = "roles/secretmanager.secretAccessor"
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
# whole-project compromise: every secret readable, every Cloud Run service
# and the Cloud SQL instance modifiable, every bucket writable. That account
# now holds no role in this project at all.
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
#     (scheduler.tf) — ADR-0013's nudge path. The task carries
#     `X-Internal-Secret` as a plain header and no OIDC token, so nothing
#     here needs to act as any account.
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
