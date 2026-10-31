# How a Platform operator can sign in to ops.doula.cloud

Research for [Research: how a Platform operator can sign in on Identity Platform (#2410)](https://github.com/markgoho/doula-cloud/issues/2410), part of [Wayfinder: the Platform operator app at ops.doula.cloud (#2409)](https://github.com/markgoho/doula-cloud/issues/2409). It feeds the sign-in grilling, [#2414](https://github.com/markgoho/doula-cloud/issues/2414). Facts only. Anything marked **Recommendation** is a suggestion, not a decision.

Researched 2026-10-31. Repo facts are cited by file path at commit `cf27d249`. Outside facts are cited to Google's own pages, the Terraform provider's source docs, and the Firebase Admin Go SDK's source.

## Summary

| Question | Answer |
| --- | --- |
| Can a tenant hold operator logins apart from Staff? | Yes. A tenant has its own users, providers, MFA config and password policy. Tenants cost nothing extra. Two catches: Terraform's `google_identity_platform_tenant` has no MFA or password-policy argument, and `doula-api`'s token check would accept a tenant's ID token today. |
| A separate GCP project? | Possible. Costs a second Identity Platform config (cannot be deleted, needs billing), a second HIPAA BAA acceptance, and a second set of everything around it. |
| Second factors Identity Platform offers | SMS and TOTP. No WebAuthn, passkeys or security keys. |
| Can MFA be required? | The `MultiFactorAuthConfig.state` enum has `MANDATORY`, at project and at tenant level. At project level it would bind every Staff member, which conflicts with ADR-0026's per-Practice switch. At tenant level it binds only that tenant's users. The repo enforces MFA in the BFF anyway. |
| Passkeys or hardware keys for the operator | Only two routes: a BFF-owned WebAuthn build (as researched for Staff in `docs/research/passkeys-for-staff.md`), or a Google account behind IAP, where Google's Advanced Protection Program requires a passkey or security key. |
| Authorized domains | Project-wide; a tenant has no such field. Documented as the list "authorized for OAuth redirects". The Staff flow uses no redirect. |
| `__session` on a second Hosting origin | Works: the repo's cookie is host-only, so `ops.doula.cloud` and the app's host each hold their own `__session`. The one-cookie limit applies only behind a Firebase Hosting rewrite. |

## 1. How the repo signs in today

- **Two populations, one session table.** Staff sign in with email and password in Identity Platform; Clients have no Identity Platform account and use a BFF-minted magic link. TOTP, never SMS, is the Staff second factor, enforced in the BFF from the `firebase.sign_in_second_factor` claim. A Staff session lasts 12 hours. (`docs/adr/0026-two-populations-two-sign-in-methods-and-one-token-table.md`, "Current rule".)
- **The BFF owns the session.** Identity Platform mints nothing the browser keeps; after `VerifyIDToken`, `authn.MintSession` writes a Postgres row and sets `__session`. (ADR-0026, "What this amends in ADR-0004"; `api/internal/session/session.go:67`.)
- **The cookie.** `NewSessionCookie` sets `Name: "__session"`, `Path: "/"`, `HttpOnly`, `Secure`, `SameSite=Lax`, and **no `Domain`**. (`api/internal/authn/authn.go:72-81`.) The name is fixed because Hosting strips every other cookie on the `/api/**` rewrite. (`api/internal/authn/authn.go:27-37`.)
- **One browser holds one tier at a time.** Signing into the other population ends the live session, after a warning. (ADR-0026, the eviction section near line 261.)
- **The founder gate.** `staffauth.FounderOnly` admits one person: the Staff member whose `staff.id` equals `FOUNDER_STAFF_ID`, on a session with a second factor. Anybody else, or no founder configured, gets a `404`. It serves two routes, `GET /api/staff/feedback` and `GET /api/staff/feedback/{feedbackId}`. (`api/internal/staffauth/founder.go`; built on [#1526](https://github.com/markgoho/doula-cloud/issues/1526), closed.)
- **`FOUNDER_STAFF_ID` is not set on the deployed service.** Locally it is the fixed id `0f0a1d5e-5a17-4c0b-9d0f-f0de15260001`, seeded by `seed:founder-session`. Deployed it is "Not set yet", so the founder routes answer `404` to everybody. (`docs/environment.md`, `FOUNDER_STAFF_ID` row.) Setting it is [#1607](https://github.com/markgoho/doula-cloud/issues/1607), open. Its step 1 has the founder sign up for a Staff login, "the Practice it creates can be the founder's own test Practice".
- **A Staff signup creates a Practice.** `api/internal/staffauth/signup.go:248` runs `INSERT INTO practices (name, timezone)`. The map notes this is why reusing a Staff login puts the operator inside a Practice, which `GLOSSARY.md`'s Platform operator ("Staff, Owner, Admin or Doula at no Practice") rules out.
- **Identity Platform's configuration is not in Terraform.** `signIn.email`, MFA settings and authorized domains are "walked by hand once". (`docs/infrastructure.md:70`.) The provider pinned is `hashicorp/google ~> 7.0`. (`terraform/versions.tf`.)
- **`doula-api`'s runtime role cannot change Identity Platform's configuration.** Its custom role reads, updates and deletes accounts only; `firebaseauth.configs.*` is withheld because it is "the power to change sign-in providers, MFA enforcement and authorized domains". (`terraform/iam.tf:205-227`.)
- **The web client** initializes with `authDomain: 'doula-cloud.firebaseapp.com'`. (`app/src/lib/firebase.ts:19`.)
- **Hosting targets.** One project, `doula-cloud`, with two Hosting sites: `doula-cloud` (marketing) and `doula-cloud-app` (app). The app site rewrites `/api/**` to Cloud Run `doula-api`. (`.firebaserc`, `firebase.json`.)
- **BAA is per project.** The HIPAA BAA is accepted on project `doula-cloud` only; "Another project that holds PHI needs its own acceptance." (`docs/research/identity-platform-tier-and-baa.md`, "Correction, 2026-09-25".)

## 2. Multi-tenancy

**What a tenant is.** "Using tenants, you can create unique silos of users and configurations within a single Identity Platform project." Each tenant has its own users, identity providers and authentication methods, auditing and IAM configuration, quota and usage breakdown. ([Multi-tenancy concepts](https://docs.cloud.google.com/identity-platform/docs/multi-tenancy).)

**What a tenant can configure.** The Tenant resource's fields are `displayName`, `allowPasswordSignup`, `enableEmailLinkSignin`, `disableAuth`, `hashConfig`, `enableAnonymousUser`, `mfaConfig`, `testPhoneNumbers`, `inheritance`, `recaptchaConfig`, `smsRegionConfig`, `autodeleteAnonymousUsers`, `monitoring`, `passwordPolicyConfig`, `emailPrivacyConfig`, `client`. There is **no `authorizedDomains`** on a tenant. ([REST: projects.tenants](https://docs.cloud.google.com/identity-platform/docs/reference/rest/v2/projects.tenants).)

**Signing in to a tenant.** The web client sets `auth.tenantId = "TENANT_ID"` before signing in; the value does not survive a reload. ([Signing in to tenants](https://docs.cloud.google.com/identity-platform/docs/multi-tenancy-authentication).) The ID token then carries the tenant in its `firebase.tenant` claim (the Go SDK's `FirebaseInfo.Tenant`, `json:"tenant"`, [auth.go](https://github.com/firebase/firebase-admin-go/blob/master/auth/auth.go)).

**Verifying a tenant's token, and the asymmetry.** In the Firebase Admin Go SDK, `verifyIDToken` refuses a token whose tenant differs from the client's **only when the client has a tenant ID**: `if c.tenantID != "" && c.tenantID != decoded.Firebase.Tenant`. ([auth.go](https://github.com/firebase/firebase-admin-go/blob/master/auth/auth.go).) So:

- An ops service using `TenantManager.AuthForTenant(opsTenant)` refuses Staff tokens (tenant empty).
- `doula-api` uses a project-level client, and a grep for `Firebase.Tenant` in `api/` finds nothing. **`doula-api` would accept an operator tenant's ID token** at `POST` session create (`api/internal/session/session.go:67`) and reauth (`api/internal/staffauth/reauth.go:49`). It would mint a Staff-tier session; `requireSelf` would then 404 on most routes for lack of a `staff` row, but the session would exist. A tenant isolates the Practice side only if `doula-api` also refuses a non-empty `firebase.tenant`.

**Turning tenants on.** The project config's `multi_tenant.allow_tenants` must be true. ([Terraform `google_identity_platform_config`](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/identity_platform_config.html.markdown).) The Identity Platform config "is created only once during initialization and cannot be deleted" and "may only be created in billing-enabled projects" (same doc). The repo does not manage it in Terraform today (`docs/infrastructure.md:70`), so bringing the tenant under Terraform means importing the config resource first, or flipping `allow_tenants` by hand.

**Terraform support.** The GA provider has `google_identity_platform_tenant` with `display_name`, `allow_password_signup`, `enable_email_link_signin`, `disable_auth`, `client.permissions`, and nothing else. **No MFA config and no password policy.** The `google-beta` provider's copy of the same resource doc has no MFA argument either. ([tenant doc, google](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/identity_platform_tenant.html.markdown); [same, google-beta](https://github.com/hashicorp/terraform-provider-google-beta/blob/main/website/docs/r/identity_platform_tenant.html.markdown).) Tenant-level TOTP must be set through the REST API (`PATCH .../tenants/TENANT_ID?updateMask=mfaConfig`) or the Admin SDK (`tenantManager().updateTenant`). ([Enable TOTP MFA](https://docs.cloud.google.com/identity-platform/docs/admin/enabling-totp-mfa).) The Go SDK supports this too: `TenantToUpdate.MultiFactorConfig`. ([tenant_mgt.go](https://github.com/firebase/firebase-admin-go/blob/master/auth/tenant_mgt.go).) Project-level MFA, including `totp_provider_config.adjacent_intervals`, is in Terraform's `google_identity_platform_config.mfa`.

**Price.** "All tenants are billed as part of their Identity Platform project." ([Monitoring billing for tenants](https://docs.cloud.google.com/identity-platform/docs/multi-tenancy-billing).) The pricing page lists no tenant charge. Tier 1 (email, phone, anonymous, social): 0–50,000 MAU free, then $0.0055 per MAU to 100,000, $0.0046 to 1,000,000, falling to $0.0025 at 10,000,000+. Tier 2 (OIDC, SAML): 0–50 MAU free, then $0.015 per MAU. SMS for phone sign-in and MFA is priced per message; "The first ten SMS that you send per day are not billed." No TOTP charge is listed. ([Identity Platform pricing](https://cloud.google.com/identity-platform/pricing).) One operator is one MAU, inside the free 50,000 either way.

### Separate GCP project instead of a tenant

Not a documented Identity Platform feature; it is a second project with its own Identity Platform config. Facts that bear:

- A new Identity Platform config needs a billing-enabled project and cannot be deleted once created (Terraform config doc above).
- The BAA is per project; a project holding PHI needs its own acceptance (`docs/research/identity-platform-tier-and-baa.md`). Whether ops holds PHI is not this ticket's question.
- Firebase Hosting's Cloud Run rewrite block takes `serviceId`, `region` and `pinTag` and no project field ([Hosting + Cloud Run](https://firebase.google.com/docs/hosting/cloud-run)), so a rewrite reaches a service in the Hosting site's own project. A separate project means its own Hosting site, its own Cloud Run service, its own IAM and Terraform state or workspace.
- The ops service would verify tokens with a client for that project; `doula-api`'s project-level verifier checks the `aud` claim against its own project ID, so it would refuse the other project's tokens without any code change. (Firebase ID tokens carry the project ID as `aud`; [Verify ID tokens](https://firebase.google.com/docs/auth/admin/verify-id-tokens).)

### Same project, no tenant

The operator's Identity Platform user sits in the same pool as Staff. Nothing forces a `staff` row: an account created with the Admin SDK has none. Facts that bear:

- `doula-api` would accept the operator's ID token and mint a Staff-tier session, then `404` on most routes (no `staff` row). The ops service would have to tell its operator apart by UID or email allowlist, the `FOUNDER_STAFF_ID` pattern.
- Setting project-level MFA to `MANDATORY` would bind every Staff member; ADR-0026 makes MFA a per-Practice switch enforced in the BFF. So MFA for the operator would be enforced in the ops service from `firebase.sign_in_second_factor`, as `FounderOnly` does now.
- No new Identity Platform configuration and no Terraform change.

## 3. Second factors

- **Offered:** SMS and TOTP. TOTP has been GA since 2023-09-21. ([Release notes](https://docs.cloud.google.com/identity-platform/docs/release-notes).) TOTP needs a provider that supports MFA and verified email ("MFA requires email verification"), and `adjacentIntervals` 0–10, default 5. ([Enable TOTP MFA](https://docs.cloud.google.com/identity-platform/docs/admin/enabling-totp-mfa).)
- **Not offered:** WebAuthn, FIDO, passkeys, security keys. The authentication concepts page lists none of them ([Authentication concepts](https://docs.cloud.google.com/identity-platform/docs/concepts-authentication)), and no release note mentions them through the latest entry, 2024-12-03 ([Release notes](https://docs.cloud.google.com/identity-platform/docs/release-notes)). This matches `docs/research/passkeys-for-staff.md` §1 and ADR-0026's passkey section.
- **Requiring one.** `MultiFactorAuthConfig.state` is `DISABLED`, `ENABLED` or `MANDATORY` ("MFA is required, so users must complete a second factor"), on the project config and on a tenant. ([REST: projects.tenants](https://docs.cloud.google.com/identity-platform/docs/reference/rest/v2/projects.tenants); [Terraform config doc](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/identity_platform_config.html.markdown).) The TOTP how-to shows only `ENABLED` and leaves enrollment to the app. Whatever the provider says, the repo already enforces the second factor in the BFF from the token claim (`founder.go`), which works the same in any of the options above.
- **Passkeys or a hardware key for the operator, two routes:**
  1. **BFF-owned WebAuthn.** The design in `docs/research/passkeys-for-staff.md`: credential and challenge tables in Postgres, the challenge returned in the response body because only `__session` survives Hosting, and the ceremony ending in the same session mint. ADR-0026 deferred it for Staff because "the relying-party ID binds permanently to the launch domain". For ops the RP ID would be `ops.doula.cloud` (or `doula.cloud`, which would also cover the app's subdomain). ADR-0026 also records that a syncable passkey alone is single-factor under NIST SP 800-63B-4.
  2. **A Google account behind Identity-Aware Proxy.** IAP can be enabled directly on a Cloud Run service with `--iap` or Terraform `iap_enabled = true`. ([IAP for Cloud Run](https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run).) The Cloud Run release notes list this as GA on 2026-03-13 (seen in a search result of [Cloud Run release notes](https://docs.cloud.google.com/run/docs/release-notes), not fetched in full). A project with no organization needs a custom OAuth client. IAP passes the user in a signed JWT, `x-goog-iap-jwt-assertion`, with `sub` and `email`, which the app should verify. ([Signed headers](https://docs.cloud.google.com/iap/docs/signed-headers-howto).) The second factor is then Google's: a personal account in the Advanced Protection Program must sign in with a passkey or security key. ([Advanced Protection FAQ](https://support.google.com/accounts/answer/7539956).) This is outside Identity Platform entirely. Inference, not checked: IAP sees the browser's Google identity only on a direct request, so it would sit on `run.app`, a domain mapping or a load balancer, not behind a Firebase Hosting rewrite.

## 4. Authorized domains and the session cookie

- **Authorized domains are per project**; the Tenant resource has no such field (REST reference above). Terraform describes `authorized_domains` as the "List of domains authorized for OAuth redirects" ([config doc](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/identity_platform_config.html.markdown)). Projects created after 2025-04-28 no longer include `localhost` by default ([Sign in with Google](https://docs.cloud.google.com/identity-platform/docs/web/google)). The Staff flow is email and password with no redirect, and DoulaCloud sends its own auth mail (ADR-0026). Whether `ops.doula.cloud` must be added for an email-and-password sign-in was not tested here; the cost of adding it is one entry on the existing project, edited by hand (`docs/infrastructure.md:70`).
- **Hosting forwards only `__session`.** "Cookies are generally stripped from incoming requests" when Hosting fronts Cloud Functions or Cloud Run; `__session` is the exception, and it becomes part of the cache key. ([Manage cache behavior](https://firebase.google.com/docs/hosting/manage-cache).)
- **A second Hosting site is cheap.** Up to 36 sites per Firebase project; each custom domain connects to one site. ([Multiple sites](https://firebase.google.com/docs/hosting/multisites); [Custom domain](https://firebase.google.com/docs/hosting/custom-domain).)
- **Two `__session` cookies do not collide.** `NewSessionCookie` sets no `Domain` (`api/internal/authn/authn.go:72-81`), so each is host-only: the browser returns it only to the exact host that set it ([RFC 6265 §5.3](https://www.rfc-editor.org/rfc/rfc6265#section-5.3), step 6). An ops service setting its own `__session` on `ops.doula.cloud` leaves the app's cookie alone, and ADR-0026's one-tier-per-browser eviction does not reach across hosts. The ops service needs its own sessions table or its own rows; it cannot read the app's cookie.
- **The limit is Hosting's, not the browser's.** A Cloud Run service reached directly (domain mapping, load balancer, or IAP) sees every cookie. The `__session`-only rule applies only if ops is fronted by a Hosting rewrite.
- **Identity Platform's own session cookies** have worked for tenants since 2022-05-01 ([Release notes](https://docs.cloud.google.com/identity-platform/docs/release-notes)); the repo does not use them, because ADR-0004 keeps sessions in Postgres.

## 5. The options side by side

| | A. Same pool, allowlisted UID | B. Operator tenant | C. Separate GCP project | D. IAP + Google account |
| --- | --- | --- | --- | --- |
| Operator kept apart from Staff | By ops-side allowlist only | Yes, separate user pool | Yes, separate project | Yes, not in Identity Platform at all |
| `doula-api` change needed | None, but it would mint a session for the operator's token | Refuse a non-empty `firebase.tenant` | None (`aud` differs) | None |
| Second factor | TOTP, enforced in ops BFF | TOTP; tenant can be `MANDATORY` | TOTP; project can be `MANDATORY` | Passkey or security key via Advanced Protection |
| Passkey / hardware key | Only by a BFF build | Only by a BFF build | Only by a BFF build | Yes |
| Terraform | Nothing new | Tenant yes; its MFA no (REST/SDK); config import to flip `allow_tenants` | Full new stack; config undeletable | `iap_enabled` on Cloud Run; OAuth client for a no-org project |
| Price | $0 | $0 | $0 for auth; a second project's other resources | IAP price not confirmed here |
| Hosting `__session` | Fits | Fits | Fits | Not behind Hosting |
| Local dev | Auth emulator as today | Emulator supports tenants (not checked) | Second emulator project | No emulator; a dev bypass needed |

**Recommendation (not a decision):** B, an operator tenant in `doula-cloud`, keeps one project and one BAA, keeps the email-and-password plus TOTP flow the founder already uses, and needs two small extras: `doula-api` refusing tenant tokens, and tenant MFA set by an Admin SDK or REST call recorded in `docs/infrastructure.md` beside the other hand-managed settings. D is the only route to a hardware key without building WebAuthn, at the cost of leaving the Hosting and emulator setup the rest of the repo uses.

## Not answered here

- Whether the Auth emulator supports tenants and tenant MFA locally (#2409 lists local dev as unspecified).
- IAP's price, and whether IAP works for a personal (non-Workspace) Google account on a project with no organization beyond the OAuth-client note.
- Whether `ops.doula.cloud` must be an authorized domain for an email-and-password sign-in.
