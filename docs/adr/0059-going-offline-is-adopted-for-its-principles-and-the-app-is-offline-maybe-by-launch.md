# Going Offline is adopted for its principles, the other books keep their standing, and the app is offline-maybe by launch

DoulaCloud is a client-rendered SPA. `app/vite.config.ts` builds with `adapter-static` and a `200.html` fallback, and `app/src/routes/+layout.ts` sets `ssr = false`, so every route paints blank before its JavaScript boots. [ADR-0002](0002-message-transport-push-triggered-fetch.md) planned the frontend as a PWA with a service worker, and [#61](https://github.com/markgoho/doula-cloud/issues/61) built that service worker for push only: `app/src/service-worker/index.ts` has no `fetch` handler, and its header reads "no offline caching, no asset precaching, per the ticket's explicit scope". [ADR-0020](0020-smoothness-is-gated-on-causes-because-the-outcome-is-not-measurable-where-the-gate-lives.md) named the blank first frame and left its intended answer, a service-worker precache, as fog on [#405](https://github.com/markgoho/doula-cloud/issues/405). [ADR-0024](0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md) left offline affordances to "a separate effort". So nothing decided what the app does when a doula's signal drops, and *Going Offline*, the one book on the reference track that is wholly about that, had no decision under [ADR-0041](0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md), under which a book is evidence and only the founder's recorded decision turns it into a rule.

On 2026-10-24 the founder worked through the theme synthesis at [`docs/research/books/themes/offline-and-resilience.md`](../research/books/themes/offline-and-resilience.md) in the offline and resilience grilling session, [#1723](https://github.com/markgoho/doula-cloud/issues/1723). His first answer gave a business fact that changed the round, so three of its questions were asked again in a revised form, and a new question came from the change. He confirmed the positions record of seven positions the same day ("Confirmed"), with both of the readings that position 4 states as the record's own ([#1723's confirmation](https://github.com/markgoho/doula-cloud/issues/1723#issuecomment-6392489902)). This ADR records those decisions. On *Going Offline*, on when offline arrives, on what a doula does with no signal, and on Client data on the device, the reason is the founder's, in his own words where he gave them; on the dropped-connection message, the two tiers of "loses work", and how the product is checked, he took the recommendation, and the reason is the recommendation's, from round 1 on #1723. A book appears only as evidence weighed.

The business fact under positions 1 to 4, in his words: "this app 100% will ultimately need to be a full pwa with service working and full offline mode, this book may be out of date from an implmeentation standpoint, but probalby has some wisdom to learn from". <!-- spelling:ignore: the founder's words, verbatim -->

**One book is adopted for its principles only, outside the order of sources. Seven of the other eight books with a position on this theme keep the standing earlier ADRs gave them, and *Flexible Typesetting* waits for the visual craft and type decision ([#1722](https://github.com/markgoho/doula-cloud/issues/1722)). The app is offline-maybe, all of it by launch: the app shell is precached for every screen, and on the Staff side a doula reads her Clients and writes notes, Visits and Messages with no signal, while money stays online. Client data is kept on the Staff device only, encrypted, for her Attachments only, and deleted at sign-out, session end, Attachment end and Erasure; the Client portal keeps nothing. A dropped connection gets its own message, "Nothing in this product loses work" becomes two tiers, and the test plans gain a throttle and an Offline step. The product changes, and each change is carried by a ticket under [#1928](https://github.com/markgoho/doula-cloud/issues/1928) or by [#1938](https://github.com/markgoho/doula-cloud/issues/1938).**

## The decision

### *Going Offline* is adopted for its principles only, not its code, outside the order of sources

*Going Offline* (Jeremy Keith, 2018; [read-back](../research/books/going-offline.md)) is adopted for its principles only, not its code, in the shape [ADR-0058](0058-atomic-design-names-the-tiers-and-two-consumers-that-should-change-together-earn-a-variant.md) uses for *Atomic Design*. It takes no place in [ADR-0021](0021-govuk-is-the-reference-for-service-patterns.md)'s order of sources (GOV.UK, then NHS.UK and ONS, then *Inclusive Components* and Adrian Roselli's writing (and, for forms, *Form Design Patterns*), then Every Layout, then original work with a recorded reason). The order ranks what a screen does; this book answers how the app reaches the device, which no rank answers. ADR-0041 asks an adopt-or-not decision where a new reference sits in an existing order of sources; for this book the answer is nowhere, on purpose.

The principles adopted, each with its page:

- A service worker "can only be deployed as an enhancement", so supporting browsers gain and others lose nothing (Ch. 2, pp. 10–11).
- The fetch logic is cache, then network, then a fallback (Ch. 5, p. 62).
- It is designed "on a case-by-case basis" (Ch. 5, p. 67), with "no one-size-fits-all service worker" (Ch. 9, p. 135).
- Freshness is weighed against speed page by page (Ch. 6, pp. 86–87).
- A device has finite room, and storage is shared and not limitless (Ch. 6, p. 93; Ch. 7, p. 94).
- The build starts from the core task and the simplest technology, with service workers layered on after (Ch. 9, p. 133).

The code and the data parts come from today's service-worker and storage documentation, as ADR-0041 says for primary documentation.

The founder's reason (#1723, Q1 and position 1), in his words as the answers comment records them: the book "may be out of date from an implementation standpoint, but probably has some wisdom to learn from".

Weighed against: its recipes are for a public site whose pages are articles, cached as a person browses or saved for offline (Ch. 6, p. 86; Ch. 8, pp. 108–110); syncing data, which positions 3 and 4 need, is a frontier the book names and does not build (Ch. 8, p. 119); and the read-back records that it never names iOS Safari, that its install criteria were one vendor's and have changed, that it says nothing about testing a service worker, and that its storage figure has no source (read-back sections 7 and 8).

Rejected: round 1's recommendation, keep it as evidence, because the founder holds that the book has wisdom to learn from beyond a single claim; adopting it as the source for how the service worker works, because its implementation may be out of date; and declining it.

### The other books on the track

Eight more books have a position on this theme, as the synthesis records. None is made a source here, and each keeps the standing an earlier ADR gave it: *Form Design Patterns* is a rank-3 source for forms by [ADR-0055](0055-form-design-patterns-is-a-rank-three-source-for-forms.md); *Inclusive Components* is at rank 3 by [ADR-0057](0057-inclusive-components-stays-a-rank-three-source-and-a-control-that-acts-at-once-is-a-switch.md); *Inclusive Design Patterns* is declined as a source by ADR-0055 and ADR-0057, and *Designing User Interfaces* by ADR-0055; *Atomic Design* is adopted for the tier names and the template–page split only by ADR-0058, and *Laying the Foundations* and *Design Systems* stay evidence by the same ADR; and *Flexible Typesetting* is the visual craft and type theme's, on [#1722](https://github.com/markgoho/doula-cloud/issues/1722). Where a position below weighs a claim from one of them, the claim and its page are named as evidence, not the book as a source. *Refactoring UI*, *Expressive Design Systems* and *Giving a Damn About Accessibility* have no position on this theme. This ADR adds no rank for any of them.

### Offline-maybe, all of it by launch

The app works online as it does today, and offline is the fallback when the signal drops. The server is the primary copy, never the device. All of it is due by launch (January 2027), not only the app shell. It has two parts:

- **The app shell.** The service worker precaches the app's own built files (its code, styles, fonts and icon) at install and answers every navigation with the app's shell, so the first frame paints from the device and an open with no signal shows the app's own screen (position 5) and not the browser's. On each deploy the precache is replaced on the next visit, not kept. The Client portal gets the shell too; it holds no person's data. Data is fetched from the network first, and a request that hangs is given up after a time limit and falls back, so a flaky signal does not leave her waiting.
- **The offline data**, which positions 3 and 4 name.

The founder's reason (#1723, Q2 and position 2): he declined "offline-first" as the frame, in his words, "I'm not sure "offline-first" is how I'd want to think of the approach, more like "offline-maybe"". The date, all of it by launch, rests on his business fact above ("100% will ultimately need to be a full pwa with service working and full offline mode"), and pre-launch is when it is cheap (`CLAUDE.md`: "Schema and API changes are cheap right now, and get expensive in January").

Weighed for: Keith's pages fetched from the network first and copied as she browses, so she may "never even notice" a lost connection (Ch. 6, p. 86), which is offline-maybe's shape; Keith's own word for the Offline First name, "somewhat misleading" (Ch. 6, p. 93); his offline page cached at install and served in place of the page asked for, without a redirect (Ch. 5, pp. 63–65); networks that "go bad" on a train or in a hotel room (Ch. 1, pp. 1–2); and Remy Sharp's point, quoted by Keith, that people will come to expect an app on the home screen to work (Ch. 9, p. 135).

Weighed against: Keith's cache-first returning visit, near instant online or not, and "a flaky connection" that can be worse than none (Ch. 6, pp. 92–93), which argues for the device first. That is why network first gives up on a request that hangs and falls back.

Rejected: offline-first, the cache-first frame Keith calls Offline First (Ch. 6, p. 93), in the founder's words above; round 1's recommendation, the shell precache alone with offline data left for later, because the business fact makes full offline the destination; #405's "not before the shell and Templates land" as the date; and push only, #61's scope. Built on [#1929](https://github.com/markgoho/doula-cloud/issues/1929).

### With no signal, a doula reads her Clients, writes notes and Visits, and uses Messages; money is not offline

With no signal, on the Staff side, a doula reads her Clients, writes notes and Visits, and uses Messages. A write made with no signal is kept on the device and sent when the signal returns. Payments and invoices refuse with no signal, with position 5's words. What is kept is set by whose Clients they are (position 4), not page by page.

The founder's answer (#1723, Q4 and position 3), as the answers comment records it: "Read her Clients; write notes and Visits; messages. Money (payments, invoices) is not in offline mode." The reason is the business fact above. The record holds no further words of his for leaving money out.

Weighed for: Silver's people who work on the go, are interrupted, and do parts of a long task offline (*Form Design Patterns*, Ch. 10, p. 376); Keith's networks that go bad (Ch. 1, pp. 1–2); and Keith's Background Sync, by which data "can be synced" (Ch. 8, p. 119).

Weighed against: that same page names syncing as a direction beyond the book's recipes, not a recipe, so the book gives no answer for a queued write that meets a later Staff edit; and Keith's "somewhat presumptuous" for caching a whole site on a device with finite room (Ch. 6, p. 93), which is why the scope is her Clients and not the Practice's.

Rejected: round 1's recommendation, that the app says it cannot reach the service and she waits, with the Birth Plan PDF as her copy, *Inclusive Design Patterns*' "easiest offline solution" (The Document, p. 40), because the business fact asks for full offline; read only, because she also writes; and money offline. Not adopted: Keith's Save for Offline control (Ch. 8, pp. 108–110), because what is kept is set by whose Clients they are.

Each queued write carries its `Idempotency-Key` from the moment she makes it, and the replay when the signal returns reuses it ([`docs/api-design.md`](../api-design.md) section 3; [#1924](https://github.com/markgoho/doula-cloud/issues/1924) first). The audit trail for a write made offline records who made it, when she made it on the device, and when the server received it on `clock.Now(ctx)` (`CLAUDE.md`'s Audit trail and Time lines). The record names, and does not decide, the rule for a queued write that meets a later edit by another Staff member; that rule, which notes are in scope, and what sign-out does to a write not yet sent (where this position meets position 4's deletion at sign-out) go to the founder on [#1932](https://github.com/markgoho/doula-cloud/issues/1932). Built on [#1931](https://github.com/markgoho/doula-cloud/issues/1931) (offline read) and [#1933](https://github.com/markgoho/doula-cloud/issues/1933) (the write queue).

### Client data is kept on the Staff device only, and guarded; the Client portal keeps nothing

The guard: only that doula's own Clients, encrypted on the device, deleted at sign-out and when the session ends, and never in plain `localStorage`. The founder confirmed two readings that the positions record stated as its own ([#1723's confirmation](https://github.com/markgoho/doula-cloud/issues/1723#issuecomment-6392489902)):

- **"Her own Clients"** is read through [`GLOSSARY.md`](../../GLOSSARY.md)'s **Attachment**, "the only answer a Doula has to "which of these forty Engagements are mine"". The copy holds the Clients of the Engagements she is attached to, and it cannot outlive the reach that put it there, so it drops a Client when the Attachment ends and at **Erasure**, which shreds the Client's key on the server. An Owner or Admin who is not attached to an Engagement keeps no offline copy of its Client.
- **The guard reaches the reload mirror too.** On the Staff side, a typed Client field kept for a reload (the intake's `sessionStorage` mirror from [#466](https://github.com/markgoho/doula-cloud/issues/466) among them) is encrypted and deleted at sign-out like the rest. On the Client portal, a form keeps what was typed through a failed send only, the reason position 6 allows.

The founder asked first, in his words: "I'm not actually sure why we have this rule - is it legal? hipaa? security?". The answer put to him, which his choice rests on: no ADR or legal record holds [#1715](https://github.com/markgoho/doula-cloud/issues/1715)'s box "No Client field is written to `localStorage` or IndexedDB"; it is an acceptance box written while the synthesis was drafted (#1714), and its reason is security (a shared or family device, after sign-out, with no audit). HIPAA does not forbid ePHI on a device; the Security Rule (45 CFR §164.312) asks for access control, automatic logoff and encryption (an addressable specification), and data encrypted to HHS's guidance is not "unsecured PHI", so a lost device holding it is not a reportable breach. [#472](https://github.com/markgoho/doula-cloud/issues/472)'s concern is the Client portal on a shared device, so the portal keeps nothing. He chose the Staff side only, guarded, as the answers comment records it.

Weighed for: Keith's copy kept as she browses, so a lost connection may go unnoticed (Ch. 6, p. 86).

Weighed against: no book on the track weighs a shared device, encryption or deletion at sign-out; and Keith's storage is shared and not limitless (Ch. 7, p. 94), which offline-maybe accepts, because the device is never the primary copy.

Rejected: #1715's box as written, because no record holds it and encryption with deletion at sign-out answers its security reason; plain `localStorage`; and any Client data on the portal, because #472's shared-device concern holds there. Built on [#1930](https://github.com/markgoho/doula-cloud/issues/1930) (the encrypted store) and [#1934](https://github.com/markgoho/doula-cloud/issues/1934) (the reload mirrors); #1715's box is amended on [#1938](https://github.com/markgoho/doula-cloud/issues/1938).

### A dropped connection is told apart from a server failure

Where a request never reached the service (the browser's request fails outright), every form, list and error page says "We could not reach the service. Check your connection and try again.", as the sign-in screens already do. A 5xx keeps the GOV.UK words ("There is a problem with the service. Try again in a few minutes."). No "You are offline" banner. With position 3, a write she makes with no signal is kept and sent later, so this message is for what offline mode does not cover (money, and a first open before the shell is installed) and for a list or record not on the device.

The founder took the recommendation (#1723, Q3 and position 5), so the reason is round 1's: the sentence already exists in `app/src/lib/formErrors.ts`'s three Identity Platform mappers; it is a line a person would say aloud (the brief's Voice); and it gives her the next step that works, where "something went wrong on our end" is false for a doula with no signal, and waiting a few minutes does not bring the signal back.

Weighed for: Silver's well-written error message, which "may have to suffice" when the network fails (*Form Design Patterns*, Ch. 3, p. 177). Weighed against: Keith's fallback, where a server down and a person who "has lost their internet connection" end the same way (Ch. 5, p. 62); round 1 read that as which page the code falls back to, not what the page says.

Rejected: one message for every failure, because it is false for the common case; and the banner, because `navigator.onLine` can report a connection on a network that has no internet, so the banner would sometimes lie, and the brief's Motion rule, as its Goal-Gradient and Zeigarnik conflict applies it, refuses "a banner that follows a person around" for a reason that holds here. The alignment table gains a row for a lost connection. Built on [#1935](https://github.com/markgoho/doula-cloud/issues/1935).

### "Nothing in this product loses work" is two tiers

The intake survives a closed tab with a server draft (#1715). Every other form keeps what was typed through a failed send and through a reload in the same tab, or records why it does not. A write made with no signal is position 3's, kept and sent later, so it does not lose work either. The brief's Zeigarnik row is reworded to say this, so it promises what is built.

The founder took the recommendation (#1723, Q5 and position 6), so the reason is round 1's: a reload (pull to refresh, a link out and back) is the interruption #466 found on a phone, and a mirror in the tab answers it for any form with the helper intake already has; a server draft for every form is a table and an API per form, for a closed tab that only intake, with its many fields, is likely to meet.

Weighed for: Silver's "jump back into the middle" and saved progress (*Form Design Patterns*, Ch. 10, pp. 376, 378, 381); the Malewiczes' state for progress that "is interrupted" (*Designing User Interfaces*, Animation, p. 305); and Keith's reader who may "never even notice" (Ch. 6, p. 86). No book argues for losing typed work.

Rejected: a server draft for every form, for the cost above; and the row written as the code is today, because it leaves the reload loss that #466 found on every form but intake. The merge screen (`app/src/lib/editMergeDraft.svelte.ts`, "No `sessionStorage` mirror") and the feedback drawer ([#1502](https://github.com/markgoho/doula-cloud/issues/1502)) each gain the mirror or record their reason. Built on [#1936](https://github.com/markgoho/doula-cloud/issues/1936); the brief is reworded on #1938.

### A slow and an absent connection are part of how the product is checked, with no timed CI gate

`CLAUDE.md`'s Performance line gains a slow connection beside a real Practice's data. Each test plan's Time step runs under one throttling profile named in [`docs/test-plans/README.md`](../test-plans/README.md). Each journey's plan gets a step with the browser set to Offline that checks positions 2, 3 and 5. Automated end-to-end tests cover offline read and queued writes, because they test behavior and not speed. No timed check runs in CI.

The founder took the recommendation (#1723, Q6 and position 7), extended for full offline: round 1's Offline step waited for the precache, and offline read and queued writes, which round 1 did not have, gain automated tests. The reason is round 1's: the Time step is already manual, so a named throttle costs the walker one setting and makes the 400ms budget mean what a doula on a phone meets, and the Offline step is the only place a person sees the shell and the message working.

Weighed for: *Inclusive Design Patterns*' "privileged and an unrepresentative" broadband and its "Regular 3G" throttle (A List of Products, pp. 204–205); Brown's evaluation "without a network connection" and under throttling (*Flexible Typesetting*, Ch. 6, p. 179); *Inclusive Components*' low-end mobile throttle (A Content Slider, p. 201); Keith's offline page tested with the browser's Offline box (Ch. 5, p. 65); and Frost's "impact of the network" that only the browser shows (*Atomic Design*, ch. 4, p. 119).

Weighed against: the read-back's record that *Going Offline* says nothing about testing a service worker (read-back section 8), so the tests come from today's documentation; and ADR-0020, under which latency cannot be read honestly on a shared runner.

Rejected: holding the checks as they are; and a timed check in CI under throttling, the gate ADR-0020 already refused for the reason it gives. Built on [#1937](https://github.com/markgoho/doula-cloud/issues/1937) (the test plans), with the end-to-end tests as acceptance criteria on #1929, #1931 and #1933; the `CLAUDE.md` line is #1938's.

### Where the other conflicts rest

The synthesis names six conflicts. Two are decided above: what the person keeps when the network goes (positions 3, 4 and 6), and the evidence each book rests on (the ground of position 1). Position 5 decides what a failure says, a question beside the conflict on what the failure page is for, which holds below. The other four hold on records that predate this ADR, as round 1 on #1723 sets out:

- **Update in place, or load a page.** `ssr = false`, [#1720](https://github.com/markgoho/doula-cloud/issues/1720)'s position 6 and ADR-0057, and `paginatedList.svelte.ts` from [#506](https://github.com/markgoho/doula-cloud/issues/506).
- **Whether screen size says anything about the connection.** `CLAUDE.md`'s Layout line and ADR-0024; nothing uses `srcset` or `<picture>`.
- **How a wait is shown.** The brief's "Loading is skeletal, not spinning", its Doherty Threshold row, and `Skeleton`. The Doherty row's "done optimistically" also holds: no book names it, and no screen does it.
- **What the failure page is for.** ADR-0021 and the alignment table's *Service unavailable* and *There is a problem with the service* rows, Aligned on `ErrorPage` ([#471](https://github.com/markgoho/doula-cloud/issues/471)). Keith's Trivago maze (Ch. 5, p. 65) loses at rank 1.

The other standing records the round named also hold: ADR-0020's 100ms and 400ms budgets, still not checked in CI (position 7 checks them by hand under a throttle); `ssr = false` and the Dialog departure (#1720's position 6, ADR-0057); ADR-0002 and [ADR-0026](0026-two-populations-two-sign-in-methods-and-one-token-table.md), a PWA (the offline part of Keith's three, Ch. 9, p. 121, is position 2); [ADR-0056](0056-the-hosting-rewrite-stays-and-a-google-run-caller-on-the-direct-path-is-an-accepted-residual.md), the hosting, since a service worker runs in the browser; [ADR-0010](0010-notification-email-delivery-is-an-outbox-not-in-request.md) and [ADR-0013](0013-outbox-delivery-gets-a-cloud-tasks-nudge.md), the outbox, which is the server's resilience to a vendor's outage; a copy a person holds (`GLOSSARY.md`'s **Birth Plan**, [#306](https://github.com/markgoho/doula-cloud/issues/306), with its print style sheet and PDF); fonts and a web font as an enhancement (the brief's metric-compatible fallback, #1722's); fewer and smaller requests (cursor pagination, [`docs/api-design.md`](../api-design.md) section 4); and the other books' standing (above).

Two hold with an update:

- **The `Idempotency-Key`** (`docs/api-design.md` section 3) is filed as #1924, and position 3 makes it the mechanism a queued offline write depends on, not only a guard on "Try again": each write carries its key from the moment she makes it, and the replay when the signal returns reuses it.
- **What is kept while a person types an intake** stays the brief's Zeigarnik row and #1715's server draft, with #1715's box amended by position 4.

### What the books add to standing decisions, and what they change

- **ADR-0020.** Changes: "The blank first frame" is answered. Its intended service-worker precache, fog on #405, is decided and due by launch (position 2), recorded by a dated note under the ADR (#1938). Holds: the 100ms and 400ms budgets stay unchecked in CI, for the reason it gives (position 7).
- **ADR-0024.** Holds unchanged. Offline affordances stay outside the layout commitment; the "separate effort" it names is now decided by this ADR, not by ADR-0024.
- **#61's scope.** Changes: the service worker is no longer push only. It gains a `fetch` handler, the app shell precache, and the offline data of positions 3 and 4, and `app/src/service-worker/index.ts`'s header ("no offline caching, no asset precaching") is rewritten to the new scope (#1929).
- **`CLAUDE.md`'s Performance line.** Changes: it names a slow connection beside a real Practice's data (position 7, #1938).
- **The brief's Zeigarnik row.** Changes: "Nothing in this product loses work" is reworded to the two tiers of position 6, with a write made offline kept and sent later (position 3, #1938).
- **#1715's box "No Client field is written to `localStorage` or IndexedDB".** Changes: amended to position 4's guard. No Client field is written to plain `localStorage`; on the Staff side, only the doula's own Clients are kept, encrypted, and deleted at sign-out and when the session ends; the Client portal keeps nothing (#1938).
- **#472's count, "the service worker caches nothing per #61".** Changes on the Staff side, where the service worker now keeps the app shell and, under position 4's guard, her Clients. Holds for anything about a Client on the portal, which keeps no Client data; the portal gets the shell, which holds no person's data.
- **The alignment table.** Adds a row for a lost connection (position 5, #1935).
- **`docs/test-plans/README.md`.** Adds one named throttling profile for every Time step, and an Offline step in each journey's plan (position 7, #1937).

## What was considered instead

**Adopt nothing.** A legitimate outcome under [#1684](https://github.com/markgoho/doula-cloud/issues/1684), and the decline option on #1723's Q1. Rejected for *Going Offline* under position 1, because the founder holds that the book has wisdom to learn from. Taken for the other eight books, which keep their earlier standing.

**Offline-first.** Rejected under position 2, in the founder's words: "more like "offline-maybe"". The server stays the primary copy.

**The app shell now, the offline data later.** Round 1's recommendation on Q2, rejected under position 2, because the business fact makes full offline the destination and pre-launch is when it is cheap.

**Wait for the signal, with the Birth Plan PDF as her copy.** Round 1's recommendation on Q4, rejected under position 3, because the business fact asks for full offline.

**Keep #1715's box, and so no Client data on any device.** Rejected under position 4, because no record holds the box, and encryption with deletion at sign-out answers its security reason.

## Consequences

- **The pattern tables are records that already exist, and no table of its own is made.** ADR-0041's third reference-track stage asks for a table an agent checks before building the thing the book is about. What a screen says when the connection drops is a row in [`docs/design/govuk-alignment.md`](../design/govuk-alignment.md) beside the *Service unavailable* and *There is a problem with the service* rows (position 5), which the `CLAUDE.md` Service patterns section already links. What is kept while a person types is the brief's Zeigarnik row (position 6), in the Laws of UX table that the Experience axis of `/code-review` reads in review ([#1682](https://github.com/markgoho/doula-cloud/issues/1682)). How the product is checked is `CLAUDE.md`'s Performance line and `docs/test-plans/README.md` (position 7). *Going Offline*'s principles are not patterns a screen is checked against; they shape one service worker, so they stay in position 1 of this ADR. No `CLAUDE.md` section governs service-worker or offline work today, so the rewritten Performance line links this ADR (#1938), and so does the rewritten header of `app/src/service-worker/index.ts` (#1929). A second table would make the next builder check two places for one rule. This follows ADR-0055, ADR-0057 and ADR-0058, which each decided their stage-3 table in their own Consequences.
- **The product changes.** ADR-0057 and ADR-0058 change nothing in the product, and ADR-0055 changes two atoms ([#1751](https://github.com/markgoho/doula-cloud/issues/1751)); this one changes how every screen reaches the device. Every screen opens from the app shell, with no signal too; on the Staff side, a doula reads her Clients and writes notes, Visits and Messages with no signal; her Clients are kept on her device under position 4's guard; a dropped connection says so; and every form keeps typed work through a reload. The build is under the parent [#1928](https://github.com/markgoho/doula-cloud/issues/1928), a sub-issue of [#1681](https://github.com/markgoho/doula-cloud/issues/1681) beside #1684, each piece carried by its own ticket:
  - The service worker's `fetch` handler, the app shell precache for the Staff side and the portal, network first with a time limit, and the precache replaced on each deploy (position 2): [#1929](https://github.com/markgoho/doula-cloud/issues/1929).
  - The encrypted on-device store, scoped to her Attachments and deleted at sign-out, session end, Attachment end and Erasure, with nothing on the portal (position 4): [#1930](https://github.com/markgoho/doula-cloud/issues/1930).
  - Offline read of her Clients, their Engagements, Visits and Messages (position 3): [#1931](https://github.com/markgoho/doula-cloud/issues/1931), blocked by #1929 and #1930.
  - The founder's decision on what a queued write does when it meets a later Staff edit, which notes are offline, and what sign-out does to a write not yet sent (positions 3 and 4): [#1932](https://github.com/markgoho/doula-cloud/issues/1932).
  - The offline write queue for notes, Visits and Messages, each write with its `Idempotency-Key` from the moment it is made, and the audit trail with device time and server time (position 3): [#1933](https://github.com/markgoho/doula-cloud/issues/1933), blocked by #1924, #1930 and #1932.
  - The Staff-side reload mirrors under the same guard, and no mirror on the portal (position 4): [#1934](https://github.com/markgoho/doula-cloud/issues/1934).
  - The lost-connection outcome in `formErrors.ts`, `ErrorPage` and `paginatedList.svelte.ts`, and its alignment-table row (position 5): [#1935](https://github.com/markgoho/doula-cloud/issues/1935).
  - The in-tab mirror, or a recorded reason, for every form other than intake (position 6): [#1936](https://github.com/markgoho/doula-cloud/issues/1936), blocked by #1934.
  - The named throttling profile and an Offline step in each journey's test plan (position 7): [#1937](https://github.com/markgoho/doula-cloud/issues/1937). The end-to-end tests for the shell, offline read and queued writes are acceptance criteria on #1929, #1931 and #1933, so no ticket carries tests apart from the code they test.
  - The client `Idempotency-Key` and the portal's wrapped POSTs: [#1924](https://github.com/markgoho/doula-cloud/issues/1924), already filed under #1684 from round 1 on #1723, which blocks #1933.
- **Changes to the records, carried by one ticket under #1684:** `CLAUDE.md`'s Performance line, the brief's Zeigarnik row and its smoothness line, a dated note under ADR-0020's "The blank first frame", #405's precache bullet, and #1715's box: [#1938](https://github.com/markgoho/doula-cloud/issues/1938).
- The old service-worker path that ADR-0020, the *Going Offline* read-back and #405's body still name is a conformance finding, not a decision, and stays with [#1709](https://github.com/markgoho/doula-cloud/issues/1709); whichever of #1709 and #1938 lands second keeps `app/src/service-worker/index.ts` as the path.
- `docs/manifesto.md` is unchanged, since it indexes value ADRs only, and the book table in [`docs/agents/literature.md`](../agents/literature.md) moves to **decided** in #1684's sweep after all five reference-track ADRs land.

**Amended, 2026-10-24 ([#1932](https://github.com/markgoho/doula-cloud/issues/1932)):** the founder answered the three questions that positions 3 and 4 left open, and #1933 is built to them.

- **A queued write meets a later edit: merge by field.** Fields that differ both apply, so a Visit keeps the time another Staff member changed and the notes she wrote offline. If both changed the same field, the person whose write arrives later sees "This changed while you were offline" with both versions and picks one. A new Message never conflicts; it is added. The audit trail keeps the device time and the server time.
- **Which notes: Visit notes only.** A Visit and its own free-text notes are offline. An Engagement status change and its note need signal, because a status change can start other things.
- **Sign-out with writes not yet sent: block, with an override.** Sign-out says how many writes are not sent yet and asks her to stay signed in until she has signal, or to delete them and sign out; she must choose. If the session ends by itself (it times out), the encrypted queue stays on the device and is sent when she signs in again as herself. This is where position 3's queue and position 4's deletion at sign-out meet: at a sign-out she chooses, deletion waits for her choice; at a session that times out, the queue outlives the session, still encrypted, and only she can send it.
