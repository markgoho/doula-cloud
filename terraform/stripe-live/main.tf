# Validated in CI, never applied by #2390. Applying it is #2463, the day
# before the switch (#2384), and the first thing that touches the live
# account. #2463 also adds this root's plan to terraform-plan.yml.

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
    prefix = "stripe/live"
  }
}

variable "stripe_api_key" {
  type        = string
  sensitive   = true
  description = "A live restricted key, from TF_VAR_stripe_api_key: the read-only one for plan, the write one for apply."

  validation {
    condition     = can(regex("^rk_live_", var.stripe_api_key))
    error_message = "The live root takes a live restricted key (rk_live_…), never a secret key."
  }
}

provider "stripe" {
  api_key = var.stripe_api_key
}

module "stripe" {
  source = "../stripe"

  livemode      = true
  base_url      = "https://doula-api-850855848778.us-central1.run.app"
  credit_prices = { "2000" = true }
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
