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
# Two of the grants below are over-broad and imported exactly as they stand,
# per this ticket's own scope: `default_compute_editor` is #1051 (the
# runtime container holds project `roles/editor` instead of four narrow
# grants), and `github_action_secretmanager_secret_accessor` is #1078 (the
# deploy identity can read all thirteen secrets instead of the one `ci.yml`
# uses). Making them reviewable is this ticket's job; narrowing them is not.

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

# `850855848778-compute@`: the Google-created default compute service
# account that `doula-api` runs as (cloud_run.tf). #1051: this should be a
# dedicated runtime service account with four narrow grants instead of
# project `roles/editor`. Imported as it stands; narrowing it is #1051's job.
resource "google_project_iam_member" "default_compute_editor" {
  member  = "serviceAccount:850855848778-compute@developer.gserviceaccount.com"
  project = "doula-cloud"
  role    = "roles/editor"
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
