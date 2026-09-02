# Two populations, two sign-in methods, and one token table

Nobody had decided how a person proves who they are to Doula Cloud. What existed was a default: email and password wired into `app/`, a provider with no federation configured, no password reset, no email verification and no second factor. That default was never argued for, and it had already leaked into a ticket — [#149](https://github.com/markgoho/doula-cloud/issues/149) carried an acceptance criterion migrating "the same Google sign-in" to session cookies, for a flow that did not exist at either end.

This ADR records the decision. It was reached on the wayfinding map [Auth methods: what Doula Cloud supports, for whom, and why](https://github.com/markgoho/doula-cloud/issues/164) across one research ticket on the provider, one on passkeys, and five grilling tickets; each section below names the ticket that holds its full argument.

It settles *how a person authenticates*. It does not touch *who owns the session* — [ADR-0004](0004-bff-owned-sessions.md) settled that, and everything here is cheap precisely because it did: every path below ends at the same `authn.MintSession`, setting the same `__session` cookie, and nothing downstream can tell them apart.

## The axis every decision turns on: the two populations are not alike

**Staff** — doulas, Practice Admins, Practice Owners — are in the product most working days, often across a laptop shared at a birth centre and a personal phone. They read other people's health records, so a compromised Staff account costs the most.

**Clients** — the pregnant and birthing people the portal serves — are invited by email, use the portal occasionally across a nine-month relationship, and are the population least likely to remember a password set once at 8 weeks and next needed at 32.

Wherever the two get different answers below, this is why, and the difference is deliberate rather than an inconsistency to be tidied away later.

## The provider and its standing

Identity Platform stays, and the compliance question does not reopen. The project runs the upgraded `IDENTITY_PLATFORM` tier — read from the Identity Toolkit Admin API's `GetConfig`, not from the console label, which shows the same thing for both tiers. Google names **Identity Platform** on its HIPAA-covered products list and does not name **Firebase Authentication** anywhere on it, so the two do not share standing, and the Google Cloud Platform HIPAA Business Associate Addendum was reviewed and accepted for this account on **2026-08-30**. MFA, multi-tenancy, SAML, OIDC and the SLA are all available on this tier; the upgrade is one-way, with no supported downgrade.

One forward obligation, not a blocker: the BAA was accepted by a sole proprietor, who is a natural person under HIPAA's own definition (45 CFR §160.103) and is both the accepting party and the operating entity today. It has to be re-executed once the planned single-member LLC forms, because an LLC is a distinct contracting party from its member whatever its tax treatment. Tracked on [#546](https://github.com/markgoho/doula-cloud/issues/546).

Full analysis in `docs/research/identity-platform-tier-and-baa.md`, decided on [#165](https://github.com/markgoho/doula-cloud/issues/165).

## Every method, marked

**Staff**

| Method | Verdict | Why, in one line |
| --- | --- | --- |
| Email and password | **Adopted** | Daily use against a 12-hour session; the first factor a second factor is added to. |
| TOTP second factor | **Adopted** | Required for Owners, optional otherwise, raisable per Practice by its Owner. Launch-blocking. |
| SMS second factor | **Rejected** | Weaker than TOTP and additionally needs a reCAPTCHA verifier, which TOTP does not. |
| Google sign-in | **Rejected** | One click in a shared browser signs you in as whoever used it last. |
| Sign in with Apple | **Rejected** | Only forced by App Store Guideline 4.8, which binds only alongside another third-party login. |
| Magic link | **Rejected** | A mailbox round-trip every working morning, on a machine several people share. |
| Passkeys | **Deferred** | No provider support, and the relying-party ID binds permanently to a launch domain that is not settled. |

**Clients**

| Method | Verdict | Why, in one line |
| --- | --- | --- |
| Magic link, minted by the BFF | **Adopted** | The mailbox is the root of trust either way; a password adds a thing to forget and buys no assurance. |
| Password | **Rejected** | Would cost the reset flow it was meant to avoid, for the population least able to remember one. |
| An Identity Platform account | **Rejected** | With no password there is nothing to verify, so it would be an empty record kept in step for nothing. |
| Second factor | **Not applied** | Deliberate asymmetry; see [Two populations, one browser](#two-populations-one-browser-and-nothing-that-links-them). |

**Both**

| Mechanism | Verdict | Why, in one line |
| --- | --- | --- |
| BFF-minted, single-use link tokens | **Adopted** | One table, four purposes: magic link, verification, reset, MFA recovery. |
| Identity Platform's own mailer | **Rejected** | Sends outside [ADR-0010](0010-notification-email-delivery-is-an-outbox-not-in-request.md)'s outbox and [ADR-0011](0011-notification-sending-identity-is-one-shared-domain.md)'s sending identity, so its delivery is unobservable to us. |
| A backup email address for recovery | **Rejected** | NIST SP 800-63B-4 §3.1.3.1 forbids email as an authentication channel outright. |

## Staff keep a password, and there is no federation at all

Decided on [#167](https://github.com/markgoho/doula-cloud/issues/167).

The live Identity Platform config for `doula-cloud` returns `signIn: {"email": {"enabled": true, "passwordRequired": true}}` and an **empty** `defaultSupportedIdpConfigs`. Google sign-in was therefore absent at both ends — no `GoogleAuthProvider` in `app/`, and no provider in the project either. Rejecting it removes nothing; there was never a toggle to turn off, and the console already matches what this ADR adopts.

Three things outweigh the case for it — that small practices live in Gmail, so it removes a password:

- **The shared birth-centre laptop turns it from a convenience into a hazard.** A "Sign in with Google" button in a browser already signed into someone's Google account is one click with no prompt, so the fastest path through the door is signing in as whoever used it last. That is a wrong-person hazard in a product holding other people's health records.
- **It is the sole cause of the account-linking problem.** With one route in, an address cannot arrive twice, and the whole class of linking bugs never exists to be solved.
- **It widens a surface just deliberately narrowed**, for a pilot population of roughly fourteen people, at the cost of an OAuth consent screen and a verified brand.

This is **provisional on one fact nobody has**: whether the pilot agency runs Google Workspace and expects Google sign-in. Nobody has asked for it, and the question is now on [#243](https://github.com/markgoho/doula-cloud/issues/243) rather than left as a silent assumption.

**Apple is rejected with its reopening condition stated**, so it needs no revisiting on its own schedule. App Store Review Guideline 4.8 binds only when an app in the App Store already offers another third-party login. Doula Cloud is a PWA, and with Google rejected there is no third-party login for the rule to match. Apple returns only if **both** an App Store build and some other federated provider happen; either alone changes nothing.

**Staff do not get the Client's magic link.** A Staff member signs in most working days against a 12-hour session, so a magic link makes that a mailbox round-trip every working morning — and on the shared laptop it teaches people to open a personal mailbox on a shared machine, which is worse than typing a password. Note the argument that does *not* carry this: "a magic link is single-factor" is false here, because Identity Platform challenges a second factor on email-link sign-in exactly as it does on password sign-in. The daily round-trip and the shared device are the reasons.

## TOTP, required for Owners, and enforced at the Practice boundary

Decided on [#167](https://github.com/markgoho/doula-cloud/issues/167) and its same-day amendment. This is launch-blocking.

**The factor is TOTP — a code from an authenticator app — not SMS.** TOTP has been generally available on Identity Platform since 2023-09-21 and needs Web SDK 9.19.1 or later; `app/` runs well past it. SMS is both the weaker factor and the more expensive one, additionally requiring a reCAPTCHA verifier that TOTP does not.

**Who must hold one is decided per Membership, not per person:**

- **Owner: required.** A Practice has at least one, and it carries full authority over that Practice's Staff, Plan Templates and billing.
- **Admin and Doula: optional**, enrolment offered rather than forced.
- **Plus a per-Practice switch.** An Owner may turn on "require MFA for all staff" in her Practice's settings, promoting every Membership at that Practice to required.

The blanket rule — every Staff account, because every Doula reads client health records — was proposed and dropped. It is true, and so is the thing it traded away: a Practice whose doulas do not all carry a smartphone cannot function under it, and Doula Cloud does not know which Practices those are. Its Owner does. A required floor on the account that can change anything, plus an Owner's switch to raise it, gets the posture without guessing at fourteen people's phones.

**Enforcement is at the Practice-scoped boundary, not at session mint.** This follows from the domain model rather than from taste: a role is held by a **Membership**, and a Membership is per-Practice, so the same identity may be Owner at her own solo Practice and Doula at the agency down the road. At mint time the request names no Practice, so there is no Membership to read a role from and no switch to read a Practice's posture from. The rule is therefore: **refuse to serve a Practice-scoped request when the caller's Membership at that Practice is Owner, or that Practice has the switch on, and the caller's token shows no second factor.** Signing in stays open to everyone; what is gated is entering a Practice that demands it. That boundary is where [ADR-0006](0006-read-follows-the-role.md)'s reach rules already resolve the Membership.

**The token fact it reads is `firebase.sign_in_second_factor`.** The Firebase Admin Go SDK strips only `iss`, `aud`, `exp`, `iat`, `sub` and `uid` from `Token.Claims`, so the whole `firebase` claim survives as a map, reachable by the same comma-ok lookup `authn.FirebaseVerifier.VerifyIDToken` already does for the email claim. The claim describes the sign-in event, and Identity Platform challenges the second factor on **every** sign-in once one is enrolled — which is what makes it a sound proxy for "this person has MFA", rather than merely a record of one occasion.

**Enrolment is per person, not per Practice.** A contractor enrols once, to get into the Practice that requires it, and then has MFA everywhere she works. The UI must not imply otherwise. And **throwing the switch can lock people out immediately** — it instantly bars every un-enrolled Doula at that Practice, possibly mid-shift — so the switch says how many people it will affect before it is thrown, and the throw lands in [ADR-0022](0022-one-activity-log-with-a-subject-and-three-kinds-of-actor.md)'s activity log with who threw it and when.

Enforcement in the BFF rather than as a browser prompt is the point. Google's own guidance for mandatory MFA is a client-side check of `user.multiFactor.enrolledFactors`, which is a prompt, not a refusal; there is no project-level "require MFA" flag to lean on either, since `multiFactorConfig.state` only says whether MFA is available on the project at all.

## Clients get a magic link, and no account with the provider

Decided on [#166](https://github.com/markgoho/doula-cloud/issues/166) and its correction.

**A Client has no password.** A password for this population is not a second factor — it is a first factor guarded by a reset flow that arrives by email, so the mailbox is the root of trust either way. Adding one adds a thing to forget, a thing to reuse, a reset flow to build and a second sign-in path to support, and adds no assurance the mailbox did not already give. An optional password was rejected for the same reason: it costs the reset flow the decision was avoiding, for a population that did not ask for it.

**The link is ours, not Identity Platform's.** She types her email address; the BFF makes a high-entropy token, stores a hash of it with an expiry, and enqueues an outbox row. She opens the mail, presses **Continue**, and the token is spent on a **POST** — the BFF verifies the hash, marks the token used, and calls `authn.MintSession`.

Identity Platform's `sendSignInLinkToEmail` was rejected, and the reason has been narrowed to the one that actually carries it: **its mail sends outside ADR-0010's outbox and ADR-0011's shared sending identity, so its delivery is unobservable to this product and unretried by its machinery.** Two other objections were raised on the ticket and are recorded here as *not* load-bearing, so that a Firebase-literate reader does not knock the decision down with them: the `firebaseapp.com` action page can be moved to a custom domain, and `signInWithEmailLink` does work cross-device if the app asks for the address again on the second device. Those are conveniences avoided, not obstacles cleared.

Four properties of the link, each with its reason:

- **15 minutes.** Mailgun retains message logs as a sold plan feature ([ADR-0012](0012-mailgun-is-vendor-no-baa-needed-for-v1.md)), so the token sits in a vendor's records for longer than it sits in her inbox. A short life is what limits that exposure.
- **Single use**, marked spent inside the same transaction that mints the session.
- **Spent on the POST behind a Continue button**, never on the GET. Mail clients and security scanners follow links to inspect them, and a scanner must not burn a Client's link before she reads the mail.
- **Not bound to a device**, and this is intended rather than a gap. It cannot be: the `/api/**` rewrite strips every cookie but `__session`, so no marker can be left in the requesting browser — and reading mail on a phone while using the portal on a laptop is the normal case.

**Identity Platform becomes Staff-only.** With no password there is nothing for it to verify for a Client, so a provider account would be an empty record kept in step for nothing. Doula Cloud issues the Client's own identifier instead.

**The invitation *is* the first magic link.** The accept-invite screen currently asks her to choose a password; with no password there is nothing to ask, so she presses Continue and lands in the portal with no account-setup step. Making her prove a mailbox we just proved she holds buys nothing. The invitation token gets the expiry it has never had — **7 days**, longer than a sign-in link because she may not read mail for days, and re-sendable by her doula.

**A 30-day rolling session, for Clients only.** Staff keep 12 hours. Occasional use across nine months against a 12-hour session means "check your email" at nearly every visit, which lands the cost of a passwordless portal on the person least willing to pay it. Staff share a laptop and read many people's records, so the short session there is deliberate and is not an inconsistency to be made uniform.

Two more, both nearly free: **sign out everywhere** from inside the portal, reusing `authn`'s delete-by-`identity_uid` query, and the existing **Staff revoke**, which already covers a lost phone.

**No email-verification step for Clients**, because a magic link proves control of the mailbox on every sign-in, which is strictly stronger than verifying it once.

Two things settled as form, recorded so they are not re-litigated: the request screen **says the same thing whether or not the address exists** ("If that address is on our records, we have sent a link"), because anything else tells a stranger which people use the product; and the wait screen **says delivery can take a minute**, since the link rides the outbox.

**A portal account is the pregnant person alone.** Some Clients share a mailbox with a partner, and that is an accepted risk rather than one magic links introduced — a partner clicking "forgot password" gets in the same way. A partner who needs genuine access needs their own invitation, which is a product feature and not a sign-in method.

## The Portal Account becomes a table, and the prefix is the namespace

This is the schema consequence of the decision above, and it was nearly missed.

**Today a Client's sign-in address is not stored in this product at all.** It lives in Identity Platform, which is exactly why `client_portal_users` holds only `identity_uid`, `client_id` and `created_at`. Take Identity Platform out of the Client path and Doula Cloud must hold that address itself, because it is what a magic-link request is looked up by.

It does not belong on `client_portal_users`. [ADR-0015](0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md) makes the **Portal Account** one person's login reaching many Clients, at most one per Practice — one identity across several rows of that table — so an address repeated per row would store one fact many times, which is the failure `CONTEXT.md` names on the Client entry itself: *a fact the system sends email with cannot live in two places.* **So the Portal Account becomes a real table**, holding the identifier and the sign-in address, with `client_portal_users` referencing it. It has existed in the model and in ADR-0015 all along and has been implied in the schema only by rows happening to share an `identity_uid`.

**The identifier is a prefixed UUID, and the prefix is the sanctioned way to tell the two populations apart.** `client_portal_users.identity_uid` is `text`, so nothing changes shape. The alternative — a tier column on `sessions` — is rejected: the prefix is not a hint about the identifier, it *is* the namespace it was issued in, total and deterministic because every uid is minted by exactly one of two issuers. A tier column would be a second place holding the same fact, free to disagree with the first. Two things read it, which is the argument for naming it here rather than leaving it to a build: the session-lifetime branch (30 days rolling against 12 hours) and the eviction check below.

**A Staff edit to a Client's contact email does nothing to portal access.** ADR-0015 already separated the two — *"The address the Practice reaches her at is the Practice's own contact detail, not her login"* — so the takeover risk that would have justified revoking access on an edit does not exist, and a rule revoking it would have forced a re-invitation every time a doula fixed a typo. **She changes her sign-in address herself, from inside the portal**, confirmed by a link sent to the **new** address, with the old one live until the new one is proved. The invitation is the single point where the two addresses meet: it goes to the contact address, because that is the only address known at that moment, and accepting it proves that mailbox, so the Portal Account's sign-in address defaults to it.

## One token table, four purposes — and Doula Cloud is the post office

Decided on [#169](https://github.com/markgoho/doula-cloud/issues/169).

**Doula Cloud sends Staff verification and password-reset mail itself, from its own token machinery. Identity Platform stays the credential store and stops being the post office.** It keeps everything only it can do — the password hash, the `emailVerified` flag, token verification, TOTP — and no credential is hashed or stored by this product.

Three options were weighed:

- **Identity Platform's own mailer** keeps the provider surface at exactly one method and fails all three of #166's objections: outside the outbox, outside the shared sending identity, unobservable, and redirecting through `https://doula-cloud.firebaseapp.com/__/auth/action` with `dnsInfo.customDomainState` at `NOT_STARTED`. Rejected; verification is not different in kind from a sign-in link.
- **The Admin SDK generates the link and we mail it**, lifting the `oobCode` onto our own URL. This clears all three objections. Rejected on two counts: the `oobCode`'s expiry clock starts at generation, so ADR-0010's deliberate outbox delay eats into a window we do not control, and it leaves a third link mechanism standing beside the two we already have.
- **The BFF mints its own token, then sets the flag** on spend. Adopted.

**What made the third cheaper rather than merely different is one token table with four purposes** — the Client magic link, Staff verification, Staff reset, and MFA recovery — sharing a purpose column and a per-purpose expiry. One expiry policy, one single-use rule, one rate limit, one place to look when a link does not arrive. Generating `oobCode`s would have added a mechanism where this reuses one. It also makes the action page's custom-domain state **permanently irrelevant**, because no Doula Cloud link ever points at it.

**Verification is self-signup only.** Accepting a Staff invitation sets `emailVerified` directly and mails nothing: the invitee received the invitation at that address and clicked it, and `acceptInvite` already refuses with 403 unless the token's `email` claim matches the invited address. Holding the invite token *is* the mailbox proof. A code finding makes this sound rather than merely convenient — `VerifyIDToken` reads `token.Claims["email"]` and **never reads `email_verified`**, and nothing in `api/` has ever checked a verified flag, so accept's security has always rested on the invite token alone.

**Verification gates nothing new.** Signup creates an Owner; Owners need MFA; Identity Platform refuses second-factor enrolment without a verified address (*"Multi-factor users must always have a verified email address"*). So signup mints a session, the Practice-scoped boundary refuses until MFA, and MFA refuses until verified. **One gate, and it is the Practice gate**, not a second one bolted on at signup.

**Password reset lands the same way plus three properties**, and the first is the most important line in this ADR:

- **Spending a reset token mints no session.** Identity Platform's own reset does not sign you in either — you reset, then sign in, then get challenged. A BFF-minted reset that set `__session` directly would walk straight past the enforced MFA above.
- **A successful reset ends every existing session for that identity**, reusing `staffauth/endsessions.go` rather than building a second revoke path.
- **The request endpoint answers identically for a known and an unknown address**, because an account-enumeration oracle is a class this product refuses.

Lifetimes: **verification 24 hours** (it grants nothing but a flag, and 24 hours survives an outbox retry cycle), **reset 1 hour** (it grants a credential change, matching Identity Platform's own default). A signed-in but unverified Staff member can **request a fresh verification link**, because the 24-hour lifetime and the retry window are roughly the same length, so a link delivered on a late retry can arrive already dead; reset gets that for free, since its request endpoint *is* the re-request.

**A Staff email change is in scope and goes the same route**, via `UpdateUser(Email(...))`, with the BFF notifying the old address through the outbox. Identity Platform automatically mails the old address a one-click `mode=recoverEmail` undo when a primary email changes; **whether an Admin SDK write suppresses that notice is unverified** and is a checkbox on the implementation ticket, because if it fires anyway the product sends two notices from two identities for one event.

**Delivery rides the outbox unchanged, with no exception carved into ADR-0010**, in ADR-0011's Platform voice (`From: notifications@{domain}`, `Reply-To: support@{domain}`). A person-specific link does not trouble ADR-0011's content rule, which governs naming the Practice; this mail is about the account and never about a Practice.

## MFA recovery is a code plus a password — never a mailbox, and never an Owner acting alone

Decided on [#605](https://github.com/markgoho/doula-cloud/issues/605). Enforced MFA with no way back locks a Practice out of its own client records, so this is launch-blocking with the MFA it recovers.

**The standard is the one this product chose voluntarily.** HIPAA is silent — nothing in current 45 CFR §164.312 mentions multi-factor authentication or account recovery, and the January 2025 Security Rule NPRM that would mandate MFA still says nothing about recovery, lockout or reset. So the bar is NIST **SP 800-63B-4** (published July 2025; the widely-quoted Rev 3 was withdrawn on 1 August 2025, so Rev 3 language is not the bar). §4.2.2.2 allows exactly three ways to recover an AAL2 account: two recovery codes obtained by different methods; one recovery code **plus** an authenticator already bound to the account — her password; or repeated identity proofing. Every path below is one of those three.

**A backup email address is rejected**, and it is the answer a reader asks after, so the reason is stated precisely. §3.1.3.1 is a hard prohibition: *"Email SHALL NOT be used for out-of-band authentication."* The same section draws the distinction the question was probing — codes *sent to* validate an address or *issued as* recovery codes are not authentication processes and are not covered — so **email is a legitimate delivery channel for a recovery code (capped at 24 hours by §4.2.1.2) and never the proof itself.** A link that alone restores access supplies one credential where two are demanded. OWASP ASVS 5.0 says it twice (6.3.6, 6.4.4), and no surveyed vendor — GitHub, Slack, Okta, Microsoft Entra, Google Workspace — accepts a bare backup-email link for lost-TOTP recovery. The product reason stands beside the standards one: a nominated backup address is systematically the *worse-defended* one, and it would become a permanent standing credential over a HIPAA-covered record that Doula Cloud can neither see, assess, nor revoke.

Three paths, chosen by whether a human sits above her:

**An Owner vouches; she does not clear.** An Owner clearing a subordinate's enrolment was adopted mid-ticket and then reshaped, because cleared, the locked-out person holds exactly one thing — her password — which §4.2 does not accept. §4.2.1.3's **recovery contact** supplies the fix without changing who is trusted: a single-use 24-hour code lands in the **Owner's own** mailbox, which is what makes it a second channel, and she hands it to the person who has just phoned her. She initiates recovery and at no point gains access to the account (ASVS 6.4.6). **Owner-only, not Admin**, matching who throws the switch, and she re-authenticates before vouching.

**Saved recovery codes go to a Practice's sole Owner and to nobody else.** There is nobody to vouch for her, and MFA is mandatory for Owners by default. Codes are at least **64 bits** from an approved RNG, hashed at rest, single-use, and reissued whenever one is spent (§4.2.1.1) — minted on the Membership event that makes her the only Owner, not only at signup, because a Practice going from two Owners to one has just created someone who needs them. Familiarity cut both ways and both were weighed: backup codes are the first-line self-service path at GitHub, Slack, Okta and Google Workspace, so Jakob's Law argues *for* them on evidence; but every unused card handed to someone who has her Owner's phone number is a credential sitting on the shared birth-centre laptop for nothing. Issuing them only where there is no alternative pays the familiarity cost once, for the one person it buys something real for.

**The last resort is a Doula Cloud operator, with no product surface.** An operator clears the enrolment after matching a live video call and a government ID against the identity-verified representative on that Practice's Stripe Connect account — the strongest proof the product holds ([ADR-0007](0007-connect-account-state-is-two-capabilities-and-a-requirements-list.md)), and the only thing in the system anyone has verified. It maps onto §4.2.1.4, repeated identity proofing, and takes the shape `credit_ledger.granted_by` already established. **No screen and no self-service endpoint**, deliberately: a support path with a UI is an attack surface that runs itself. **No mandatory hold** either — the Stripe match is documentary rather than conversational, so a delay adds little against a determined attacker while costing a real doula a day of lockout during a birth; notice goes out at the moment of the reset instead. **This is the weakest point in the whole scheme and is recorded as such.** An attacker attacks this, not the TOTP.

**The provider fixes the sequence.** Identity Platform challenges on every sign-in once a factor exists, so with the phone gone the sign-in cannot complete at all. A recovery code is therefore spent at an **unauthenticated** endpoint, and spending it **clears the enrolment and mints nothing** — the same rule as a reset link, and for a sharper version of the same reason. She then signs in with her password, and the Practice gate refuses her until she has enrolled the new phone. Clearing an enrolment ends every live session for that identity and notifies the affected person at her account address, whoever ran it.

**The cross-Practice case needs no rule, which is worth stating because it looks like it should.** A contractor at two Practices, one requiring MFA and one not, can be vouched for by the Practice that does not require it. The other loses nothing: the gate is at *Practice entry* and reads the token claim, so it refuses her at its own boundary until she re-enrols. **A Practice's posture is held by its own gate, not by who cleared the factor.** So an Owner may vouch for anyone holding a Membership at her Practice, full stop. For the same reason, **turning a Practice's switch back off is not a recovery lever**: the provider challenges before our gate ever runs.

**The record does not go in ADR-0022's `activity` table**, and that is not an oversight: `activity.practice_id` is `NOT NULL`, and a second factor is enrolled per **person**, not per Membership — the same concession ADR-0022 already makes for `staff.work_state`. The Owner-vouched path could be Practice-scoped; the sole-Owner and operator paths cannot be, and one event needs one home. So a **person-scoped auth-events table**, sibling to `staff_work_state_events`: append-only, `GRANT SELECT, INSERT` and no `UPDATE` or `DELETE`, recording the subject, the actor, the actor's kind, the reason and the time — including the case where the actor is a named human at Doula Cloud rather than any of ADR-0022's three kinds.

Identity Platform supplies none of this, by Google's own admission: *"Identity Platform does not provide a built-in mechanism for recovering second factors. If a user loses access to their second factor, they will be locked out of their account."*

## Two populations, one browser, and nothing that links them

Decided on [#168](https://github.com/markgoho/doula-cloud/issues/168).

**Nothing links, because nothing can.** There is exactly one route into an Identity Platform account (Staff, email and password) and exactly one route into a Portal Account (a BFF-minted magic link), so no address can arrive twice and the collision has no path. One-account-per-email is nonetheless in force rather than merely moot: the live `signIn` block carries no `allowDuplicateEmails` field at all, and an absent boolean in proto3 JSON is `false` — the safe default, by the provider's own default rather than by a toggle anyone remembered to throw. **The unverified-email takeover path therefore has nowhere to run**, and email verification survives on this map only as MFA enrolment's prerequisite. **Linking reopens if and only if federation does.**

**The same address twice within Staff was already built and already right.** `resolveStaff` keys on `identity_uid`, reuses one `staff` row across every Practice a person joins, and takes `staff.email` from the **verified token**, never from anything the caller typed. One human, one staff row, many Memberships; `staff.email` is deliberately not unique, because it is a contact address and not a key.

**One human may hold both a Staff identity and a Portal Account, as two records that nothing joins.** After Clients left Identity Platform the two live in **different identifier namespaces**, so there is no shared key to collide on even by accident, and ADR-0015 already says a Portal Account is legible only from a portal session. Linking them would be a leak, not a feature: the only thing it buys is a convenience nobody asked for, and what it costs is that a doula's own pregnancy record becomes reachable from her staff identity. Migration `00006`'s RLS guards — written when a shared `identity_uid` was possible, and naming this exact human — are demoted to belt-and-braces and **stay**, because the backstop is not where a saving is spent.

**She may be a Client of the Practice she works at, with no special handling.** Her record is a Client record and ADR-0006's reach rules apply unchanged, so her Owner and her Admin read it and so does she from her staff session. Forbidding it is the product refusing something that actually happens; flagging her record and cutting her own reach into it would put a special case in the reach model — the single place a special case is most expensive to keep honest — to hide a record from the very colleagues giving her the care.

**The portal invitation flow does nothing on an address that matches a Staff address**, and should not. It keys by `client_id`, `staff.email` is a non-unique contact address, and ADR-0015 separates a Client's contact address from her login outright. A check would also be an **account-enumeration oracle** — a Staff member could learn whether an arbitrary address holds a Doula Cloud account by typing it into an invite form — which is worse than the problem it solves.

**The MFA asymmetry is deliberate**: a Staff session reaches many people's health records and a portal session reaches only her own, and the magic link is itself a possession factor delivered to a mailbox she controls.

**A browser holds one tier at a time, and the eviction is announced.** The constraint is physical: one `__session` cookie at `Path: "/"`, the rewrite strips every other cookie on the way to Cloud Run, and the `sessions` table matches by `identity_uid` with no tier column. **Signing into the other population ends the live session, and she is told before it happens** — on the Continue button the magic link already requires, since it is already a POST she must press, and symmetrically on Staff sign-in with a live portal session. Nothing about the other tier is disclosed beyond what she proved she holds by presenting the cookie. Silent last-write-wins, which is what the code does today by accident, is rejected outright: swapping a person's identity without telling her, on a shared laptop, is precisely the failure that rejected one-click federated sign-in.

The rejected alternative was real and was weighed: `__session` could carry **two** tokens, chosen by request path, each `sessions` row keeping its own lifetime, honouring the 30-day portal session and the 12-hour Staff session at once. It was rejected because it keeps a **second live credential on a machine that is shared by design**, to serve a person who is her own Client somewhere else. That is rare, and signing in again is cheap.

One implementation note, so it is not rediscovered: `api/internal/sessionnotice` already sends "session revoked" and is the right sender for an evicted **Staff** session, but it resolves its recipient from `identity_uid` via the `staff` table, so it cannot notify an evicted **portal** session as written.

## Passkeys are deferred, on a dependency rather than a doubt

Decided on [#171](https://github.com/markgoho/doula-cloud/issues/171); full research in `docs/research/passkeys-for-staff.md`.

Identity Platform has **no WebAuthn support**, so passkeys would be a BFF-owned build — which ADR-0004 makes cheap, since a successful assertion calls the same `authn.MintSession` that mints `__session` today. The shared birth-centre laptop does **not** break them: cross-device QR and Bluetooth transport is the flow FIDO's own UX guidance recommends for exactly that case, and the private key never lands on the shared machine, subject to confirming Bluetooth on real pilot hardware.

Two constraints shape any future build: the cookie strip means the ceremony challenge lives in Postgres rather than a second cookie, and **the relying-party ID binds permanently to the launch domain** once the first credential is enrolled.

**The deferral now stands on that second constraint alone.** It originally rested on two, the other being that recovery bottomed out in email — and recovery no longer does, since email carries a code and is never the proof. The launch domain is not an auth-methods decision and may settle outside this map.

**The bar a passkey must clear has moved**, and it is stated here so a future adoption cannot quietly supersede this ADR. The position for accounts MFA is *required* on is **"multi-factor per NIST SP 800-63B-4", not "TOTP specifically"** — TOTP is the implementation chosen because it is GA, on this tier, and cheaper than SMS. A syncable passkey standing alone is classed as single-factor, so for an Owner it would be a **downgrade** and would have to reopen this decision explicitly. For an Admin or Doula at a Practice that has not thrown the switch, a passkey as the sole factor replacing a password is a straight improvement, and needs no reopening at all.

## What this amends in ADR-0004

[ADR-0004](0004-bff-owned-sessions.md) reduced Identity Platform to a single `VerifyIDToken` call. **That is widened here, deliberately**, and the widened surface is exactly:

- `VerifyIDToken` — unchanged, and now also read for `firebase.sign_in_second_factor`.
- `GetUser` — to read `MultiFactor.EnrolledFactors` when a recovery path drops one factor while keeping others.
- `UpdateUser` — `EmailVerified`, `Password`, `Email`, and `MFASettings`.

Nothing else. **Session ownership is untouched**: the session is still a Postgres row, Identity Platform still mints nothing, and every path here still ends at `authn.MintSession`. No new dependency and no new infrastructure — `authn.go` already builds an ADC-authenticated Admin SDK client, and the pinned `firebase.google.com/go/v4 v4.21.0` carries all of the above (TOTP admin support landed in v4.13.0). Two mechanism facts worth not rediscovering: `MFASettings` replaces the whole factor list, there being no per-factor delete; and `accounts.mfaEnrollment:withdraw` is **not** usable, because it requires the end user's own ID token and cannot be driven by the service account. `VerifyIDToken` needs no IAM at all; `UpdateUser` needs user-management permission on the Cloud Run service account, which is a **new grant**.

## Implementation

| Work | Ticket |
| --- | --- |
| Portal Account table, prefixed identifier, invite-token expiry | [#616](https://github.com/markgoho/doula-cloud/issues/616) |
| Magic-link request and redeem, the invitation as the first link, portal password removal | [#617](https://github.com/markgoho/doula-cloud/issues/617) |
| 30-day rolling Client session and sign-out-everywhere | [#618](https://github.com/markgoho/doula-cloud/issues/618) |
| A Client changes her own sign-in address | [#619](https://github.com/markgoho/doula-cloud/issues/619) |
| Session eviction across tiers, announced | [#610](https://github.com/markgoho/doula-cloud/issues/610) |
| Staff auth mail on one token table | [#613](https://github.com/markgoho/doula-cloud/issues/613) |
| TOTP, the Owner requirement and the per-Practice switch | [#606](https://github.com/markgoho/doula-cloud/issues/606) |
| MFA recovery: vouching, saved codes, the operator path | [#615](https://github.com/markgoho/doula-cloud/issues/615) |
| Rate limiting, which gates every public link-request endpoint | [#602](https://github.com/markgoho/doula-cloud/issues/602) — landed |
| `staffauth.signup` writes `staff.email` from the request body, not the verified token | [#614](https://github.com/markgoho/doula-cloud/issues/614) |

## What would reopen this

- **Federation**, if the pilot agency turns out to run Google Workspace and to want it ([#243](https://github.com/markgoho/doula-cloud/issues/243)). Account linking reopens with it, and only with it.
- **Apple**, only if an App Store build and some other federated provider both happen.
- **Passkeys**, once the launch domain is settled hard enough to fix the relying-party ID forever — and, for an Owner, only against the multi-factor bar above.
- **The provider**, on nothing recorded here. The tier is BAA-covered and the BAA is executed, so the compliance question does not reopen; replacing Identity Platform would be its own effort.
