# #1403: an uptime check on the app and one on the API, each alerting the
# founder's phone and nobody else, and only from 07:00 to 22:00
# America/New_York. The window is the Scheduler pair in scheduler.tf.
#
# The phone channel is made by hand in the console, where its SMS
# verification happens, so the number is never in this repository, in state
# written from CI, or in a plan log. Terraform finds it by display name. The
# data source fails the plan when no channel has that name, so it is read
# only once `founder_phone_channel_exists` is true, flipped in a commit after
# the channel exists.
#
# A later policy (#2417) joins by using `local.founder_phone_channels` and
# adding itself to `local.waking_hours_alert_policies`.
locals {
  founder_phone_channel_exists = false

  founder_phone_channels = local.founder_phone_channel_exists ? [data.google_monitoring_notification_channel.founder_phone[0].name] : []

  waking_hours_alert_policies = {
    app-uptime = google_monitoring_alert_policy.app_uptime.name
    api-uptime = google_monitoring_alert_policy.api_uptime.name
  }

  uptime_check_regions = ["USA_IOWA", "USA_OREGON", "USA_VIRGINIA"]
}

data "google_monitoring_notification_channel" "founder_phone" {
  count = local.founder_phone_channel_exists ? 1 : 0

  display_name = "Founder phone"
  project      = "doula-cloud"
  type         = "sms"
}

# The app is the Hosting `app` target. `doula.cloud` is the marketing site
# (`site` target), and a check there would stay green with the app down.
resource "google_monitoring_uptime_check_config" "app" {
  checker_type     = "STATIC_IP_CHECKERS"
  display_name     = "App"
  period           = "60s"
  project          = "doula-cloud"
  selected_regions = local.uptime_check_regions
  timeout          = "10s"

  http_check {
    path           = "/"
    port           = 443
    request_method = "GET"
    use_ssl        = true
    validate_ssl   = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      host       = "doula-cloud-app.web.app"
      project_id = "doula-cloud"
    }
  }
}

resource "google_monitoring_uptime_check_config" "api" {
  checker_type     = "STATIC_IP_CHECKERS"
  display_name     = "API"
  period           = "60s"
  project          = "doula-cloud"
  selected_regions = local.uptime_check_regions
  timeout          = "10s"

  content_matchers {
    content = "hello world"
    matcher = "CONTAINS_STRING"
  }

  http_check {
    path           = "/api/hello"
    port           = 443
    request_method = "GET"
    use_ssl        = true
    validate_ssl   = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      host       = trimprefix(local.doula_api_base_url, "https://")
      project_id = "doula-cloud"
    }
  }
}

# Fires when two of the three regions have failed for two minutes running,
# and resolves on its own when the check passes again. `enabled` is the
# Scheduler pair's to move, and the daily plan runs inside the night window.
resource "google_monitoring_alert_policy" "app_uptime" {
  combiner              = "OR"
  display_name          = "App is down"
  notification_channels = local.founder_phone_channels
  project               = "doula-cloud"

  alert_strategy {
    auto_close = "1800s"
  }

  conditions {
    display_name = "App uptime check failing"

    condition_threshold {
      comparison      = "COMPARISON_GT"
      duration        = "120s"
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.app.uptime_check_id}\""
      threshold_value = 1

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.*"]
        per_series_aligner   = "ALIGN_NEXT_OLDER"
      }

      trigger {
        count = 1
      }
    }
  }

  lifecycle {
    ignore_changes = [enabled]
  }
}

resource "google_monitoring_alert_policy" "api_uptime" {
  combiner              = "OR"
  display_name          = "API is down"
  notification_channels = local.founder_phone_channels
  project               = "doula-cloud"

  alert_strategy {
    auto_close = "1800s"
  }

  conditions {
    display_name = "API uptime check failing"

    condition_threshold {
      comparison      = "COMPARISON_GT"
      duration        = "120s"
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.api.uptime_check_id}\""
      threshold_value = 1

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.*"]
        per_series_aligner   = "ALIGN_NEXT_OLDER"
      }

      trigger {
        count = 1
      }
    }
  }

  lifecycle {
    ignore_changes = [enabled]
  }
}
