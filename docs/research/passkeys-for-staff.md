# Passkeys for Staff — feasibility and the shared-device case

Research for GitHub issue [#171](https://github.com/markgoho/doula-cloud/issues/171), part of the auth-methods map [#164](https://github.com/markgoho/doula-cloud/issues/164). Question: can Staff sign in with passkeys, what would it take to build, and does the shared-birth-center-laptop reality of doula work break them?

## Verdict: defer

Passkeys are not a provider toggle here — Identity Platform does not support WebAuthn, so adopting passkeys means the BFF builds and owns the whole ceremony. That is buildable, and ADR-0004 makes it cheaper than it would otherwise be: the BFF already owns session storage in Postgres, so a credential table sits naturally beside `sessions`, and a successful assertion mints exactly the `__session` cookie every other path mints. The shared-laptop case does not break passkeys either — cross-device (hybrid/QR) authentication is the flow FIDO Alliance's own UX guidance recommends specifically for shared and public computers, and it never lands the private key or a resident credential on the shared machine.

The reason to defer rather than adopt now is scope, not feasibility: this is new server-side cryptographic ceremony code (registration, assertion, challenge lifecycle, clone-detection via signature counters), a library still at v0 with no stability guarantee, and a credential-management UI — none of which exists yet, and the last of which is currently blocked by the repo-wide "no new components" freeze ([#518](https://github.com/markgoho/doula-cloud/issues/518)). Doula Cloud also has no password-reset or transactional-email infrastructure yet (per #164's map), which is the same gap passkey recovery ultimately falls back into — building passkey recovery well means building that infrastructure first, or building it twice. Reject is too strong: nothing here is blocked by the provider, the design is sound, and it is the strongest available answer for a population holding other people's health records. Defer until email/password plus the recovery path it depends on is solid, then revisit — at that point the credential table and challenge table below are most of the schema work already done.

## 1. Does Identity Platform support WebAuthn?

No. Confirmed against Google Cloud's live documentation, not from recall.

Google Cloud's Identity Platform "Authentication" concepts page lists every sign-in method the product supports: email/password, phone number (SMS), the federated providers Google, Facebook, Twitter, GitHub, Apple, and Microsoft, SAML, OpenID Connect, custom auth systems, and anonymous auth. WebAuthn, FIDO2, security keys, and passkeys appear nowhere on that list. [docs.cloud.google.com/identity-platform/docs/concepts-authentication](https://docs.cloud.google.com/identity-platform/docs/concepts-authentication)

This is not a recent gap. The public Firebase/Cloud Identity Platform issue tracker carries a passkey feature request (Firebase's own engineers describe it as one of Firebase's top-10 most-requested features, and the one with the most votes in the public Cloud Identity Platform tracker) that remains open with no shipped support and no committed timeline. [github.com/firebase/firebase-android-sdk#6981](https://github.com/firebase/firebase-android-sdk/issues/6981)

The workaround some integrators use — a third-party WebAuthn relying party (e.g., Beyond Identity) that authenticates the passkey ceremony itself and then hands Identity Platform a token over OIDC — is available, but it adds a vendor and a second BAA question on top of the one ADR-0004 already resolved for Identity Platform, for a product that already decided (ADR-0004, resolved in #181) that behind a one-method interface the provider should stay a vendor preference, not a growing integration surface. It is not evaluated further here.

## 2. If the BFF has to own it, what does it cost?

### The shape ADR-0004 already gives this

`api/internal/authn/store.go` already has the primitive this needs: `MintSession(ctx context.Context, q Querier, identityUID string, now time.Time) (*http.Cookie, error)`. It sweeps expired sessions, generates a token, inserts a `sessions` row keyed by `identity_uid`, and returns the `*http.Cookie` built by `NewSessionCookie`. `session.CreateHandler` (`api/internal/session/session.go:50-77`) calls it today after verifying a Bearer ID token from Identity Platform:

```go
verified, err := verifier.VerifyIDToken(r.Context(), idToken)
...
cookie, err := authn.MintSession(r.Context(), db, verified.UID, now)
...
http.SetCookie(w, cookie)
```

A passkey assertion handler would do the same thing after verifying a WebAuthn assertion instead of an ID token: resolve the `identity_uid` the credential belongs to, call the same `authn.MintSession`, set the same cookie. This is not a hypothetical — it is the exact function signature already in the repo, already the seam `staffauth.SessionHandler` and every route behind `authn.Begin` reads through. Nothing downstream of the cookie would know or care which ceremony minted it.

### The constraint the ticket omits: no second cookie

`authn.go`'s own comment on `SessionCookieName` names the reason: since #139 the deployed app reaches the BFF through a Firebase Hosting rewrite of `/api/**` to Cloud Run, and "Hosting strips every incoming Cookie header on that hop except one named exactly `__session`." A WebAuthn ceremony normally needs a short-lived, server-generated challenge held somewhere between its "begin" and "finish" calls — commonly a second cookie or a server-side session keyed by one. Neither is available here: any cookie this feature mints beyond `__session` is discarded in production on the very next request.

The challenge therefore has to be a table, not a cookie, and the client has to hand the challenge identifier back explicitly rather than have it ride implicitly on a cookie:

- `begin` (registration or authentication) generates a random challenge, inserts a row keyed by a random opaque `challenge_id`, and returns `{ challengeId, publicKeyCredentialOptions }` in the JSON response body.
- The frontend holds `challengeId` in memory for the few seconds the ceremony takes (`navigator.credentials.create` / `.get`), then POSTs it back explicitly alongside the attestation or assertion to `finish`.
- `finish` looks the row up by `challenge_id`, checks it is unexpired and of the right purpose, consumes it (one-time use, deleted on success or expiry), and only then verifies the cryptographic response.

This is the same pattern `sessions` already uses for the credential itself — an opaque server-side token the client presents back explicitly — just applied to the challenge instead of the session, and for the same reason: nothing rides on an implicit cookie that Hosting will strip.

### Schema

Two tables, following the `sessions` table's own conventions (00028_sessions.sql): `identity_uid` as the join key rather than `staff_id`, because this serves the same two populations `sessions` does, and no row-level security, for the same reason `sessions` has none — this is BFF infrastructure that runs before `app.current_identity_uid` has a value.

```sql
CREATE TABLE webauthn_credentials (
    credential_id   bytea PRIMARY KEY,       -- from the authenticator, base64url in transit
    identity_uid    text NOT NULL,
    public_key      bytea NOT NULL,
    sign_count      bigint NOT NULL DEFAULT 0,  -- clone/replay detection
    transports      text[] NOT NULL DEFAULT '{}',
    backup_eligible boolean NOT NULL,         -- synced-capable per the attestation flags
    backup_state    boolean NOT NULL,         -- currently backed up (synced) right now
    label           text NOT NULL,            -- "iPhone", set at registration so a person
                                               -- can tell their passkeys apart later
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_used_at    timestamptz
);
CREATE INDEX webauthn_credentials_identity_uid_idx ON webauthn_credentials (identity_uid);

CREATE TABLE webauthn_challenges (
    challenge_id  text PRIMARY KEY,      -- rand.Text(), same primitive sessions.token_hash uses
    identity_uid  text,                  -- null until an assertion resolves it (discoverable flow)
    purpose       text NOT NULL,         -- 'registration' | 'assertion'
    challenge     bytea NOT NULL,        -- the actual WebAuthn challenge bytes
    expires_at    timestamptz NOT NULL,  -- short TTL, minutes not hours
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webauthn_challenges_expires_at_idx ON webauthn_challenges (expires_at);
```

`go-webauthn/webauthn` is the natural library choice — it is the Go ecosystem's standard implementation, handles attestation/assertion verification and the ceremony option structs, and deliberately leaves session/cookie issuance to the caller (confirmed against its own repository documentation), which matches this design exactly: it produces a verified credential and identity, and `authn.MintSession` takes it from there. [github.com/go-webauthn/webauthn](https://github.com/go-webauthn/webauthn) The caveat: it is versioned `v0.x`, and its own docs warn of breaking changes without notice — a real cost for a library this feature would depend on directly, worth pinning carefully and re-checking before this ships.

### Endpoints and RP ID

Four endpoints, mirroring the `sessions` package's split between the exchange and end handlers: `POST /api/passkeys/register/begin` and `/finish` (called by a signed-in Staff member from account settings — this is enrollment, not sign-in, so it runs behind `authn.Begin` like any other authenticated route), and `POST /api/passkeys/authenticate/begin` and `/finish` (called by a signed-out visitor, alongside the existing `session.CreateHandler`, since both are ways to arrive at the same `MintSession` call). A "list/delete my passkeys" pair belongs in account settings for the same reason session revocation does.

The relying party ID has to be the production domain: `doula-cloud-app.web.app` (from `docs/environment.md`'s `APP_BASE_URL`). Coexistence with Identity Platform is automatic rather than designed: `staff.identity_uid` is the join key regardless of which ceremony authenticated it, so a Staff member with no passkey keeps going through `session.CreateHandler` exactly as today, and both paths converge at the same `MintSession` call.

This RP ID is a one-way door worth flagging now rather than at ship time. The WebAuthn spec binds every credential to the RP ID it was registered under, and a credential registered for one RP ID is not usable for a different one — there is no re-parenting a credential to a new domain after the fact. If passkeys are enrolled against `doula-cloud-app.web.app` and Doula Cloud later launches on a custom domain, every enrolled passkey is orphaned and every Staff member who registered one has to re-enroll from scratch. Pre-launch with a January 2027 target, the practical implication is: settle the production domain before the first passkey is ever registered, not after.

**Scope, roughly:** two migrations, one new Go package (four handlers plus the store functions, on the model of `authn/store.go`), and frontend work — a passkey option on the existing login/signup flow and an enrollment screen in account settings. The frontend half is presently constrained by the repo's "no new components" freeze (#518): a passkey enrollment screen is new UI, not a retrofit of something existing, so it would need either an exemption or #518 closed first. This is a scoping fact for whoever picks this back up, not a reason to avoid the backend design work now.

## 3. The shared-device case: does cross-device authentication carry it?

The scenario the ticket names is concrete: a doula works from her own phone and, some shifts, a laptop that belongs to the birth center and is shared across staff. A platform (resident) passkey created on that shared laptop would be exactly wrong — the next person to sit down could sign in as her.

Cross-device authentication — hybrid transport in the WebAuthn spec's own terms, historically called caBLE — is designed for exactly this case, not adapted to it. `"hybrid"` is a named value of the spec's own `AuthenticatorTransport` enumeration, so this is a first-class transport, not a workaround. The mechanism: the shared laptop's browser shows a QR code; the doula scans it with her own phone's camera; the two devices confirm physical proximity over Bluetooth Low Energy; the phone (which holds the passkey) signs the challenge locally and the signed assertion crosses back to the laptop over the established link. The private key never leaves the phone — only the signed assertion crosses the link. [W3C WebAuthn Level 3, §5.8.4 `AuthenticatorTransport`](https://www.w3.org/TR/webauthn-3/#enumdef-authenticatortransport); [Corbado: WebAuthn Passkey QR Codes & Bluetooth](https://www.corbado.com/blog/webauthn-passkey-qr-code)

Chrome supports this cross-device flow on every platform, and it is exactly the flow FIDO's own passkey UX guidance recommends for this situation: its design-guidance page on cross-device sign-in reports that people specifically valued "the separation and transparency of the passkey-based QR sign-in when using a foreign or public device," describing it as "a safe way to access [an] account on a foreign device or a public device," and recommends supporting QR sign-in "as a fallback, especially for public or shared devices." [Passkey Central: Cross-Device Sign-In](https://www.passkeycentral.org/design-guidelines/optional-patterns/cross-device-sign-in) [Google for Developers: Passkey support on Android and Chrome](https://developers.google.com/identity/passkeys/supported-environments)

So the answer is yes, cross-device authentication carries the shared-laptop flow, with two caveats worth naming rather than glossing over:

- **Hardware.** This needs Bluetooth on both ends of the link — the shared laptop, to run the proximity check and display the QR code, and the phone, both for Bluetooth and for the camera it uses to scan that code. A birth center's laptop is plausibly older or IT-locked-down hardware; if Bluetooth is disabled or absent there, cross-device sign-in has no fallback except typing a password on the shared machine — which is the exact failure mode passkeys exist to remove. This should be checked against whatever hardware Doula Cloud's pilot birth centers actually use before this ships, not assumed.
- **The "remember this device" prompt.** Some platforms offer to pair a phone and a computer for faster future QR-less reconnection after a first cross-device sign-in. This is a device *pairing* convenience, not a credential copy — it does not create a resident passkey on the shared laptop — but it is exactly the kind of prompt a doula moving quickly through a shift could click through on shared hardware without meaning to establish an ongoing link. Product copy at that prompt, or disabling it outright on this flow, is worth deciding before shipping this, not after a birth center reports a strange "remembered device" on its shared laptop.

## 4. Synced versus device-bound

A **synced** passkey (iCloud Keychain, Google Password Manager) is backed up and replicated across a person's own devices by the platform account it is tied to; losing one device does not lose the credential. A **device-bound** passkey (a hardware security key, or a platform authenticator with syncing turned off) never leaves the single authenticator it was created on; losing that device loses the credential outright unless a second one was enrolled in advance. [passkeys.dev: Terms](https://passkeys.dev/docs/reference/terms/)

For Staff, **synced is the right default.** The population most likely to lose a phone mid-workday, or upgrade one, is exactly the population that cannot afford a locked-out account at 3 a.m. during a birth — device-bound's stronger isolation guarantee is not worth its recovery cliff for this population. Synced passkeys do widen the exposure surface slightly, since the private key is now replicated through Apple's or Google's own backend rather than confined to one chip, but that backend is the same one already trusted for the phone's screen lock, its email, and (for many Staff) its existing Google account — it is not a new trust boundary Doula Cloud is introducing.

Whether **Practice Owners** — who hold the most privileged role in a Practice and are the population #167 is already deciding an MFA position for — should be required or encouraged toward device-bound (or synced-plus-a-mandatory-second-factor) is a real question, but it is the MFA-position question #167 already owns, not a separate one this ticket should answer. It is noted here so #167 has it in view: if passkeys are adopted, "device-bound for Owners" is one lever available.

## 5. Recovery, and its weakest link

What happens when a Staff member loses the device holding her passkey has one honest answer right now: there is no better path than falling back to whatever proves her identity independent of any device, and Doula Cloud does not have that infrastructure yet. Per #164's own map, there is no transactional email today (`api/go.mod` has no mail dependency, `api/internal/` has no mail package) and no password-reset flow (`sendPasswordResetEmail` appears nowhere in `app/`). A synced passkey mitigates most loss scenarios by surviving a device swap automatically — but the case where it does not (a new phone signed into a fresh platform account, not a restore) still needs a fallback, and industry guidance is consistent that **recovery security is capped by the weakest fallback offered**, not by the passkey itself: "if you use super-secure passkeys but your fallback method is a simple email one-time password, then your overall security is only as strong as that email OTP." [MojoAuth: Your Account Security Is Only as Strong as Your Passkey Recovery Path](https://mojoauth.com/blog/account-security-passkey-recovery-path)

Concretely, the weakest link here would be whatever email account is on file — the same weak link email/password already has today, since Identity Platform's own password reset is itself an email flow. Passkeys do not make this worse, but they do not make it better either unless a second enrolled credential (a backup passkey, registered in advance) is required at enrollment time, which is the recovery pattern with no email dependency at all. That is a real design lever worth building into the enrollment flow above (`register/begin` and `/finish` naturally support enrolling more than one credential per identity) rather than an afterthought.

## 6. Sole factor or second factor?

A WebAuthn ceremony already combines two things in one step — possession of the authenticator, and a local unlock of it (biometric or PIN) — which is why the credential is commonly described as inherently phishing-resistant and stronger than a bare password. NIST's current digital-identity guidelines are explicit, though, that a syncable passkey used alone is still classified as a **single-factor cryptographic authenticator**: NIST SP 800-63B-4 states that reaching AAL2 (its two-factor assurance level) requires "proof of possession and control of two distinct authentication factors," and a syncable authenticator on its own is one of them, not two — the syncing and the local biometric/PIN unlock do not themselves add a second factor in NIST's framework. [NIST SP 800-63B-4, Digital Identity Guidelines — Authentication and Authenticator Management](https://pages.nist.gov/800-63-4/sp800-63b.html)

The recommended position, if adopted: a passkey is the **sole factor**, replacing email/password rather than sitting alongside it, the same way it replaces a password industry-wide. During any transition period, email/password would need to remain enrolled for Staff who have not yet set up a passkey — this repo's `identity_uid` join key makes that trivial, since a Staff row does not care which ceremony authenticated the identity behind it. Whether Owners additionally need a true second factor on top of a passkey is, again, #167's MFA-position question to answer, not this ticket's.

## Sources

- [Identity Platform: Authentication concepts](https://docs.cloud.google.com/identity-platform/docs/concepts-authentication) — Google Cloud, live docs, full sign-in-method list, no WebAuthn/FIDO2/passkey entry
- [firebase/firebase-android-sdk#6981](https://github.com/firebase/firebase-android-sdk/issues/6981) — public feature request for native passkey support, open, unresolved
- [go-webauthn/webauthn](https://github.com/go-webauthn/webauthn) — Go WebAuthn library, v0.x, session/cookie issuance left to the caller
- [W3C WebAuthn Level 3, §5.8.4 `AuthenticatorTransport`](https://www.w3.org/TR/webauthn-3/#enumdef-authenticatortransport) — `"hybrid"` as a spec-defined transport
- [Corbado: WebAuthn Passkey QR Codes & Bluetooth](https://www.corbado.com/blog/webauthn-passkey-qr-code) — hybrid transport mechanics
- [Google for Developers: Passkey support on Android and Chrome](https://developers.google.com/identity/passkeys/supported-environments) — Chrome cross-device support
- [Passkey Central: Cross-Device Sign-In](https://www.passkeycentral.org/design-guidelines/optional-patterns/cross-device-sign-in) — FIDO Alliance UX guidance, shared/public device recommendation
- [passkeys.dev: Terms](https://passkeys.dev/docs/reference/terms/) — synced vs. device-bound definitions
- [MojoAuth: Your Account Security Is Only as Strong as Your Passkey Recovery Path](https://mojoauth.com/blog/account-security-passkey-recovery-path) — recovery/weakest-link framing
- [NIST SP 800-63B-4, Digital Identity Guidelines — Authentication and Authenticator Management](https://pages.nist.gov/800-63-4/sp800-63b.html) — syncable authenticators classed single-factor, AAL2's two-distinct-factor requirement
- `api/internal/authn/authn.go` — `SessionCookieName` comment: the Hosting rewrite strips every cookie but `__session`
- `api/internal/authn/store.go`, `api/internal/session/session.go`, `api/db/migrations/00028_sessions.sql` — this repo's existing session design (ADR-0004)
- `docs/adr/0004-bff-owned-sessions.md` — the ADR this design extends
- `docs/environment.md` — production origin (`doula-cloud-app.web.app`)
