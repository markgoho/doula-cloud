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
# until that moment. Every grant below is its own real
# `google_secret_manager_secret_iam_member` resource, kept next to the secret
# it grants access to — a secret imported without its grants would be exactly
# the half-configuration this ticket exists to close. Each of #743's was read
# from `gcloud secrets get-iam-policy` rather than assumed; the two #1078
# added were created by an apply rather than imported from a live binding.
#
# `firebase-app-hosting-github-oauth-github-oauthtoken-a16322` is not here.
# See docs/infrastructure.md's by-hand table for why.
#
# There are two kinds of accessor grant below, and the resource name says
# which. A `*_runtime_accessor` is `doula-api-runtime@` — the identity the
# container runs as — resolving a `secret_key_ref` at container start. A
# `*_deploy_accessor` is `github-action-733741680@` fetching a payload inside
# a GitHub Actions job, and there are exactly two of those: the migration DSN
# and the site builder DSN. Nothing else in CI reads a secret payload, and a
# secret that grows a third deploy accessor should be a question at review.
# Until #1078 the deploy identity read those two through a
# project-wide `roles/secretmanager.secretAccessor` that reached all thirteen
# secrets in the project, including the Stripe key and every Mailgun key, so
# neither secret carried a binding of its own and neither one's access
# boundary could be read off the resource. It holds no project-level
# secret role now, and that role is no longer in the project's IAM policy at
# all (`terraform/iam.tf`).
#
# Every `*_runtime_accessor` below names `doula-api-runtime@` since #1051.
# Each was the default compute account before that. These grants were
# the real boundary even then, and still are: `roles/editor`, which that
# account held over everything else in the project, deliberately excludes
# `secretmanager.versions.access`, so a secret without a grant here was
# unreadable to the container no matter what else it could do. #743 walked
# exactly that failure. There are nine, one per `secret_key_ref` environment
# variable the live service declares (cloud_run.tf) — that count, not the
# number of secrets in the project, is what decides this list. Ten until
# #1183 took `NOTIFICATION_WORKER_SECRET` off the service.

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

# This DSN is not read by `doula-api`'s own container. It is read by CI: the
# `migrate` job in `ci.yml` fetches it on every trunk push and hands it to
# `migrate.sh`. Until #1078 that read worked without any binding on the
# secret at all, because the deploy identity held project-wide
# `secretAccessor` — which is what made this secret's own IAM policy an empty
# set and its access boundary impossible to read off the resource.
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

# #1078. `_deploy_accessor` is the suffix `notification_worker_secret` carried
# for the same principal until #1183 removed it, so this reuses a name the file
# has already used rather than coining one.
resource "google_secret_manager_secret_iam_member" "pg_migrate_dsn_deploy_accessor" {
  member    = google_service_account.github_action.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.pg_migrate_dsn.id
}

# Read by CI, like `pg_migrate_dsn` above and by the same identity: the
# `build` job in `firebase-hosting-merge.yml` fetches it so #441's per-Practice
# page generator can reach the database through the Cloud SQL Auth Proxy. That
# job runs `SYNC_PRACTICE_PAGES=required`, so losing this grant is a failed
# site build rather than a site quietly published with no Practice pages in it.
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

resource "google_secret_manager_secret_iam_member" "pg_site_builder_dsn_deploy_accessor" {
  member    = google_service_account.github_action.member
  project   = "doula-cloud"
  role      = "roles/secretmanager.secretAccessor"
  secret_id = google_secret_manager_secret.pg_site_builder_dsn.id
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
