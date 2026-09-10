# Imported from "projects/doula-cloud/locations/us-central1/services/doula-api"
# (docs/infrastructure.md). Terraform owns the shape of this service, never
# its image: the `lifecycle` block below is the whole of the split between
# this configuration and `deploy-api` in `.github/workflows/ci.yml`.
resource "google_cloud_run_v2_service" "doula_api" {
  annotations          = {}
  client               = "gcloud"
  client_version       = "568.0.0"
  custom_audiences     = []
  default_uri_disabled = false
  deletion_policy      = "DELETE"
  # Set deliberately, distinct from `lifecycle.prevent_destroy` below: this is
  # a provider-level argument Terraform can also flip to false in the same
  # apply that then destroys the service, where `prevent_destroy` is Terraform
  # refusing to run the plan at all (docs/infrastructure.md, "Making the
  # first apply safe").
  deletion_protection  = true
  description          = null
  iap_enabled          = false
  ingress              = "INGRESS_TRAFFIC_ALL"
  invoker_iam_disabled = false
  labels               = {}
  launch_stage         = "GA"
  location             = "us-central1"
  name                 = "doula-api"
  project              = "doula-cloud"
  tags                 = null
  # Stale residue from a long-past `gcloud run deploy --source`; inert today
  # since every deploy since has been by image reference (docs/infrastructure.md,
  # "The Cloud Run image conflict"). Declared as-is and added to `ignore_changes`
  # below rather than cleared: the live service has no direct API surface to
  # clear it (no `gcloud` flag, only a raw REST PATCH with an update mask), and
  # making that call here would be an out-of-band change to the running
  # service outside this ticket's import-only-until-empty-plan mechanism.
  build_config {
    base_image               = null
    enable_automatic_updates = false
    environment_variables    = {}
    function_target          = null
    image_uri                = "us-central1-docker.pkg.dev/doula-cloud/cloud-run-source-deploy/doula-api"
    service_account          = null
    source_location          = "gs://run-sources-doula-cloud-us-central1/services/doula-api/1786507208.676945-cf2c994d25094f0eb9b4039d4459085a.zip#1786507208905336"
    worker_pool              = null
  }
  scaling {
    manual_instance_count = 0
    max_instance_count    = 20
    min_instance_count    = 0
    scaling_mode          = null
  }
  template {
    annotations                   = {}
    encryption_key                = null
    execution_environment         = null
    gpu_zonal_redundancy_disabled = false
    health_check_disabled         = false
    labels = {
      commit-sha            = "27c9af6456831ba9fea92a0afd3b9e2bbe5326a4"
      managed-by            = "github-actions"
      stripe-account-secret = "v2"
    }
    max_instance_request_concurrency = 80
    revision                         = null
    # #1051. This was `850855848778-compute@developer.gserviceaccount.com`,
    # the Google-created default compute account, which held project
    # `roles/editor` — the container could change or delete any Cloud Run
    # service, alter the Cloud SQL instance, and write to any bucket in the
    # project. Not read every secret: `roles/editor` excludes
    # `secretmanager.versions.access`, which is why the per-secret grants in
    # secrets.tf were load-bearing even then. `deploy-api` in
    # ci.yml passes the same address as `--service-account` on every deploy,
    # so a deploy cannot quietly put the default account back; see iam.tf for
    # where each of this identity's grants comes from.
    service_account  = google_service_account.doula_api_runtime.email
    session_affinity = false
    timeout          = "300s"
    containers {
      args             = []
      base_image_uri   = null
      command          = []
      depends_on       = []
      image            = "us-central1-docker.pkg.dev/doula-cloud/api/doula-api:27c9af6456831ba9fea92a0afd3b9e2bbe5326a4"
      name             = null
      sandbox_launcher = false
      working_dir      = null
      env {
        name  = "APP_BASE_URL"
        value = "https://doula-cloud-app.web.app"
      }
      env {
        name  = "DATABASE_URL"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-pg-app-runtime-dsn"
            version = "latest"
          }
        }
      }
      env {
        name  = "EXPECTED_ORIGINS"
        value = "https://doula-cloud-app.web.app"
      }
      env {
        name  = "GCS_ATTACHMENTS_BUCKET"
        value = "doula-cloud-attachments"
      }
      env {
        name  = "GITHUB_DISPATCH_TOKEN"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-github-dispatch-token"
            version = "latest"
          }
        }
      }
      env {
        name  = "MAILGUN_API_KEY"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-mailgun-api-key"
            version = "latest"
          }
        }
      }
      env {
        name  = "MAILGUN_DOMAIN"
        value = "mg.doula.cloud"
      }
      env {
        name  = "MAILGUN_WEBHOOK_SIGNING_KEY"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-mailgun-webhook-signing-key"
            version = "latest"
          }
        }
      }
      env {
        name  = "NOTIFICATION_TASKS_QUEUE"
        value = "projects/doula-cloud/locations/us-central1/queues/doula-cloud-notification-nudge"
      }
      env {
        name  = "NOTIFICATION_TASKS_TARGET_BASE_URL"
        value = "https://doula-api-850855848778.us-central1.run.app"
      }
      env {
        name  = "NOTIFICATION_WORKER_SECRET"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-notification-worker-secret"
            version = "latest"
          }
        }
      }
      env {
        name  = "STRIPE_ACCOUNT_WEBHOOK_SECRET"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-stripe-account-webhook-secret"
            version = "latest"
          }
        }
      }
      env {
        name  = "STRIPE_API_KEY"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-stripe-api-key"
            version = "latest"
          }
        }
      }
      env {
        name  = "STRIPE_CONNECT_WEBHOOK_SECRET"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-stripe-connect-webhook-secret"
            version = "latest"
          }
        }
      }
      env {
        name  = "STRIPE_CREDIT_PRICE_ID"
        value = "price_1U9yTw1rKoVEA79v6CcEaxZg"
      }
      env {
        name  = "STRIPE_WEBHOOK_SECRET"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-stripe-webhook-secret"
            version = "latest"
          }
        }
      }
      env {
        name  = "VAPID_PRIVATE_KEY"
        value = null
        value_source {
          secret_key_ref {
            secret  = "doula-cloud-vapid-private-key"
            version = "latest"
          }
        }
      }
      env {
        name  = "VAPID_PUBLIC_KEY"
        value = "BCy44BWTeiwIi5f7-7O5PC7W8NwCssG6pl0_MlLGZYOxcP88qN5qcl4H1bMhRgsOZdssZ8PXc5R1wGhj8sPbaZs"
      }
      env {
        name  = "VAPID_SUBSCRIBER"
        value = "mailto:admin@doula.cloud"
      }
      ports {
        container_port = 8080
        name           = "http1"
      }
      resources {
        cpu_idle = true
        limits = {
          cpu    = "1000m"
          memory = "512Mi"
        }
        startup_cpu_boost = true
      }
      startup_probe {
        failure_threshold     = 1
        initial_delay_seconds = 0
        period_seconds        = 240
        timeout_seconds       = 240
        tcp_socket {
          port = 8080
        }
      }
      volume_mounts {
        mount_path = "/cloudsql"
        name       = "cloudsql"
        sub_path   = null
      }
    }
    scaling {
      max_instance_count = 20
      min_instance_count = 1
    }
    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = ["doula-cloud:us-central1:doula-cloud-pg"]
      }
    }
  }
  traffic {
    percent  = 100
    revision = null
    tag      = null
    type     = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
  }

  # `prevent_destroy` here is Terraform-side: it makes `terraform` refuse to
  # run any plan that would destroy this resource, before an apply is ever
  # attempted. See the comment on `deletion_protection` above for how this
  # differs from the provider-level argument. Never removed; the service holds
  # identity (its URL, its Cloud SQL attachment, its 19 environment variables
  # and secret references) that nothing here should ever destroy.
  #
  # `ignore_changes` is the split between what Terraform owns (the shape of
  # the service) and what `deploy-api` in `.github/workflows/ci.yml` owns (the
  # image), from docs/infrastructure.md's "The Cloud Run image conflict":
  # `client`/`client_version` record which tool last wrote the service and
  # move only when `setup-gcloud` changes gcloud's version; the image and the
  # revision template's `commit-sha` label are what every real deploy writes;
  # `build_config` is the stale residue documented above.
  lifecycle {
    prevent_destroy = true

    ignore_changes = [
      client,
      client_version,
      template[0].containers[0].image,
      template[0].labels["commit-sha"],
      build_config,
    ]
  }
}
