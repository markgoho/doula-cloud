# The Stripe configuration DoulaCloud depends on, declared once and applied
# to both accounts (#2390): the Sandbox from terraform/stripe-sandbox, the
# live account from terraform/stripe-live (#2463). See docs/environment.md,
# "Deployed webhook endpoints", and ADR-0034's 2026-10-31 amendment.
#
# Each route's events live in destinations.json so that
# api/stripe_destinations_test.go can compare them with the handlers
# without parsing HCL.
#
# Provider 0.3.0 always asks for the signing secret on create, so a created
# destination's first secret lands in state. The founder rolls it in the
# Dashboard straight after the apply, and only the rolled secret goes to
# Secret Manager (docs/infrastructure.md, "Stripe state").

terraform {
  required_providers {
    stripe = {
      source  = "stripe/stripe"
      version = "0.3.0"
    }
  }
}

variable "livemode" {
  type        = bool
  description = "Whether the account this applies to is the live account. Each resource refuses to plan against an account in the other mode."
}

variable "base_url" {
  type        = string
  description = "The origin each event destination's route is appended to."

  validation {
    condition     = can(regex("^https://[^/]+$", var.base_url))
    error_message = "base_url is an https origin with no path and no trailing slash."
  }
}

variable "credit_prices" {
  type        = map(bool)
  description = "Each credit Price, keyed by its unit_amount in cents, with whether it is active. A rise in the price of a Credit adds one before its date and archives the old one after it (docs/environment.md)."
}

locals {
  destinations = jsondecode(file("${path.module}/destinations.json"))
}

resource "stripe_product" "credit" {
  name        = "Engagement credit"
  description = "One DoulaCloud Engagement. Consumed when a Practice creates an Engagement."
  type        = "service"
  tax_code    = "txcd_10103001"

  lifecycle {
    prevent_destroy = true
    postcondition {
      condition     = self.livemode == var.livemode
      error_message = "This Product is in the other Stripe mode from this root."
    }
  }
}

resource "stripe_price" "credit" {
  for_each = var.credit_prices

  product      = stripe_product.credit.id
  currency     = "usd"
  unit_amount  = tonumber(each.key)
  tax_behavior = "exclusive"
  active       = each.value

  lifecycle {
    prevent_destroy = true
    postcondition {
      condition     = self.livemode == var.livemode
      error_message = "This Price is in the other Stripe mode from this root."
    }
  }
}

resource "stripe_v2_core_event_destination" "route" {
  for_each = local.destinations

  type                 = "webhook_endpoint"
  name                 = each.value.name
  description          = each.value.description
  event_payload        = each.value.event_payload
  events_from          = each.value.events_from
  snapshot_api_version = each.value.snapshot_api_version
  enabled_events       = each.value.enabled_events
  include              = ["webhook_endpoint.url"]

  webhook_endpoint {
    url = "${var.base_url}${each.key}"
  }

  lifecycle {
    postcondition {
      condition     = self.livemode == var.livemode
      error_message = "This event destination is in the other Stripe mode from this root."
    }
  }
}

output "credit_product_id" {
  value = stripe_product.credit.id
}

output "credit_price_ids" {
  value = { for amount, price in stripe_price.credit : amount => price.id }
}

output "destination_ids" {
  value = { for route, destination in stripe_v2_core_event_destination.route : route => destination.id }
}
