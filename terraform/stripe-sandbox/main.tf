terraform {
  required_version = "~> 1.16"

  required_providers {
    stripe = {
      source  = "stripe/stripe"
      version = "0.3.0"
    }
  }

  backend "gcs" {
    bucket = "doula-cloud-tfstate"
    prefix = "stripe/sandbox"
  }
}

variable "stripe_api_key" {
  type        = string
  sensitive   = true
  description = "A Sandbox key, from TF_VAR_stripe_api_key: the read-only restricted key for plan, the write restricted key for apply."

  validation {
    condition     = can(regex("^(rk|sk)_test_", var.stripe_api_key))
    error_message = "The Sandbox root takes a Sandbox key (rk_test_… or sk_test_…)."
  }
}

provider "stripe" {
  api_key = var.stripe_api_key
}

module "stripe" {
  source = "../stripe"

  livemode      = false
  base_url      = "https://doula-api-850855848778.us-central1.run.app"
  credit_prices = { "2000" = true }
}

import {
  to = module.stripe.stripe_product.credit
  id = "prod_V7cZNA52RwbgZC"
}

import {
  to = module.stripe.stripe_price.credit["2000"]
  id = "price_1U9yTw1rKoVEA79v6CcEaxZg"
}

import {
  to = module.stripe.stripe_v2_core_event_destination.route["/api/stripe/account-webhook"]
  id = "ed_test_61VGn5QffuUmiONhX16VGl100QSQI7KSJpC2tKXrs4xU"
}

import {
  to = module.stripe.stripe_v2_core_event_destination.route["/api/stripe/connect-webhook"]
  id = "we_1U7Ocp1rKoVEA79vT9AXETKU"
}

import {
  to = module.stripe.stripe_v2_core_event_destination.route["/api/stripe/webhook"]
  id = "we_1U7NT01rKoVEA79vnOcBFqtV"
}

output "credit_product_id" {
  value = module.stripe.credit_product_id
}

output "credit_price_ids" {
  value = module.stripe.credit_price_ids
}

output "destination_ids" {
  value = module.stripe.destination_ids
}
