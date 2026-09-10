# Imported from live state (docs/infrastructure.md, #1045). Both Scheduler
# jobs and the Cloud Tasks queue they feed are #481's case: a missing or
# misconfigured job was invisible until an outbox's own close-out went and
# looked, and this is what turns that into a red `terraform plan` instead.
#
# Both jobs authenticate with a Google-signed OIDC token minted for
# `internal-caller@` (ADR-0037, #1183), replacing the `X-Internal-Secret`
# header each used to carry. The header's value was read from Secret Manager
# through a `nonsensitive()` data source and still landed in Terraform state
# as a field on the job — a cost docs/infrastructure.md accepted while a
# shared string was the boundary. A service account address is not a secret,
# so state now carries none at all.

# `prevent_destroy` is deliberately not set on any of the four resources in
# this file. None holds data or identity: a Scheduler job is a schedule and a
# target URL, re-created from this configuration in one apply, and the queue
# and its IAM binding are the same. A plan that proposes to recreate one of
# them is the correct answer, not a danger to block — unlike `doula-api` in
# cloud_run.tf, or the Cloud SQL instance and buckets #1048 will add, none of
# which can be recreated without losing something.
resource "google_cloud_scheduler_job" "process_outbox_drain" {
  attempt_deadline = "180s"
  deletion_policy  = "DELETE"
  description      = "ADR-0010 as amended by #481: runs every outbox in the registry. The durability backstop under ADR-0013's nudge, replacing one Cloud Scheduler job per outbox."
  name             = "process-outbox-drain"
  paused           = false
  project          = "doula-cloud"
  region           = "us-central1"
  schedule         = "*/5 * * * *"
  time_zone        = "Etc/UTC"

  http_target {
    http_method = "POST"
    uri         = "${local.doula_api_base_url}/api/internal/outboxes/drain"

    # `audience` is set explicitly, and must be. Cloud Scheduler defaults an
    # unset audience to the full target URI — the `/api/internal/outboxes/drain`
    # path included — while `internalauth.Guard` validates against
    # `INTERNAL_OIDC_AUDIENCE`, the service's base URL, because one value has
    # to serve all twenty-three internal addresses. A job left to the default
    # therefore 401s on its first tick with a token that is otherwise
    # perfectly valid and signed by the right account.
    oidc_token {
      audience              = local.doula_api_base_url
      service_account_email = google_service_account.internal_caller.email
    }
  }

  retry_config {
    retry_count          = 0
    max_retry_duration   = "0s"
    min_backoff_duration = "5s"
    max_backoff_duration = "3600s"
    max_doublings        = 5
  }
}

resource "google_cloud_scheduler_job" "verify_practice_pages" {
  attempt_deadline = "180s"
  deletion_policy  = "DELETE"
  description      = "#443: probes every published Practice Page and records whether it resolved; catches a build that failed and never reported"
  name             = "verify-practice-pages"
  paused           = false
  project          = "doula-cloud"
  region           = "us-central1"
  schedule         = "*/15 * * * *"
  time_zone        = "Etc/UTC"

  http_target {
    http_method = "POST"
    uri         = "${local.doula_api_base_url}/api/internal/site/verify-pages"

    # Explicit for the same reason as the drain job above: the default
    # audience is this full URI, and the guard checks the base URL.
    oidc_token {
      audience              = local.doula_api_base_url
      service_account_email = google_service_account.internal_caller.email
    }
  }

  retry_config {
    retry_count          = 0
    max_retry_duration   = "0s"
    min_backoff_duration = "5s"
    max_backoff_duration = "3600s"
    max_doublings        = 5
  }
}

# ADR-0013's nudge path: the queue existing without the enqueuer binding below
# is exactly the silent half-configuration this import pass exists to catch.
resource "google_cloud_tasks_queue" "notification_nudge" {
  deletion_policy = "DELETE"
  desired_state   = "RUNNING"
  location        = "us-central1"
  name            = "doula-cloud-notification-nudge"
  project         = "doula-cloud"

  rate_limits {
    max_concurrent_dispatches = 1000
    max_dispatches_per_second = 500
  }

  retry_config {
    max_attempts  = 100
    min_backoff   = "0.100s"
    max_backoff   = "3600s"
    max_doublings = 16
  }
}

# The enqueuer is `doula-api-runtime@` since #1051. It was the default
# compute account until then, which held the role through project
# `roles/editor` as well as through this binding.
resource "google_cloud_tasks_queue_iam_member" "notification_nudge_runtime_enqueuer" {
  location = "us-central1"
  member   = google_service_account.doula_api_runtime.member
  name     = google_cloud_tasks_queue.notification_nudge.id
  project  = "doula-cloud"
  role     = "roles/cloudtasks.enqueuer"
}
