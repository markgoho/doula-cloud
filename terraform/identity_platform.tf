# The operator tenant (ADR-0062, #2610). doula-ops makes any login in this
# tenant an operator who is on its `operators` list, and the list is the
# second gate; this is the first: nobody can sign up to the tenant, so a
# stranger cannot even become a login. Created by hand on 2026-11-01
# (docs/infrastructure.md, "Bringing up `doula-ops`", step 2) and adopted by
# the import block below, so a later change by hand shows as drift on `plan`.
#
# `allow_password_signup` is not "may people sign up". On a tenant it is the
# whole email/password provider: false turns password sign-in off
# (`PASSWORD_LOGIN_DISABLED`). Closing signup is
# `client.permissions.disabled_user_signup`.
#
# Not here, because `google_identity_platform_tenant` has no field for it:
# the tenant's `mfaConfig` (state MANDATORY, TOTP ENABLED, adjacentIntervals
# 5), which stays by hand per step 2. `google_identity_platform_config` owns
# multi-factor settings for the default tenant only.
import {
  to = google_identity_platform_tenant.operators
  id = "projects/doula-cloud/tenants/operators-fj5ur"
}

resource "google_identity_platform_tenant" "operators" {
  display_name             = "operators"
  allow_password_signup    = true
  enable_email_link_signin = false
  disable_auth             = false

  client {
    permissions {
      disabled_user_signup   = true
      disabled_user_deletion = false
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}
