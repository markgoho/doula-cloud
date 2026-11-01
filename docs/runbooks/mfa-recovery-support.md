# Runbook: MFA recovery, DoulaCloud support action

[#615](https://github.com/markgoho/doula-cloud/issues/615)'s third recovery path: a DoulaCloud operator clears a person's TOTP enrollment in ops, after every other path has failed her. [#605](https://github.com/markgoho/doula-cloud/issues/605)'s resolution comment records why this exists and why no Practice can reach it — read that first if you have not. It has no screen in the product a Practice uses; its one screen is in ops ([#2483](https://github.com/markgoho/doula-cloud/issues/2483)).

**When this applies.** A Practice's sole Owner has lost both her phone (the TOTP enrollment) and her saved recovery codes ([#615](https://github.com/markgoho/doula-cloud/issues/615)'s second path). She has no Owner above her to vouch for her (the first path). Nothing else in the product can get her back in.

## Required proof

**A live video call, plus a government-issued photo ID, matched against the identity-verified representative on that Practice's Stripe Connect account.**

That match is the whole authorization. There is no lesser standard, and no substitute:

- Do not act on an email request alone, however convincing, however many details it gets right about the Practice. Email is not identity proof (this is the same reasoning #605 rejected a backup-email recovery address on).
- Do not act on a phone call alone. You need to *see* a face and a government ID together, live, not described.
- The person on the call must be the same person Stripe verified as that Practice's account representative when its Connect account was set up (ADR-0007). If the Practice's Stripe Connect account was never completed, or its verified representative is someone else, **stop and escalate** — do not clear the enrollment.
- If you cannot see the Stripe Connect account's verified representative from your own tooling, ask engineering rather than guessing from the Practice's own claims about who owns it.

**No mandatory hold.** #605 weighed a waiting period and rejected it: the Stripe match is documentary, not conversational, so a delay buys little against a determined attacker while costing a real doula a day of lockout during a birth. Clear the enrollment as soon as the match is confirmed. The affected person is notified automatically, at the moment of the reset, regardless.

**This is the weakest point in the whole recovery scheme, and that is recorded on purpose.** An attacker attacks this step, not the TOTP itself. Do not make it easier than the standard above already is.

## Running it

Since [#2483](https://github.com/markgoho/doula-cloud/issues/2483) the reset is taken only in ops ([ADR-0062](../adr/0062-the-platform-operator-works-in-a-separate-app-at-ops-doula-cloud-with-its-own-service-and-database-role.md)). `doula-api` no longer serves it, and there is no token to mint and no name to type: who ran it comes from your operator sign-in.

1. Sign in at `https://ops.doula.cloud`.
2. Choose **Reset two-factor authentication** on the ops home page, and enter the email address the person tells you she signs in with. The lookup lists each live Staff member with that sign-in email, her email and her Practices. Showing that contact is recorded as a `staff_lookup` row in the Operator log, with the email you asked for. If no one is found, check the spelling with her; a deleted Staff member is not found.
3. Choose her name. The check page names the Staff member and every Practice she belongs to, and restates the two conditions above. Check the name against the person on the call before you go on.
4. Enter your password and a new authenticator-app code, then press **Reset two-factor authentication**. The code admits this one act.

The result page says the reset is done: every authenticator on her sign-in is removed, every session for that identity is ended, and she has been mailed a notice. A refusal says why in plain words. "There is no Staff member at this address" means the Staff member was deleted after the lookup — look her up again. If Identity Platform refuses, nothing has changed; try again. Pressing the button twice on one page is the same reset, not a second one.

**What happens next, for the person.** She receives an email that her two-factor authenticator was reset and every session was signed out. She signs in with her password — Identity Platform no longer challenges for a second factor once none is enrolled — and [#606](https://github.com/markgoho/doula-cloud/issues/606)'s Practice gate refuses her at any Practice boundary that requires MFA until she enrolls a new authenticator. No extra step is needed from you; this is the same gate every other recovery path relies on.

## Recording it

The reset writes three records, and they are the permanent record: an `operator_acts` row (`second_factor_cleared`, with the time of your fresh code), the `staff_auth_events` row (`reason = 'support'`) keyed to that act, and a row in each of her Practices' Activity that shows the reset under your name with DoulaCloud. There is nothing further to file — do not also open a support ticket restating what the rows already say, and do not note this in any Practice-facing channel. If the Stripe Connect match was unusually difficult to make (an expired ID, a name mismatch you resolved some other way, anything that would make a future reviewer ask "how did we know it was really her?"), say so in your own team's incident notes — the records say *that* the match was made, not the details of how.
