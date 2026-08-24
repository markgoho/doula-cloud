# Mailgun is the Notification vendor for v1, with no BAA in force

Map [#213](https://github.com/markgoho/doula-cloud/issues/213) charted three questions before any Notification could send: what to call it ([ADR-0009](0009-notification-is-one-term-two-voices-keyed-by-recipient.md)), how to send it reliably ([ADR-0010](0010-notification-email-delivery-is-an-outbox-not-in-request.md)), and what identity it sends from ([ADR-0011](0011-notification-sending-identity-is-one-shared-domain.md)). This ADR records the fourth: the vendor, and Doula Cloud's compliance position sending through it — closing out research done across [#214](https://github.com/markgoho/doula-cloud/issues/214) and [#221](https://github.com/markgoho/doula-cloud/issues/221).

**Mailgun, no bake-off.** The map's own Notes fixed Mailgun as vendor at charting — already in use, a bake-off scoped out unless compliance research forced one. It didn't.

**#214's research found a real BAA trigger, then #221 removed it.** #214 established that a Practice-voice Notification's address-plus-named-Practice pairing is protected health information once any tenant Practice becomes a covered entity (the existing [#30](https://github.com/markgoho/doula-cloud/issues/30) trigger) — the no-content rule strips clinical detail but not the fact of the pairing, and Mailgun's log/message retention (a sold plan feature, not incidental to transmission) forecloses the conduit exception. Under that finding, Mailgun would sign a BAA — it publishes a HIPAA addendum — but only after a prior-written-consent step its Terms of Service §4.1 requires, on a plan tier that could not be confirmed first-party.

**#221 closed the question a different way: remove the trigger, not clear it.** ADR-0009 and ADR-0011 record that no Notification, of either voice, names a Practice in `From`, subject, or body for v1. Per #214's own test, the exposure was the address-plus-named-Practice pairing; remove the pairing and the relates-to-health-care limb of the PHI definition is never met, for either voice. **No BAA is needed for v1 traffic.** Mailgun's self-serve Foundation/Scale plans suffice: no Enterprise tier, no ToS §4.1 consent gate, no sales process to clear before onboarding a tenant.

**Residual items, documented rather than acted on, because nothing forces a decision on them for v1:**

- Mailgun's transport is *opportunistic* TLS — it downgrades to plaintext if the receiving server doesn't offer TLS, confirmed first-party in Mailgun's own security guide. This is a general email-transport property, not specific to the BAA question, and holds regardless of what the map decided about naming.
- Sinch (Mailgun's parent)'s HIPAA validation is scoped to voice/fax/UCaaS, not email — it cannot be cited if this question ever reopens.
- #214 recommended legal review before #216 was decided; #221's naming reversal made that review unnecessary for v1's actual traffic, since the trigger it would have reviewed no longer fires.

**This is conditional on the naming rule holding.** ADR-0011 left per-Practice custom domains deferred, not ruled out, for exactly this reason. If a future Practice bake-off or product decision reopens naming the Practice in a Notification, #214's full BAA analysis — the PHI test, the conduit-exception failure, the ToS §4.1 consent step, plan tier and cost still unconfirmed first-party — reapplies unchanged, and legal review becomes warranted again before that reopening ships.

Full research: [`research/mailgun-baa-posture`](https://github.com/markgoho/doula-cloud/blob/research/mailgun-baa-posture/docs/research/mailgun-baa-posture.md).
