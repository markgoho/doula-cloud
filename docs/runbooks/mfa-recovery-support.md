# Runbook: MFA recovery, Doula Cloud support action

[#615](https://github.com/markgoho/doula-cloud/issues/615)'s third recovery path: a Doula
Cloud operator clears a person's TOTP enrolment by hand, after every other path has failed
her. [#605](https://github.com/markgoho/doula-cloud/issues/605)'s resolution comment records
why this exists and why it has no screen — read that first if you have not.

**When this applies.** A Practice's sole Owner has lost both her phone (the TOTP enrolment)
and her saved recovery codes ([#615](https://github.com/markgoho/doula-cloud/issues/615)'s
second path). She has no Owner above her to vouch for her (the first path). Nothing else in
the product can get her back in.

## Required proof

**A live video call, plus a government-issued photo ID, matched against the
identity-verified representative on that Practice's Stripe Connect account.**

That match is the whole authorization. There is no lesser standard, and no substitute:

- Do not act on an email request alone, however convincing, however many details it gets
  right about the Practice. Email is not identity proof (this is the same reasoning #605
  rejected a backup-email recovery address on).
- Do not act on a phone call alone. You need to *see* a face and a government ID together,
  live, not described.
- The person on the call must be the same person Stripe verified as that Practice's account
  representative when its Connect account was set up (ADR-0007). If the Practice's Stripe
  Connect account was never completed, or its verified representative is someone else, **stop
  and escalate** — do not clear the enrolment.
- If you cannot see the Stripe Connect account's verified representative from your own
  tooling, ask engineering rather than guessing from the Practice's own claims about who
  owns it.

**No mandatory hold.** #605 weighed a waiting period and rejected it: the Stripe match is
documentary, not conversational, so a delay buys little against a determined attacker while
costing a real doula a day of lockout during a birth. Clear the enrolment as soon as the
match is confirmed. The affected person is notified automatically, at the moment of the
reset, regardless.

**This is the weakest point in the whole recovery scheme, and that is recorded on purpose.**
An attacker attacks this step, not the TOTP itself. Do not make it easier than the standard
above already is.

## Running it

There is no screen for this — deliberately, per #615's AC. It is one authenticated HTTP call,
made by an operator who can mint a token for `internal-caller@doula-cloud.iam.gserviceaccount.com`
(ADR-0037: the boundary is a caller identity, not a shared secret, and the token expires within
the hour rather than being a string somebody keeps):

```bash
TOKEN=$(gcloud auth print-identity-token \
  --impersonate-service-account=internal-caller@doula-cloud.iam.gserviceaccount.com \
  --audiences=https://doula-api-850855848778.us-central1.run.app \
  --include-email)

curl -s -o /dev/null -w '%{http_code}\n' \
  -X POST 'https://doula-api-850855848778.us-central1.run.app/api/internal/staffauth/mfa-recovery/support-clear' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"staffId": "<the staff row id>", "operator": "<your name>"}'
```

`--audiences` is the service's **base** URL, not this endpoint's own URL, and `--include-email`
is required: the guard checks the token's `email` claim against the allowlist, and a token
minted without that flag carries none. Getting either wrong produces a `401` from a token that
is otherwise perfectly valid.

- `staffId` is the `staff.id` UUID of the person whose enrolment you are clearing — find it
  from the Practice's roster in a support tool, or by asking engineering to look it up by
  email if you only have that. It is **not** a Practice id.
- `operator` is your own name, plain text. It becomes `staff_auth_events.actor_operator` —
  the permanent record of who ran this and cannot be blank. Use a name a colleague reading
  the audit trail later would recognize, not an initials or a ticket number.
- A `204` means it worked: the enrolment is cleared, every session for that identity is
  ended, and the person has been mailed a notice. A `404` means the `staffId` does not name
  a real Staff row — double-check it before retrying. A `401` means the token is wrong — most
  often an expired one, a missing `--include-email`, or an `--audiences` carrying this
  endpoint's path rather than the base URL. Mint a fresh one rather than reusing a stale copy.

**What happens next, for the person.** She receives an email that her two-factor
authenticator was reset and every session was signed out. She signs in with her password —
Identity Platform no longer challenges for a second factor once none is enrolled — and
[#606](https://github.com/markgoho/doula-cloud/issues/606)'s Practice gate refuses her at any
Practice boundary that requires MFA until she enrolls a new authenticator. No extra step is
needed from you; this is the same gate every other recovery path relies on.

## Recording it

The `staff_auth_events` row this call writes (`reason = 'support'`) is the permanent record.
There is nothing further to file — do not also open a support ticket restating what the row
already says, and do not note this in any Practice-facing channel. If the Stripe Connect
match was unusually difficult to make (an expired ID, a name mismatch you resolved some other
way, anything that would make a future reviewer ask "how did we know it was really her?"),
say so in your own team's incident notes — the audit row records *that* the match was made,
not the details of how.
