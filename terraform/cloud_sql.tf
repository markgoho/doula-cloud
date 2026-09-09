# Imported from live state (docs/infrastructure.md, #1048). #797's boundary
# owns the instance's shape and its two databases, never the logins.
#
# `deletion_protection` (top level) and `settings.deletion_protection_enabled`
# are two different mechanisms, both set deliberately, and neither is a
# substitute for `lifecycle.prevent_destroy` below:
#   - `deletion_protection` is this resource's own Terraform-side argument
#     (default `true` in the provider). It blocks `terraform destroy`/a
#     destructive `terraform apply` from deleting this resource at all.
#   - `settings.deletion_protection_enabled` mirrors the GCP API field
#     (`settings.deletionProtectionEnabled`) that `gcloud sql instances patch
#     --deletion-protection` sets — the live, mutable, GCP-level protection
#     that #1044 turned on before any Terraform ran. It is not the same field
#     as the one above: this one can be flipped to `false` and the instance
#     then deleted, through the API or the console, entirely outside
#     Terraform.
#   - `lifecycle.prevent_destroy` is Terraform refusing to run a plan that
#     would destroy this resource at all, before either of the two API-level
#     flags above is ever consulted.
# All three are set, the same three-mechanism reasoning `cloud_run.tf` and
# `secrets.tf` already use for `doula-api` and the secret shells.
#
# `google_sql_user` is never a resource in this configuration. It writes the
# generated or supplied password to Terraform state in plaintext, and
# `app_runtime_login` and `site_builder_login` already have their DSNs in
# Secret Manager (`secrets.tf`'s `pg_app_runtime_dsn` and
# `pg_site_builder_dsn`). Creating and rotating the logins by hand, per
# docs/infrastructure.md's by-hand table, is the only way this configuration
# never holds a database password.
resource "google_sql_database_instance" "doula_cloud_pg" {
  database_version                           = "POSTGRES_16"
  deletion_policy                            = "DELETE"
  deletion_protection                        = true
  enforce_new_sql_network_architecture       = true
  include_replicas_for_major_version_upgrade = null
  instance_type                              = "CLOUD_SQL_INSTANCE"
  maintenance_version                        = "POSTGRES_16_14.R20260712.01_06"
  name                                       = "doula-cloud-pg"
  project                                    = "doula-cloud"
  region                                     = "us-central1"

  settings {
    activation_policy            = "ALWAYS"
    auto_upgrade_enabled         = false
    availability_type            = "ZONAL"
    connector_enforcement        = "NOT_REQUIRED"
    deletion_protection_enabled  = true
    disk_autoresize              = false
    disk_size                    = 10
    disk_type                    = "PD_HDD"
    edition                      = "ENTERPRISE"
    enable_dataplex_integration  = true
    enable_google_ml_integration = false
    pricing_plan                 = "PER_USE"
    replication_lag_max_seconds  = 31536000
    retain_backups_on_delete     = false
    tier                         = "db-f1-micro"
    user_labels                  = {}

    backup_configuration {
      binary_log_enabled             = false
      enabled                        = true
      point_in_time_recovery_enabled = false
      start_time                     = "08:00"
      transaction_log_retention_days = 7

      backup_retention_settings {
        retained_backups = 7
        retention_unit   = "COUNT"
      }
    }

    data_cache_config {
      data_cache_enabled = false
    }

    ip_configuration {
      custom_subject_alternative_names              = []
      enable_private_path_for_google_cloud_services = false
      ipv4_enabled                                  = true
      server_ca_mode                                = "GOOGLE_MANAGED_INTERNAL_CA"
      ssl_mode                                      = "TRUSTED_CLIENT_CERTIFICATE_REQUIRED"
    }

    location_preference {
      zone = "us-central1-c"
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_sql_database" "doula_cloud" {
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = "DELETE"
  instance        = google_sql_database_instance.doula_cloud_pg.name
  name            = "doula_cloud"
  project         = "doula-cloud"

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_sql_database" "postgres" {
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = "DELETE"
  instance        = google_sql_database_instance.doula_cloud_pg.name
  name            = "postgres"
  project         = "doula-cloud"

  lifecycle {
    prevent_destroy = true
  }
}
