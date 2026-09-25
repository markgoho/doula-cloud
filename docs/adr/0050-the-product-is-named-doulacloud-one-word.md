# The product is named DoulaCloud, one word

The product's name is **`DoulaCloud`**: one word, capital D and capital C. This form goes everywhere a person reads the name: the app's chrome, the audit trail's system actor, the Notification `From` ([ADR-0011](0011-notification-sending-identity-is-one-shared-domain.md)), the marketing site, the DONA booth, and the Certificate of Assumed Name that [#403](https://github.com/markgoho/doula-cloud/issues/403) files for Briandor LLC. Decided on [#338](https://github.com/markgoho/doula-cloud/issues/338) on 2026-09-25. Until then the repo wrote "Doula Cloud", two words.

**Why.** The founder's reason, in his words: the single word feels more like a brand or a trademark than two common words put together. The evidence points the same way. Every surface that cannot hold a space already uses one word: the handles (`@doulacloud`), `doulacloud.com`, and the Stripe shortened descriptor. That descriptor is limited to 10 characters, and `DoulaCloud` has exactly 10. The teaser social cards already say `DoulaCloud`. The name clearance research ([#996](https://github.com/markgoho/doula-cloud/issues/996)) found both forms clear.

**What this is not.** It is not a claim that one word is stronger in trademark law. The USPTO usually treats two joined descriptive words the same as the two words apart. The choice is about how the name reads.

**Considered option.** "Doula Cloud", two words. It needed no rename and reads as plain English. But each surface above would carry a second form of the name, and the teaser cards would have to be made again.

**Consequences.**

- Identifiers and slugs stay as they are, because they are not the name: the repo `doula-cloud`, the GCP project, `doula.cloud`, and `mg.doula.cloud`.
- The rename of the existing two-word form, and a build gate that stops it from coming back, are [#1463](https://github.com/markgoho/doula-cloud/issues/1463).
- Changing the name again costs a second DBA filing, a Stripe descriptor change, and a change to email that has already been sent.
