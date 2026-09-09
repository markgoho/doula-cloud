# Imported from live state (docs/infrastructure.md, #1045). Both Scheduler
# jobs and the Cloud Tasks queue they feed are #481's case: a missing or
# misconfigured job was invisible until an outbox's own close-out went and
# looked, and this is what turns that into a red `terraform plan` instead.
#
# The `X-Internal-Secret` header both jobs send is read from Secret Manager
# through this data source, never written as a literal here. It still lands
# in Terraform state as a field on the job — docs/infrastructure.md accepts
# that explicitly as the one secret state carries.
data "google_secret_manager_secret_version" "notification_worker_secret" {
  secret = "doula-cloud-notification-worker-secret"
}

# `prevent_destroy` is deliberately not set on any of the four resources in
# this file. None holds data or identity: a Scheduler job is a schedule and a
# target URL, re-created from this configuration in one apply, and the queue
# and its IAM binding are the same. A plan that proposes to recreate one of
# them is the correct answer, not a danger to block — unlike `doula-api` in
# cloud_run.tf, or the Cloud SQL instance and buckets #1048 will add, none of
# which can be recreated without losing something.
resource "google_cloud_scheduler_job" "process_outbox_drain" {
  name             = "process-outbox-drain"
  project          = "doula-cloud"
  region           = "us-central1"
  description      = "ADR-0010 as amended by #481: runs every outbox in the registry. The durability backstop under ADR-0013's nudge, replacing one Cloud Scheduler job per outbox."
  schedule         = "*/5 * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "180s"
  paused           = false
  deletion_policy  = "DELETE"

  http_target {
    http_method = "POST"
    uri         = "https://doula-api-850855848778.us-central1.run.app/api/internal/outboxes/drain"
    headers = {
      X-Internal-Secret = nonsensitive(data.google_secret_manager_secret_version.notification_worker_secret.secret_data)
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
  name             = "verify-practice-pages"
  project          = "doula-cloud"
  region           = "us-central1"
  description      = "#443: probes every published Practice Page and records whether it resolved; catches a build that failed and never reported"
  schedule         = "*/15 * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "180s"
  paused           = false
  deletion_policy  = "DELETE"

  http_target {
    http_method = "POST"
    uri         = "https://doula-api-850855848778.us-central1.run.app/api/internal/site/verify-pages"
    headers = {
      X-Internal-Secret = nonsensitive(data.google_secret_manager_secret_version.notification_worker_secret.secret_data)
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
  name            = "doula-cloud-notification-nudge"
  project         = "doula-cloud"
  location        = "us-central1"
  deletion_policy = "DELETE"

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

resource "google_cloud_tasks_queue_iam_member" "notification_nudge_enqueuer" {
  project  = "doula-cloud"
  location = "us-central1"
  name     = google_cloud_tasks_queue.notification_nudge.id
  role     = "roles/cloudtasks.enqueuer"
  member   = "serviceAccount:850855848778-compute@developer.gserviceaccount.com"
}
