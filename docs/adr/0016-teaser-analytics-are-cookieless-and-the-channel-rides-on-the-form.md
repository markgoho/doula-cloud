# The teaser's analytics are cookieless, and the channel rides on the form

The pre-launch teaser ([map #359](https://github.com/markgoho/doula-cloud/issues/359)) needs one number that nothing was going to produce. Buttondown reports how many people joined the list. Nothing reported how many people *saw the page and did not join*, and without that denominator a disappointing January has two explanations — the page failed to persuade, or the distribution failed to deliver anybody — with no way to tell them apart.

Two decisions answer it, and they are separate mechanisms.

**Visitors are counted by [Pirsch](https://pirsch.io), at $6/month.** It is cookieless by construction: a visitor is identified by a one-way hash of IP address, user agent, and a salt unique to this site, and the IP is consumed by the hash and never stored. The salt rotates and sessions expire after 24 hours, so it cannot follow anybody across days or across sites. Data is held in Germany, and Pirsch offers a Data Processing Agreement. UTM parameters are a first-class dimension, so `doula.cloud/?utm_source=fb-birthworkers` reports visits per channel without any extra work — which matters because Facebook rewrites the `Referer` header and its in-app browser often sends none, so referrer alone would not have done this job.

**A subscriber's channel is carried by a hidden field on the form, not by the analytics.** Pirsch cannot identify a person, by design, and Buttondown never sees the address bar the form was on — a form sends only its own inputs, and the UTM tag lives in the URL. So the two ledgers cannot be joined after the fact. The teaser therefore carries a hidden `metadata__source` input, filled from `utm_source` by one line of first-party JavaScript before the POST. Buttondown stores the value on the subscriber record, so the January CSV answers "how many *confirmed* subscribers came from that Facebook group" rather than "how many clicks did it get".

## Why there is no consent banner

This is the part a future reader will query, because "we added analytics and did not add a banner" reads like an oversight. It is not. Two different laws are in play and only one of them is about banners.

- **ePrivacy** — the cookie law — governs storing or reading anything on a visitor's device. Pirsch stores nothing there: no cookie, no local storage. Nothing is triggered, so no consent banner is required. The "no cookies, so no banner" finding from [#363](https://github.com/markgoho/doula-cloud/issues/363) survives Pirsch intact.
- **GDPR** still applies, because hashing an IP address is processing personal data even though the IP is discarded a moment later. That needs a lawful basis and an honest disclosure, not a banner.

The basis named is **legitimate interest**, deliberately and not by default. Naming consent instead would oblige us to ask — which is the banner we are avoiding — and would make the measurement worthless anyway, since a page whose entire job is one form cannot afford a modal in front of it.

**This is a fourth question for the lawyer** named in [#363](https://github.com/markgoho/doula-cloud/issues/363), alongside the three already there: whether cookieless reach measurement stands up as legitimate interest in every jurisdiction a Facebook group post can reach. Everything is built to the conservative reading — Pirsch's DPA signed, Pirsch named as a processor on `/privacy`, the basis stated in plain English. A lawyer's answer in December can only relax that.

## Considered options

- **Measure nothing.** Free, and defensible for a four-month teaser. Rejected because it leaves the January post-mortem with two explanations and no evidence, which is the exact failure this map exists to avoid.
- **Firebase Hosting request logs, via the Cloud Logging integration.** The strongest free rival, and genuinely cheap: no script on the page, no cookie, no third party, and the log entry carries the full request URL with its query string, so UTM tags are captured server-side. Rejected on the quality of the number, not the price. Raw CDN logs count bots as well as people, and at teaser volumes bots can dominate the denominator — which corrupts the one ratio the measurement exists to produce. The default 30-day bucket retention is also shorter than the teaser itself. Worth noting that the option is *not* the free-by-default one it appears to be: the console's usage tab shows aggregate bandwidth only, and per-request logging is a switch somebody has to flip, per site.
- **Cloudflare Web Analytics.** Free, cookieless, and a cleaner number than raw logs because bots do not run JavaScript. Lost to Pirsch on data residency and on the DPA story, having no price advantage that mattered at $6/month.
- **Plausible or Fathom**, $9–15/month. Equivalent in posture, more expensive, no better fitted.
- **A self-hosted counter.** Rejected on operations: it needs a server we do not run, to save $72 a year.

## Consequences

- ~~**This is the third recurring cost on a product with no revenue**, beside the virtual mailbox (~$99/year, [#363](https://github.com/markgoho/doula-cloud/issues/363)) and the Buttondown tier ([#366](https://github.com/markgoho/doula-cloud/issues/366)). $72/year, and Pirsch has no permanent free tier — only a 30-day trial.~~ **Corrected below: the marginal cost is nil.**
- **`/privacy` changes, and its date changes with it.** The line "there's no analytics on it", fixed on [#363](https://github.com/markgoho/doula-cloud/issues/363), is now false. The replacement copy is on that ticket.
- **The hidden field is a one-way door, and it is the only one here.** A subscriber who joins before the field exists is untagged forever; there is no way to backfill where somebody came from. Pirsch itself is trivially reversible — remove a script tag — so the field, not the vendor, is what has to be right at go-live.
- **The teaser now runs JavaScript**, where [#363](https://github.com/markgoho/doula-cloud/issues/363) recorded it as having none. One line of ours, plus Pirsch's tag. Both are outside the form's own path: if either fails or is blocked, the hidden field posts empty and the signup still works.
- **Attribution degrades quietly rather than loudly.** A blocked script means an untagged subscriber, not a lost one — so the channel counts are a floor, never a total, and should be read that way.

## Amendment, 2026-08-26: four hidden fields, not one

Settled on [#368](https://github.com/markgoho/doula-cloud/issues/368). The vendor, the cookieless posture, the legitimate-interest basis and the no-banner finding are all unchanged. Only the field count moves.

The form carries **four** hidden inputs rather than one, all filled by the same line of first-party JavaScript from the matching UTM parameter:

- `metadata__utm_source` — the venue, at venue granularity: `fb-rochester-birth-workers`, not `facebook`
- `metadata__utm_medium` — `social`, `email`, `qr`
- `metadata__utm_campaign` — `teaser`
- `metadata__utm_content` — which social card was live when the link was posted

`utm_term` is skipped; there is no paid search. Buttondown stores metadata as a JSON dict with arbitrary keys, so four flat keys need nothing special, and its subscriber search works on `key:value` — which four fields serve and one concatenated field would not. Writing four is *less* code than one: a loop over the four names rather than a hand-written read.

The reason to widen it now rather than later is the one-way door this ADR already names. A subscriber who joins before a field exists is untagged forever. One field answers "which venue"; it cannot answer "which card" or "which push", and those questions cost nothing to keep open in August and cannot be reopened in December. `/privacy` gains one word with it — "one hidden field" becomes "a few hidden fields", on [#363](https://github.com/markgoho/doula-cloud/issues/363).

**`utm_content` is a label, never a verdict.** The teaser ships as a single page with one card at a time, swapped over the four months rather than run in parallel — [#368](https://github.com/markgoho/doula-cloud/issues/368) rules out a creative test, because two cards in two different venues confound card with audience, and sequencing them by month confounds card with time instead. The field records which card was live when a link was posted. It does not measure which card is better, and nothing downstream should read it that way.

That accuracy is also unenforced: the tag is only true if whoever posts sets it to the card currently live. A swapped card with an unswapped tag lies silently, which is worse than an absent tag. The constraint belongs to distribution, which is its own map.

## Amendment, 2026-08-26: the teaser's Pirsch cost is nil, not $72/year

The consequence above assumed a Pirsch account bought for the teaser. There already is one, and it carries roughly eight sites — the teaser is a marginal tenant on a subscription that would exist whether or not this decision had gone the other way.

So the honest number for **this** map is **$0/year**, and the recurring-cost list shortens to two: the virtual mailbox (~$99/year, [#363](https://github.com/markgoho/doula-cloud/issues/363)) and the Buttondown tier ([#366](https://github.com/markgoho/doula-cloud/issues/366)). The tier movement on that account is driven by the other sites, not by anything the teaser does, and should not be charged here or read as evidence that a teaser draws real traffic.

This strengthens the decision rather than disturbing it. Firebase Hosting request logs were rejected on the quality of the number — raw CDN logs count bots, and bot noise corrupts the one ratio the measurement exists to produce — with price a secondary consideration. That secondary consideration has now gone to zero, so the option that lost on quality also no longer wins on cost.

**One thing it complicates, and it belongs to [the business map](https://github.com/markgoho/doula-cloud/issues/375) rather than here.** A DPA covers an *account*, not a site. This account is personal and serves seven unrelated sites, so signing Pirsch's DPA ([#392](https://github.com/markgoho/doula-cloud/issues/392)) signs it as an individual for all eight — which is correct today, because the LLC does not exist yet ([#377](https://github.com/markgoho/doula-cloud/issues/377) is unfiled). It stops being correct once it does. `doula.cloud` cannot be handed to the entity by moving the account, because seven other sites would go with it; it needs its own account or its own Pirsch organization, owned by the LLC and covered by the LLC's own DPA.

That is not worth doing for the teaser. It is worth doing before January, when the same domain carries the evaluator-facing marketing site and the business should own its own vendor agreements — the same instinct behind the Google Cloud BAA ([#336](https://github.com/markgoho/doula-cloud/issues/336)) and taking Stripe to production under the real entity ([#387](https://github.com/markgoho/doula-cloud/issues/387)).
