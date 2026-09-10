# Imported from live state (docs/infrastructure.md, #1048). #797's boundary
# owns `doula-cloud-attachments`, the one project-owned bucket: uniform
# bucket-level access, enforced public access prevention, no lifecycle rule
# and no versioning.
#
# No `versioning` block is declared. `lifecycle_rule` is an ordinary Optional
# set, so its absence here already makes a hand-added rule show up as a plan
# diff. `versioning` is different: on a bucket that has never had versioning
# turned on, the live API returns no versioning object at all rather than
# `{enabled: false}`, so declaring `versioning { enabled = false }` here was
# tried and produced `1 to change` on an otherwise clean plan — a false
# drift, not a real one. Turning versioning on by hand would still be
# invisible to `plan` as a result; that gap is accepted rather than forced,
# since forcing it means carrying a permanent phantom diff instead.
#
# No other bucket is here, and none should be. `doula-cloud.firebasestorage.app`
# and `run-sources-doula-cloud-us-central1` are both created by something
# other than this project for its own use (Firebase enabling itself, and
# `gcloud run deploy --source` build scratch), and are in docs/infrastructure.md's
# by-hand table for that reason — a second owner on either would read as
# drift, not detect it. `gs://doula-cloud-tfstate` is also not here: it holds
# this configuration's own state, and a configuration that owns its state
# bucket can propose to delete it (docs/infrastructure.md, "State").
resource "google_storage_bucket" "attachments" {
  default_event_based_hold    = false
  deletion_policy             = "DELETE"
  enable_object_retention     = false
  force_destroy               = false
  labels                      = {}
  location                    = "US-CENTRAL1"
  name                        = "doula-cloud-attachments"
  project                     = "doula-cloud"
  public_access_prevention    = "enforced"
  requester_pays              = false
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true

  hierarchical_namespace {
    enabled = false
  }

  soft_delete_policy {
    retention_duration_seconds = 604800
  }

  lifecycle {
    prevent_destroy = true
  }
}

# #1051: the attachments half of `doula-api-runtime@`'s grants, and the one
# place the runtime identity is granted anything on this bucket. The binding
# is on the bucket, never on an object: uniform bucket-level access is on, so
# an object ACL would be ignored even if something set one.
#
# `roles/storage.objectUser`, on this bucket alone, rather than the narrower
# `objectCreator` + `objectViewer` pair. `objectstore.GCSStore` exposes two
# operations, `Put` and `Get`, and the narrow pair covers them right up until
# `Put` writes a key that already exists — which it is documented to do, and
# which `contracts.SignedPDFObjectPath` makes reachable, because that key is
# derived from the contract rather than made unique per write. Overwriting an
# object in a bucket with no versioning needs `storage.objects.delete` as
# well as `.create`, so the narrow pair turns a re-signed contract into a
# `403` at the moment it is written. `objectUser` is the narrowest predefined
# role that covers create, read and the delete that an overwrite performs.
#
# It is still an object-level role on one bucket: it cannot read the bucket's
# IAM policy, change its configuration, or reach any other bucket in the
# project. Before this, the container reached the bucket through project
# `roles/editor` on the default compute account, which could do all three.
resource "google_storage_bucket_iam_member" "attachments_runtime_object_user" {
  bucket = google_storage_bucket.attachments.name
  member = google_service_account.doula_api_runtime.member
  role   = "roles/storage.objectUser"
}
