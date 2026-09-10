# Imported from live state (docs/infrastructure.md, #1046). #797's boundary
# owns the shell of each Secret Manager secret — the container, its
# replication policy, and its accessor grants — and never the version.
#
# No `google_secret_manager_secret_version` resource exists in this
# configuration, and none ever should: a version's payload is the secret
# value itself, and Terraform state is plaintext. Owning a version here would
# put the Stripe key, the two Mailgun keys, and three Postgres DSNs into a
# file that anyone with read access to `gs://doula-cloud-tfstate` could read
# in the clear. Values are added by hand with `gcloud secrets versions add`,
# per docs/infrastructure.md's by-hand table.
#
# `prevent_destroy = true` is on every secret below: each one holds identity
# (a Stripe key, a Mailgun key, a database DSN, a signing key) that nothing
# here should ever destroy, same reasoning as `doula-api` in `cloud_run.tf`.
#
# `deletion_protection` is left at the generated `false` rather than flipped
# to `true`: flipping it would be a live change to the resource, and this
# ticket is import-only until the plan is empty — `terraform plan` must
# report imports and nothing else. Turning it on is a real (if narrow) apply,
# out of scope here.
#
# The accessor grant is the point of #743: a secret that exists but is not
# granted to the identity reading it fails at container start, silently,
# until that moment. Every grant below is its own real, imported
# `google_secret_manager_secret_iam_member` resource, read from `gcloud
# secrets get-iam-policy` rather than assumed, and kept next to the secret it
# grants access to — a secret imported without its grants would be exactly
# the half-configuration this ticket exists to close.
#
# `firebase-app-hosting-github-oauth-github-oauthtoken-a16322` is not here.
# See docs/infrastructure.md's by-hand table for why.
#
# Every `*_runtime_accessor` below names `doula-api-runtime@` since #1051.
# Each was the default compute account before that. These ten grants were
# the real boundary even then, and still are: `roles/editor`, which that
# account held over everything else in the project, deliberately excludes
# `secretmanager.versions.access`, so a secret without a grant here was
# unreadable to the container no matter what else it could do. #743 walked
# exactly that failure. Ten, not eleven: the live service declares ten
# `secret_key_ref` environment variables and nine plain ones (cloud_run.tf),
# which is the count that decides this list.

resource "google_secret_manager_secret" "github_dispatch_token" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-github-dispatch-token"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "github_dispatch_token_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.github_dispatch_token.id
}

resource "google_secret_manager_secret" "mailgun_api_key" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-mailgun-api-key"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "mailgun_api_key_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.mailgun_api_key.id
}

resource "google_secret_manager_secret" "mailgun_webhook_signing_key" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-mailgun-webhook-signing-key"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "mailgun_webhook_signing_key_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.mailgun_webhook_signing_key.id
}

resource "google_secret_manager_secret" "notification_worker_secret" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-notification-worker-secret"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

# Three accessors, not one: the runtime container itself, `deploy-api` (which
# writes the header value onto the Scheduler jobs at deploy time — see
# docs/environment.md), and `terraform-plan@`, added in #1044 so the `data`
# source in `scheduler.tf` can read this secret during `plan`.
resource "google_secret_manager_secret_iam_member" "notification_worker_secret_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.notification_worker_secret.id
}

resource "google_secret_manager_secret_iam_member" "notification_worker_secret_deploy_accessor" {
  member    = "serviceAccount:github-action-733741680@doula-cloud.iam.gserviceaccount.com"
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.notification_worker_secret.id
}

resource "google_secret_manager_secret_iam_member" "notification_worker_secret_terraform_plan_accessor" {
  member    = "serviceAccount:terraform-plan@doula-cloud.iam.gserviceaccount.com"
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.notification_worker_secret.id
}

resource "google_secret_manager_secret" "pg_app_runtime_dsn" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-pg-app-runtime-dsn"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    user_managed {
      replicas {
        location = "us-central1"
      }
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "pg_app_runtime_dsn_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.pg_app_runtime_dsn.id
}

# No accessor grant: `gcloud secrets get-iam-policy doula-cloud-pg-migrate-dsn`
# returns an empty binding set. This DSN is not read by `doula-api`'s own
# container, so there is nothing to import here beyond the shell.
resource "google_secret_manager_secret" "pg_migrate_dsn" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-pg-migrate-dsn"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    user_managed {
      replicas {
        location = "us-central1"
      }
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}

# No accessor grant: same as `pg_migrate_dsn` above, `gcloud secrets
# get-iam-policy doula-cloud-pg-site-builder-dsn` returns an empty binding set.
resource "google_secret_manager_secret" "pg_site_builder_dsn" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-pg-site-builder-dsn"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret" "stripe_account_webhook_secret" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-stripe-account-webhook-secret"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "stripe_account_webhook_secret_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.stripe_account_webhook_secret.id
}

resource "google_secret_manager_secret" "stripe_api_key" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-stripe-api-key"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "stripe_api_key_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.stripe_api_key.id
}

resource "google_secret_manager_secret" "stripe_connect_webhook_secret" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-stripe-connect-webhook-secret"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "stripe_connect_webhook_secret_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.stripe_connect_webhook_secret.id
}

resource "google_secret_manager_secret" "stripe_webhook_secret" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-stripe-webhook-secret"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "stripe_webhook_secret_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.stripe_webhook_secret.id
}

resource "google_secret_manager_secret" "vapid_private_key" {
  annotations         = {}
  deletion_policy     = "DELETE"
  deletion_protection = false
  labels              = {}
  project             = "doula-cloud"
  secret_id           = "doula-cloud-vapid-private-key"
  tags                = null
  ttl                 = null
  version_aliases     = {}
  version_destroy_ttl = null

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret_iam_member" "vapid_private_key_runtime_accessor" {
  member    = google_service_account.doula_api_runtime.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.vapid_private_key.id
}
