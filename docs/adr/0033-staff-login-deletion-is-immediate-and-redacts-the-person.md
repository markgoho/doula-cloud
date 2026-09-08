# Staff login deletion is immediate, and redacts the person

A Staff person — a doula, an Admin, an Owner — had no way to leave Doula Cloud. [ADR-0027](0027-erasure-redacts-in-place-and-shreds-the-key.md) answered this shape for a Client asking her Practice, and [ADR-0031](0031-practice-deletion-is-a-thirty-day-window-then-the-erasure-cascade.md) answered it for a Practice asking Doula Cloud. This is the third direction and the one neither of them reaches: a *person* asking Doula Cloud, whose reach runs across every Practice she works at rather than sitting inside one. An Owner's tool for ending someone's access is Membership removal, and it is correctly scoped to the one Practice that Owner owns; no Owner has, or should have, reach into the other Practices a contractor doula works at.

Decided on [#892](https://github.com/markgoho/doula-cloud/issues/892), split from [#871](https://github.com/markgoho/doula-cloud/issues/871) decision 6. GDPR and every non-US jurisdiction are out of scope, the same carve-out ADR-0027 and ADR-0031 both make.

## Vocabulary: Erasure, Deletion, and now a login

ADR-0031 split the two words and this ADR holds the split. **Erasure** is what happens to a Client. **Deletion** is what happens to a Practice, and now to a Staff login. A Staff person is a natural person like a Client, so she gets ADR-0027's *mechanism*; she is not a Client, holds no Portal Account and is nobody's billing party, so she does not get ADR-0027's *word*.

## Deletion redacts the `staff` row; it never deletes it

ADR-0027's redact-in-place template applies here for its own reason and for a second one this ADR adds.

The first reason is ADR-0027's: she is a natural person, and the point of the act is that Doula Cloud stops holding who she is.

The second is that a hard `DELETE FROM staff` is not merely unwise, it is impossible. Sixteen migrations carry a foreign key to `staff (id)`. `activity.actor_staff_id` is one of them, and `activity` holds `GRANT SELECT, INSERT` and no `DELETE` — by design, ADR-0022. A delete would have to unwrite an append-only log to succeed, and it cannot.

So the row keeps its id forever. `name` becomes `Deleted Staff Member` and `email` becomes `deleted@deleted.invalid` — both columns are `NOT NULL`, so neither can be nulled, and the replacements say plainly what happened rather than inventing a person, exactly as `client.ErasedGivenName` does. `identity_uid` is `NOT NULL UNIQUE`, so it takes `'deleted:' || id`, a sentinel unique for free because the id is: a shared constant would make the second person to delete her login collide with the first. `deleted_at` is stamped, and is to a `staff` row exactly what `clients.erased_at` is to a Client's — proof the act ran, and the gate that stops it running twice.

## No restore window, where Practice deletion has thirty days

ADR-0031 gave Practice deletion a restore window and stated its reason plainly: that act can cut off *other people's* access, where Client erasure only ever costs the person who asked. Deleting a Staff login ends her own access and nobody else's — the last-Owner refusal below is what guarantees no Practice is left without an Owner. So the principle points at immediacy, and this act matches Client erasure rather than Practice deletion.

**This decision overrides a business answer the maintainer gave on #871**, where a window was chosen for a Practice. It is recorded here in as many words so that reversing it later is a one-paragraph change and not an archaeology exercise: if a Staff login should get a window, the change is to ADR-0031's `deletion_requested_at`/`deletion_finalize_at` shape applied to `staff`, plus a lockout in `authn.Begin`. Nothing else in this ADR depends on immediacy.

## Every Membership ends, and the last-Owner rule refuses

Deleting the login ends every Membership she holds, at every Practice, in the same transaction. An Owner does not have to remove her from each Practice first — that would make leaving Doula Cloud a negotiation with every Practice she ever worked at.

Each ended Membership records the same `removed` membership event `RemoveMembershipHandler` records, naming the roles and employment type it held, with her as her own actor. Losing her can leave a remaining co-Owner newly sole, so `reconcileOwnersAtPractice` runs per Practice afterwards, exactly as it does on the Owner-run path — [#615](https://github.com/markgoho/doula-cloud/issues/615)'s saved-recovery-code rule is inherited, not restated.

The act inherits `removesLastOwner`'s refusal and nothing else. It is refused, 409, with the Practices named, while she is the sole Owner of any Practice that has not been finalized — **including a Practice already pending deletion**, because letting her go during ADR-0031's restore window would foreclose the restore that window exists to promise. A Practice whose `deleted_at` is already stamped does *not* refuse: nobody can reach it any more, so being its Owner no longer means anything and must not trap her. She hands ownership over, or deletes the Practice first through ADR-0031's own path.

## Her authored work keeps resolving, unredacted

ADR-0027's rule for a Client applies to a Staff person unchanged: redact the identity, keep the record. Every `actor_staff_id`, `staff_id`, `offered_by`, `decided_by`, `attached_by`, `ended_by`, `requested_by`, `invited_by`, `revoked_by` and `cleared_by` still resolves — to the redacted row. Messages she sent, Contracts she is named on, Engagements she was assigned, Visits she worked, Offers she made and Attachments she ended all keep their content.

Free text that happens to name her by hand is not scrubbed. That is ADR-0027's existing stated limitation, inherited here rather than re-argued.

## No crypto-shredding, and no unsettled-invoice refusal

Both were considered, and both were answered no rather than left for a reader to wonder about.

**No `staff_data_keys`.** ADR-0027 seals a Client's `activity` diffs under a per-Client key and destroys the key on erasure, because those diffs carry her identifying data. Nothing equivalent exists for a Staff person: the `membership` subject kind records roles and employment type, `staff_auth_events` a reason and an actor, `staff_work_state_events` a work state. There is nothing sealed and nothing to shred, so this act builds no analog.

**No refusal over an unsettled Invoice.** ADR-0027 refuses a Client's erasure while any of her invoices is `draft` or `open`, because deleting her Stripe Customer underneath an unpaid invoice would leave the Practice unable to collect. Nothing in `payments` or `invoices` makes a Staff person a billing party — an Invoice runs from a Practice to a Client — so the condition has nothing to attach to. #892's own hedge ("an unsettled Invoice she's tied to, *if that turns out to matter*") resolves to no.

## The act is recorded in `staff_auth_events`, not in `activity`

`activity.practice_id` is `NOT NULL` and its INSERT policy needs `app.current_practice_id`. This act is not Practice-scoped, so recording it there would mean one row per Practice she was a member of — and a person whose Memberships were all removed by Owners belongs to no Practice at all, which would leave the act unrecorded for exactly the person with the least else on file.

`staff_auth_events` (migration 00062) is deliberately un-scoped, already carries her `staff_id`, and is append-only with `GRANT SELECT, INSERT`. It takes a new reason, `login_deleted`, on the actor branch that demands a Staff actor: she runs the act on herself, so `actor_staff_id` equals `staff_id`, the same shape `self_service` already has. This is 00062's own reasoning applied to a fourth act, and 00043's precedent before it: where `activity` cannot hold the actor, the record lives on the table that owns the fact.

The per-Practice `removed` membership events still go to `activity`, one per Practice, because a Membership genuinely is Practice-scoped. Between the two, the question "how did this come to be?" has an answer from either end.

## What deletion destroys, redacts and keeps

**Destroyed.**

- Her Identity Platform account (`authn.AccountManager.DeleteAccount`). This existed once for Client erasure ([#394](https://github.com/markgoho/doula-cloud/issues/394)) and was retired by [#796](https://github.com/markgoho/doula-cloud/issues/796) when Clients stopped holding Identity Platform accounts; it is restored for the population that still does. An account Identity Platform already reports absent is a success, never an error, which is what lets a retry finish.
- Every `sessions` row of hers, at every browser. **The order is load-bearing:** `sessions` is keyed on `identity_uid`, not on `staff_id`, so the delete must run *before* the sentinel is written or it matches nothing and she stays signed in on a live cookie.
- Every `practice_memberships` row of hers, at every Practice.
- Her enrolled MFA factor, as a consequence of the account going, not as a separate act.

**Redacted.** `staff.name`, `staff.email`, `staff.identity_uid`, with `staff.deleted_at` stamped. Nothing else on the row is touched: `work_state` and `work_state_reported_at` are a US state abbreviation and a date, neither of which identifies anybody.

**Kept, and still resolving to the redacted row.** `activity` (every row she is the actor of, and every `membership`-subject row about her), `staff_auth_events`, `staff_work_state_events`, `messages`, `visits`, `contracts`, `engagements`, `engagement_attachments`, `offers`, `contract_void_requests`, `credit_ledger.granted_by`, `staff_invitations.invited_by`, and every other foreign key into `staff`.

**Stated limitations — her address survives in three places this act cannot reach.**

- `email_suppressions.address` holds her raw address, and the table carries no `DELETE` grant. That is [ADR-0029](0029-email-suppression-is-address-keyed-and-outbox-wide.md)'s design, outbox-wide and deliberate: a suppression that could be deleted is a suppression that can be forgotten, and mailing someone who bounced or complained is worse than holding the address that proves she did.
- `staff_token_mail_outbox` and `staff_email_change_outbox` hold it too, for the same append-only reason every outbox in this schema does.
- Mailgun's own send logs are outside this system entirely — ADR-0027's existing stated limitation, unchanged.

Retroactive shredding across the database backup retention window is out of reach, which ADR-0027 already records.

## Queued mail rechecks her live state rather than dead-lettering

Outbox rows naming her can already be queued when she deletes her login, and her Identity Platform account going would make each one fail its `GetAccount` call and dead-letter. Two mechanisms cover this, and they are not redundant with each other.

**Resolved at source.** The deletion transaction itself marks every *pending* row addressed to her uid sent, having sent nothing — `staff_token_mail_outbox`, `staff_email_change_outbox`, `session_notice_outbox` (keyed on `identity_uid`) and `staff_mfa_recovery_outbox` (on `recipient_identity_uid`). It runs before the sentinel is written, for the same reason the session delete does: every one of these tables addresses a person by her uid, so a row becomes unfindable the moment it changes. This is also the only thing that can resolve a queued *password reset*: that row's identity was resolved through Identity Platform, which holds Client Portal accounts too, so a worker that saw no `staff` row could not tell a deleted Staff person from a Client and must not guess.

**Rechecked at send.** Each affected worker also rechecks her live state immediately before acting, and marks a row for a deleted person sent having done nothing. This is ADR-0031's skip-at-send recheck, unchanged in shape and for the same stated reason: these tables carry no `DELETE` grant, so a queued row cannot be cancelled, only made a no-op at the moment it fires. It covers what resolve-at-source structurally cannot — a row queued by a request that was already in flight, which no statement in the deletion's own transaction can see.

Two facts about that recheck, recorded because neither is obvious from the code:

- **On three of the four tables, the recheck is on absence, not on `deleted_at`.** The redaction rewrites `identity_uid` in the same `UPDATE` that stamps `deleted_at`, so a `WHERE identity_uid = $1` lookup afterwards matches no row at all and never reaches the column. Absence of a live `staff` row is the signal, and it is only sound because every queue site on those tables is Staff-gated. `staff_mfa_recovery_outbox`'s *subject* is the exception — it is keyed on `subject_staff_id`, so that one reads `deleted_at` for real.
- **`staff_email_change_outbox` needed no recheck at all**, and does not have one. It resolves no Staff person at send time — the notice is composed from the `old_email` captured on the row — so it cannot hit `ErrAccountNotFound`, and the notice it carries is still true and still worth delivering. Resolve-at-source covers it anyway.

## The endpoint is self-only, by shape

`DELETE /api/staff/account` — the `/api/staff/...` family with no `{practiceId}` in it, beside `PUT /api/staff/work-state` and `DELETE /api/staff/mfa`. It carries no staff id anywhere in the route or the body, so there is no path parameter a caller could redirect at another person and no authorization branch to get wrong. The row acted on is the one the session cookie's identity resolves to. Migration 00101's `staff_self_login_deletion` policy is the same rule again at the boundary that can actually enforce it — it admits the caller's own row, in the pre-Practice window, leaving only with `deleted_at` stamped and `identity_uid` holding the sentinel it spells out in full.

`staffauth.RequireConfirmed` gates it, the same `X-Confirmed` backstop `RemoveMembershipHandler` and Practice deletion use: a hard block with a deliberate override, per `CLAUDE.md`, not a dismissible warning. The confirmation names what is destroyed — her login, every Membership, her access to every Practice — and what is kept — every record she authored, and Doula Cloud's own logs — before it commits.

## The Identity Platform call sits inside the transaction

Every other split leaves a worse half-state. Account destroyed and row intact locks her out of a login she asked to delete and can no longer finish deleting. Row redacted and account intact leaves a live credential for a person Doula Cloud says is gone. Putting the call last inside the transaction makes a failure roll everything back, which is the one outcome that is honestly recoverable: she tries again.

That a held-open Admin SDK round trip is acceptable at a once-per-person seam is `authn.BeginBootstrap`'s own recorded reasoning, and `ChangeEmailHandler`, `RemoveSecondFactorHandler` and `SpendResetHandler` already make Admin SDK calls inline. No outbox is introduced.

## Considered and rejected

- **A restore window, by symmetry with ADR-0031.** Rejected on ADR-0031's own stated reason, which is about other people's access and does not apply. Recorded above as an override of a business answer, not a derivation from one.
- **An Owner-facing "delete this person's login" control.** Rejected: Membership removal already exists, is already Owner-only, and is already correctly scoped. An Owner-triggered login deletion would reach across Practices that Owner has nothing to do with.
- **Recording the act once per Practice in `activity`.** Rejected: a person with no Memberships left would have the act recorded nowhere.
- **A `staff_data_keys` crypto-shredding analog.** Rejected: nothing about a Staff person is sealed, so there is nothing to shred.
- **An outbox for the Identity Platform delete.** Rejected: it would put the account's destruction after the commit and reintroduce the half-state the ordering above exists to prevent.
- **Data export for a Staff person before she goes.** Out of scope. [#288](https://github.com/markgoho/doula-cloud/issues/288) built export for a Practice; whether a person gets her own is a separate question nobody has asked yet.
