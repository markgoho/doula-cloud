# Lena Vasquez — test plan

- **Journey**: [contractor-doula.md](../journeys/contractor-doula.md)
- **Persona**: [contractor-doula.md](../personas/contractor-doula.md)
- **A pass means**: the Engagement she took is finished, she can point to what she agreed and what she was paid, and she never saw a Client who was not hers.

Every stage can be walked now. Accepting a second Practice's invitation on an account she already holds succeeds. The Practice offers her the work, and she accepts or declines it on its own screen. She reaches only the work she accepted, and that reach ends when the Engagement completes. The Practice records paying her fee, and she reads it on **Your offers** after the job ends (LV-G3, closed by [#2271](https://github.com/markgoho/doula-cloud/issues/2271)).

## Preconditions

- **Two** Practices: the other agency where Lena already works, and the one that invites her. She holds a Staff account at the first one already — that is the point of her. On the `dev:full` seed, Rooted Birth Collective is her first agency, so the second walk built the other one by hand and walked stage 1 on its invitation (see its Run log).
- At the Practice she works for in stages 3 to 8: an Owner, an Engagement with no Doula yet to offer her, and Clients who are not hers.
- **No fixture bypass for her second membership.** Step 1.2 produces it: accepting the invitation on the account she already has resolves to that existing `staff` row and inserts a `practice_memberships` row for the new Practice.
- The invitation carries her employment type (**Contractor**), and an Offer is made through the product (**Offer this work**). Stage 3 needs nothing provisioned.

## Steps

### Stage 1 — Join the agency, on an account she already has

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link at `/accept-invite` | "Join *Practice name*", with the roles and the invited address under it, and one question: Password. No account question and no email field ([#2238](https://github.com/markgoho/doula-cloud/issues/2238)) | `manual` |
| 1.2 | Sign in as herself and submit | **I already have an account — sign in** reveals Email address and Password. **Continue** shows "Check your details", her name and work state "from the Staff account you already have", and **Accept invite**. `staffauth`'s accept path (`resolveStaff`, `accept.go:306`) finds her existing `staff` row by `identity_uid` and reuses it: **`200`**, holding the same `staffId` her other agency already gave her and the inviting Practice's `practiceId`. The screen lands on the Practice she joined, with "You've joined *Practice*." and the role and employment type, and that Practice is her last-used one from then on. **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** Was: it landed on the Practice she used last ([LV-G10](https://github.com/markgoho/doula-cloud/issues/2265)) | `manual` |
| 1.2-a | Check for a second membership | **It exists.** The same request that succeeded inserted a `practice_memberships` row for the inviting Practice on her existing `staff` row (`accept.go:226`), rather than a second `staff` row. The header's Practice switcher lists both | `manual` |
| 1.3 | Record that she is a contractor, not an employee | The invite form asks **Employment type**, Employee or Contractor, and the inviting Owner's Staff roster shows her as "Contractor" in its Employment type column | `manual` |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The accept form, then the Practice it joins her to | `manual` |
| 1.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law) | `manual` |
| 1.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The screen names the agency she has joined and that her other one is untouched | `manual` |
| 1.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 2 — Sign in and choose the agency

Reachable once stage 1 is walked — step 1.2 is what gives her the second membership this stage's picker needs.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Sign in at `/login` | `POST /api/session` succeeds | `automated (staff-login.e2e.ts)` |
| 2.2 | Choose Rooted Birth Collective from **two** memberships | Sign-in shows no picker: it lands on the Practice she used last. The header's Practice switcher, "Your practices", lists both, and choosing one moves her there | `manual` |
| 2.2-a | Read what she is at each Practice from the picker | The switcher shows her roles and her employment type at each: "Doula, as a contractor", "Doula, as an employee". **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** Was: her role only ([LV-G11](https://github.com/markgoho/doula-cloud/issues/2266)) | `manual` |
| 2.3 | Read the top bar | Overview, Clients, Schedule, On call, Offers, Settings. No Credits, Invoices, Contracts or Staff. The Overview draws no Quick links ([#2398](https://github.com/markgoho/doula-cloud/issues/2398)). Settings holds only Plan Templates | `manual` |
| 2.3-a | Open **Billing** (Credits) by its address | Refused: "You cannot view this", "Your role does not have permission to see this. Trying again will not change that.", **Go to your Practice overview** | `manual` |
| 2.3-b | Open **Getting paid** by its address | The billing mode ("By card, through Stripe") and payment terms, which ADR-0008 lets her read for her own Engagements, then "Only a Practice Owner or Admin can see how this Practice gets paid." No Stripe Connect status and no control | `manual` |
| 2.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). `/login` with its Practice picker, then the Practice landing | `manual` |
| 2.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law) | `manual` |
| 2.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). The picker says what she is at each Practice, so she need not recall which agency is which | `manual` |
| 2.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 2.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 3 — The offer (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Receive the offer of the February Engagement | Renata's **Offer this work** on an Engagement with no Doula yet, to **Someone already at this Practice**, asks for a fee once a contractor is chosen. **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The Offer is also mailed to Lena: "You've been offered work on DoulaCloud", with a link to **Your offers** at the Practice that sent it and the date to answer by. Read it in the catcher, open the link signed out, sign in, and check she lands on that Practice's **Your offers** ([LV-G12](https://github.com/markgoho/doula-cloud/issues/2267)). Before: the Offer reached Lena only on her Overview ("Offers awaiting your answer") and **Your offers** | `manual` |
| 3.2 | Read enough to take or refuse it — Client, dates, on-call terms, fee — while still an outsider | The Client's first initial, the general area, the due date (printed `2027-02-14`, [#2236](https://github.com/markgoho/doula-cloud/issues/2236)), the fee and the terms, then **Accept** and **Decline**. No Client name, which is [#230](https://github.com/markgoho/doula-cloud/issues/230)'s offered-not-accepted rule | `manual` |
| 3.3 | Decline, and have Renata see the refusal | **Decline** records it at once. The card goes, and **Your offers** keeps a "Declined" row. Renata's Engagement Offers section reads "Declined" and Activity reads "Offer for the Client's care declined", Lena Vasquez. Renata gets no mail | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). The offer itself: Client, dates, on-call terms and fee on one screen | `manual` |
| 3.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law). The Overview she meets the Offer on, each headed group one decision: her Offers (Accept or decline), Recent activity (RA-G11, [#2228](https://github.com/markgoho/doula-cloud/issues/2228); the Quick links dropped by [#2398](https://github.com/markgoho/doula-cloud/issues/2398)) | `manual` |
| 3.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Nothing from the conversation with Renata has to be recalled; the terms are on the screen she decides on | `manual` |
| 3.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 3.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). Accepting says the Engagement is now hers and where to find it; declining says Renata has been told | `manual` |
| 3.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 4 — Find the one job she took

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients` | The list renders for a non-owner | `manual` |
| 4.2 | Count the rows | **Only hers.** "Showing 2 Clients with work.", the two Engagements she holds a granted attachment on, and **See everyone** shows the same two. Nothing else of the agency's book reaches her (ADR-0008) | `manual` |
| 4.2-a | Pick hers out | Every row is hers, and each says "Your fee" with the fee she accepted | `manual` |
| 4.1-a | Add a Client to the agency's book | **She is refused, by name.** The list offers no **Find or add a Client**. At `/clients/search` she reads "Work at this Practice reaches you as an Offer, so there is no Client to search for or add here. To take on Clients of your own, set up a Practice." `client.CreateHandler` and the `clients_insert` RLS policy refuse the write independently ([LV-G9](https://github.com/markgoho/doula-cloud/issues/292) closed) | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 4.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Her Engagement is marked as hers, so she need not remember the Client's name from the offer | `manual` |
| 4.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 5 — Check the terms and the fee

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open her Engagement | The single-page view renders | `manual` |
| 5.2 | Read the Contract section | **She does not get the Client's price, and she reaches only her own work.** On a Draft Contract the Owner reads as "Fee $1,500.00", she reads "Status Draft, not sent yet", "Your fee $550.00, the fee you accepted on Oct 28, 2026" and **Read the Contract text**. The Contract read's body carries the prose, the merge fields and their values with no `price` key (`priceForReader`). The Invoices section says "You see when each Invoice was raised and where it stands, not what the Client pays. Your own fee is in the Contract section." | `manual` |
| 5.2-a | Price and send a Contract on a Client who is not hers | **She never reaches the Engagement.** Another Doula's Engagement address answers "Page not found", and its reads and `POST .../contract/send` answer `404`, the same as an Engagement that does not exist. A Contract's price is resolved from the rate card for anyone ([PR-G8](https://github.com/markgoho/doula-cloud/issues/282) closed) | `manual` |
| 5.3 | Find what *she* is owed | "Your fee $450.00, the fee you accepted on Oct 27, 2026" in the Contract section of her Engagement, and "Your fee" on each Clients row. What she was *paid* is stage 7 | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 5.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). The terms and fee she accepted at the offer are on the Engagement she reads now | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — Do the work

Identical to Priya's stages 6–8 and marked there; walk [employed-doula.md](employed-doula.md) steps 6.1 to 8.2 as Lena and record only where a contractor differs. Nothing is expected to — which is the finding: the care half is the half the product already treats correctly.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Read the Birth Plan | As Priya 6.1–6.2: the section shows progress, "Last edited" and whether the Client has read it, and every answer, read-only, with **Print for the hospital** beside **Change the Birth Plan** ([DW-G14](https://github.com/markgoho/doula-cloud/issues/2243)). She may also start and change it | `manual` |
| 6.2 | Log a Visit | As Priya 7.1: **Add a Visit** asks only the scheduled date and time, the type is worked out ("Prenatal"), and the row's **Change** writes the date and notes. She logs it for herself; she reaches this Engagement only through the granted attachment her acceptance of the Offer opened. She may also write notes on another Doula's Visit on the same Engagement (the notes rule is "any Staff member who may read a Visit") | `manual` |
| 6.3 | Message the Client | As Priya 8.1: **Write a message**, Message, Attachment, **Send**, **Cancel**; "Message sent." and her message in the thread. The thread opens only once the Client has accepted a portal invite | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). When the Visit was, which kind, what was covered, and the save | `manual` |
| 6.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 7 — Get paid

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Point to what she was paid for the February birth | Renata, on the Engagement's **Doula fees** (her Attachment ended or not): **Mark fee paid**, a Method, an optional Note and **Date paid**, **Continue**, **Confirm and record**: "Recorded: Lena Vasquez paid $450.00 on …, by bank transfer." Lena, on **Your offers**, on the accepted Offer: "Due Feb 2027", the month that tells it from her other accepted jobs ([#2383](https://github.com/markgoho/doula-cloud/issues/2383)), "Accepted on …", "Fee $450.00", "Paid $450.00 on …, by bank transfer", or "Not recorded yet". It still reads so after her Attachment ends. No money moves ([#391](https://github.com/markgoho/doula-cloud/issues/391)) | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). One screen says what she was paid for the February birth | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 8 — The job ends

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | End the attachment when the job is finished | Renata records the birth outcome and presses **Mark care complete**, with a reason ("The work finished as agreed"): "The care phase is now Completed." Completing the Engagement ends every attachment on it (`EndAttachments`), and the Doula row goes. Nothing ends one attachment on its own, and Lena cannot end hers | `manual` |
| 8.1-a | Re-run step 4.1 afterwards | **Her reach on it is gone.** Her Clients list holds only her other open Engagement, the completed Engagement's address answers "Page not found", and its reads answer `404`. Nothing tells her it ended ([LV-G15](https://github.com/markgoho/doula-cloud/issues/2270)) | `manual` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The ended Engagement says it ended, and when, and is no longer in her list | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 9 (8 of them [budget steps](README.md#budget-steps)) |
| `manual` | 52 (13 on the plan as drafted, plus 2.3-a, 2.3-b, 4.1-a and 5.2-a appended by the first walk, plus 1.2-a re-marked 2026-09-20, plus 1.3, 2.2-a, 3.1, 3.2, 3.3, 5.3 and 8.1 re-marked by the second walk, plus 7.1 and 7.B-screens re-marked 2026-10-31; 26 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 0 |

LV-G5 is observed at 4.2 rather than given a step of its own: the list opens, and what it shows is the finding. On the second walk it shows only her own work.

## Run log

### 2026-10-31 — 7.1 walked again with two accepted jobs ([#2383](https://github.com/markgoho/doula-cloud/issues/2383))

**Stack**: `bun run --cwd app dev:full` from the #2383 worktree at port offset 1: the app at `http://localhost:5273`, in WSL2 with rootless Podman, on the branch that adds the due month, based on trunk at `a80f2192`. The seeded Rooted Birth Collective, fresh.

**Devices**: headless Chrome inside WSL, driven by the chrome-devtools MCP, one isolated context for Renata Alvarez and one for Lena, Lena's at 320x800 and 1280x900, in UTC. No throttling, on an unbundled Vite dev server.

**Accounts and preconditions**: the seed's accepted Offer, Lena on **Reese Bishop**'s birth ($450). The seed moves each due date by the days since the World's day zero, so on this day that birth is due in March 2027, not February; the March job stands in for "the February birth". Renata then sent Lena a second Offer from **Casey Estrada**'s Engagement (**Offer this work**, Lena Vasquez, Fee 600, General area "Irondequoit", Due date Nov 1, 2026, Terms "Backup for the birth, on call now."): "Offer sent to Lena Vasquez." Lena accepted it on **Your offers**. Only 7.1 was walked.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 7.1 | `manual` | **walked** | Lena, on **Your offers** at 320 and 1280, two accepted Offers, both "Accepted on Oct 31, 2026" and both "Paid Not recorded yet". Before this change they differed only by the fee. Now the first reads "Due Nov 2026", "Fee $600.00", and the second "Due Mar 2027", "Fee $450.00", so she points to the March birth by its month. Neither shows a Client initial, an area or a full due date. Her inbox read answers `dueMonth` `2026-11` and `2027-03` with `dueDate`, `clientFirstInitial` and `clientArea` empty. Renata's read of Casey's Offers answers the accepted Offer with an empty `dueDate` and no `dueMonth`, as before. At 320 the page's scroll width is 320, with no sideways scroll |

**Not walked again**: the Fee Payment half of 7.1 and the read after her Attachment ends, both walked by the stage 7 entry below. The due month is read from the Offer itself, with no join to her Attachment, so ending it does not change what this row shows.

**Is the journey completable end to end?** Unchanged by this walk. The gap the stage 7 entry noticed is closed: with two jobs she tells them apart by the birth's month.

### 2026-10-31 — stage 7 walked ([#2271](https://github.com/markgoho/doula-cloud/issues/2271))

**Stack**: `bun run --cwd app dev:full` from the #2271 worktree at port offset 3: the app at `http://localhost:5473`, in WSL2 with rootless Podman, on the branch that builds the Fee Payment. The seeded Rooted Birth Collective, fresh.

**Devices**: headless Chrome inside WSL, driven by the chrome-devtools MCP, one isolated context for Renata Alvarez and one for Lena, each at 320x800 and 1280x900, in UTC. No throttling, on an unbundled Vite dev server.

**Accounts and preconditions**: the seed's accepted Offer, Lena on **Reese Bishop**'s birth ($450, "Backup for the birth, on call from 38 weeks, one prenatal visit."). Only stage 7 and the 8.1 completion it needs were walked.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 7.1 | `manual` (was `missing-feature (LV-G3)`) | **walked** | Renata, Reese's Engagement at 320px: **Doula fees**, a card "Lena Vasquez, Fee $450.00, On this birth Since Oct 31, 2026, Paid Not recorded yet", **Mark fee paid**. "Record paying Lena Vasquez $450.00": Method, Note (optional), Date paid (today, prefilled), **Continue**; the check step "Amount $450.00, Method Bank transfer, Note —, Date paid Oct 31, 2026", **Confirm and record**. The status line "Recorded: Lena Vasquez paid $450.00 on Oct 31, 2026, by bank transfer." and the card's Paid and "Recorded by Renata Alvarez". Activity: "Fee paid to Lena Vasquez", Renata Alvarez, with no amount. At 1280 the section is a table: Doula, Fee, On this birth, Paid, Recorded by, Actions. Renata then completed the Engagement (live birth, "The work finished as agreed"): the row reads "On this birth Oct 31, 2026 to Oct 31, 2026", and **Remove payment** with a reason ("Payment to Lena Vasquez removed.") and a second **Mark fee paid**, Other, Note "Zelle, confirmation 4471", both still work on the ended Attachment. Lena, after the end, on **Your offers** at 320 and 1280: "Accepted", "Accepted on Oct 31, 2026", "Fee $450.00", "Paid $450.00 on Oct 31, 2026, by bank transfer", "Terms …". Her read of the Engagement answers `404`, and `GET …/doula-fees` answers her `403` |

**Re-marked**: 7.1 and 7.B-screens move from `missing-feature (LV-G3)` to `manual`. The Marks summary is recounted from 9 / 50 / 0 / 2 to 9 / 52 / 0 / 0, and [README.md](README.md)'s run-status row and Total move with it.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 7.B-screens | 1 | 1: **Your offers** holds the accepted Offer and what she was paid, before and after her Attachment ends | yes |

Renata's acts, timed in the page from the press to the result on screen: **Remove payment** opens its form in 14 ms; **Confirm and remove** 55 ms and **Confirm and record** 51 ms to the status line, each with the row read again. The 320px step stays `automated (route-continuum.svelte.spec.ts)`; neither page scrolled sideways at 320px.

**Is the journey completable end to end?** Stage 7 no longer stops it. Against "a pass means": she can point to what she agreed and what she was paid, on one screen, after the job ends. The other stages are as the 2026-10-28 walk left them; their open findings are not re-walked here. One thing the walk noticed: an accepted Offer names no Client (#230's lapse), so with two jobs she tells them apart by the day she accepted and the fee, not by "the February birth".

### 2026-10-28 — second manual walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177))

**Stack**: the shared walk stack, `bun run --cwd app dev:full` from the main checkout at port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`, in WSL2 with rootless Podman. The record was written on trunk at `c59e5ec5`. `scripts/stripe-listen.sh` was running against it with the Stripe test-mode key, and this walk made no Stripe call. The stack was not restarted.

**Devices**: headless Chrome inside WSL, driven by the chrome-devtools MCP, at 1280x900 in UTC. Her journey names no device. There was no CPU or network throttling. Each person had one isolated browser context: the Ridgeline Owner, Renata Alvarez, Lena at desktop, a fresh context for Lena for 1.B-cold, another for 3.B-cold, and Adaeze Nwosu in the portal. Every timing is on an unbundled Vite dev server, which compiles a route the first time it is visited. Each timed act was repeated warm where it could be.

**Accounts and preconditions**, on the seeded Rooted Birth Collective (`19a0107e-…`) as Priya Raman's walk ([#2176](https://github.com/markgoho/doula-cloud/issues/2176)) left it: 18 Staff, billing by card and connected to Stripe, 14 Credits.

- **Lena** is the seeded `lena.vasquez@sim.doula.cloud`, a contractor Doula at Rooted, signing in with the seed password ([walk-a-screen-locally.md](../testing/walk-a-screen-locally.md)). Her seeded work is **Reese Bishop**'s birth: an accepted Offer ($450, "Backup for the birth, on call from 38 weeks, one prenatal visit."), Doulas Joss Adeyemi and Lena Vasquez, and a Visit on Nov 1, 2026.
- **The plan's order is reversed.** The plan has Rooted invite her second. On the seed, Rooted is her *first* agency. So her second agency was built through the product: a new Owner signed up at `/signup` as `ridgeline.owner@sim.doula.cloud` ("Beth Alvarado"), Practice **Ridgeline Doulas** (`05af3e0f-…`), New York, Eastern time. That Owner invited `lena.vasquez@sim.doula.cloud` from **Invite a Staff member** with **Doula** and **Contractor**. Stage 1 is walked on that invitation, accepted on the account Lena already has. The Owner's password is a generated local-emulator value and is kept out of this document. The other way, a fresh Lena at both Practices, would have lost the seeded Offer and Visit.
- **Stage 3**: Renata saved a new Client, **Adaeze Nwosu** (the February birth), and started work: birth, due Feb 14, 2027, **No Doula yet**, which spent one Credit (14 to 13). That gives Engagement `b4c1151c-…`. She pressed **Offer this work** to Lena with a $500 fee, area "Rochester, South Wedge" and terms. Lena declined it (3.3). Renata sent a second Offer, $550, which Lena accepted, so the accept half of 3.B-confirmation could be read and stage 8 had an Engagement to end.
- **Stage 5**: Renata saved a Draft Contract on Adaeze's Engagement (Fee $1,500.00 from the rate card), so the Contract read had a price to strip.
- **Stage 6**: on Reese Bishop's Engagement for the Visit. Reese has no email, so the message went on Adaeze's: Renata added `adaeze.nwosu@sim.doula.cloud` and sent a portal invite, and Adaeze accepted it in her own context. Lena saved Adaeze's Birth Plan (birth center, atmosphere), so 6.1 had answers to read.
- **Stage 8**: Renata recorded a live birth on Oct 28, 2026 on Adaeze's Engagement and completed it. Reese Bishop's Engagement was left open, so 8.1-a could tell an ended reach from a standing one. Camille Boyd's and Mira Castellanos's Engagements and Contracts were not touched.

The plan's `automated` steps were **not** re-run as specs. 2.1 was walked anyway, since stage 2 sits behind it.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected, cell corrected | The catcher's mail #27, "You've been invited to join a practice on DoulaCloud", names neither Practice nor inviter. Its link, opened in a fresh context: "Accept your Staff invite", "First, sign in or create an account with the address your invite was sent to.", "Do you already have a DoulaCloud account?", **I'm new here — create an account**, **I already have an account — sign in** ("You already sign in to DoulaCloud for another practice."), **Continue**. No Practice is named |
| 1.2 | `manual` | as expected, and **it lands on the wrong Practice** | Email address and Password, **Continue**, then "Check your details": "These come from the Staff account you already have, so we are not asking again.", "Lena Vasquez", "You work from New York, self-reported Oct 27, 2026.", **Change where you work**, **Accept invite**. `POST /api/staff/accept-invite` answered `200` with `staffId` `ac2be253-…` (the same id Renata's Offer form lists for her at Rooted) and Ridgeline's `practiceId`. The screen then went to **Rooted's** Overview, her last-used Practice, and said nothing about joining ([LV-G10, #2265](https://github.com/markgoho/doula-cloud/issues/2265)) |
| 1.2-a | `manual` | as expected | The header's Practice name became a switcher, "Your practices": Rooted Birth Collective / Doula, Ridgeline Doulas / Doula |
| 1.3 | `manual` (was `missing-feature (LV-G1)`) | **walked** | The invite form asks **Employment type**: Employee ("Inside the business, and sees every Engagement at this Practice") or Contractor ("Outside the business, and sees only the work offered and accepted, unless also an Owner or Admin"). Ridgeline's Staff roster: Lena Vasquez, Doula, **Contractor**, New York |
| 2.1 | `automated (staff-login.e2e.ts)` | read on the walk | `/login`, **Sign in**, Rooted's Overview |
| 2.2 | `manual` | **falsified**, cell corrected | No picker. Sign-in went straight to Rooted, her last-used Practice. The switcher took her to Ridgeline, and Ridgeline's address and header then showed **Rooted's** Overview body ("Clients 1 active", Reese Bishop's activity) until a reload, twice ([#2272](https://github.com/markgoho/doula-cloud/issues/2272)). Ridgeline's own Overview offers her **Add your first Client**, which refuses her ([#2273](https://github.com/markgoho/doula-cloud/issues/2273)) |
| 2.2-a | `manual` (was `missing-feature (LV-G1)`) | **walked** | The switcher says "Doula" under each Practice and nothing about employment type. Both of her memberships are Contractor, so the walk could not see the distinction fail, only that there is no place for it ([LV-G11, #2266](https://github.com/markgoho/doula-cloud/issues/2266)) |
| 2.3 | `manual` | **falsified** (the tiles are gone), cell corrected | Top bar: Overview, Clients, Schedule, On call, Offers, Settings. Quick links: Clients "1 active", Your offers, Plan Templates, Contract Template. Settings holds Plan Templates only |
| 2.3-a | `manual` | **holds** (was a read of the whole ledger), cell corrected | `/billing`: "You cannot view this", "Your role does not have permission to see this. Trying again will not change that.", **Go to your Practice overview** |
| 2.3-b | `manual` | **holds**, cell corrected | `/settings/payments`, not linked for her: "Getting paid", "Billing mode By card, through Stripe", "Payment terms Due 30 days after an invoice is raised.", "Only a Practice Owner or Admin can see how this Practice gets paid." No Connect status, no control. The billing mode is hers to read under ADR-0008 |
| 3.1 | `manual` (was `missing-feature (LV-G6)`) | **walked** | Renata: **Offer this work**, **Someone already at this Practice**, sixteen Doulas, Lena among them (no contractor is offered under **Start work**'s "Who is the Doula?"). Choosing Lena adds **Fee (USD)**. The Client's first initial ("A") is filled, but the due date is not, though the Engagement holds it ([#2230](https://github.com/markgoho/doula-cloud/issues/2230), commented). **Send Offer**: "Offer sent to Lena Vasquez." Lena's mail got nothing ([LV-G12, #2267](https://github.com/markgoho/doula-cloud/issues/2267)). She found it on signing in: "Your offers 1 waiting on you" and the card under "Offers awaiting your answer" |
| 3.2 | `manual` (was `missing-feature (LV-G7)`) | **walked** | "Awaiting a decision", Client "A", Area "Rochester, South Wedge", Due date "2027-02-14" ([#2236](https://github.com/markgoho/doula-cloud/issues/2236), commented), Fee "$500.00", Terms "Primary doula for the birth: two prenatal visits, on call from 38 weeks, one postpartum visit.", **Accept**, **Decline**. Above it: "Accepting puts you on the birth; declining is final for that offer." Enough to decide, and no Client name |
| 3.3 | `manual` (was `missing-feature (LV-G6)`) | **walked** | **Decline**, with no check first: the card went and "You have no offers." **Your offers**: "Declined, Fee $500.00, Terms …". Renata's Engagement, on a reload: Offers "Lena Vasquez, Oct 28, 2026, $500.00, Declined", and Activity "Offer for the Client's care declined", Lena Vasquez. Renata's mail got nothing. Her second Offer, $550, was accepted on **Your offers**: "Accepted, Fee $550.00, Terms …", and the Engagement's Doula became Lena Vasquez |
| 4.1 | `manual` | as expected | **Clients** renders for her |
| 4.2 | `manual` | **falsified**, cell corrected | "Showing 2 Clients with work.": Adaeze Nwosu (Birth, Intake, "Your fee $550.00") and Reese Bishop (Birth, Active, "Your fee $450.00"). **See everyone** showed the same two. Rooted has more than thirty Clients |
| 4.2-a | `manual` | **falsified**, cell corrected | Every row is hers, and each carries "Your fee" |
| 4.1-a | `manual` | as expected, cell corrected | No **Find or add a Client** on her list. `/clients/search`: "Add a Client. Work at this Practice reaches you as an Offer, so there is no Client to search for or add here. To take on Clients of your own, set up a Practice.", **Set up a Practice** |
| 5.1 | `manual` | as expected | Reese Bishop's Engagement from her Clients row: "Doulas Joss Adeyemi, Lena Vasquez", On call Mar 3 to Apr 7, 2027 for both, no **Change** on care, due date or phase |
| 5.2 | `manual` | as expected, cell corrected | On Adaeze's, with Renata's Draft (Renata reads "Fee $1,500.00"): "Status Draft, not sent yet", "Your fee $550.00, the fee you accepted on Oct 28, 2026", **Read the Contract text**, **Change the Draft Contract** (a non-money Contract action, ADR-0008). `GET .../contract` `200` with prose, merge fields and values and no `price` key |
| 5.2-a | `manual` | as expected, cell corrected | Camille Boyd's Engagement address: "Page not found". `GET` Camille's and Ines Calloway's Engagements and Contracts `404`; `POST` Ines's `.../contract/send` `404 engagement not found` |
| 5.3 | `manual` (was `missing-feature (LV-G3)`) | **walked** | Reese's Contract section: "Your fee $450.00, the fee you accepted on Oct 27, 2026". The terms she accepted are not on the Engagement ([LV-G14, #2269](https://github.com/markgoho/doula-cloud/issues/2269)) |
| 6.1 | `manual` | as expected, cell corrected | Reese's Birth Plan is not started. Lena saved Adaeze's (birth center; "Low light, my sister on the phone. Ask before any exam."): "Birth Plan saved.", "2 of 4 questions answered", "Last edited Oct 28, 2026 by Lena Vasquez", "Adaeze has not read this yet", **Change the Birth Plan**, **Print for the hospital** |
| 6.2 | `manual` | as expected, cell corrected | On Reese's, **Add a Visit**: "Scheduled date and time (optional)", **Save Visit**, **Cancel**. The date typed into the headless field did not take, so it saved "Not yet scheduled": "Your Visit is added." The row's **Change** set Nov 5, 2026, 2:00pm and notes: "The Visit for Lena Vasquez is saved." and the row "Nov 5, 2026, 2:00pm, Prenatal, Lena Vasquez, Second prenatal: …". **Change** is also on Joss Adeyemi's Visit; notes written there saved, and were cleared again |
| 6.3 | `manual` | as expected, cell corrected | Reese has no email: "Reese has no email address on file, so cannot be invited to the portal to read a message." On Adaeze's, before she accepted: "The thread opens once Adaeze has accepted the portal invite." After: **Write a message**, Message, Attachment, **Send**, **Cancel**; "Message sent." and "Lena Vasquez, Oct 28, 2026, 10:34pm" in the thread |
| 7.1 | `missing-feature (LV-G3)` [#2271](https://github.com/markgoho/doula-cloud/issues/2271) | as expected | Confirmed unwalkable. Her fee is recorded; nothing records that it was paid. [#225](https://github.com/markgoho/doula-cloud/issues/225) closed without building pay, and [#391](https://github.com/markgoho/doula-cloud/issues/391) ruled the payout out. Whether the product records one is filed as [#2271](https://github.com/markgoho/doula-cloud/issues/2271) |
| 8.1 | `manual` (was `missing-feature (LV-G4)`) | **walked** | Renata on Adaeze's: **Record what happened**, "The baby was born alive", Oct 28, 2026, **Record this outcome**. Then **Mark care complete**, "Why is this Engagement ending?", "The work finished as agreed", **Confirm completion**: "The care phase is now Completed." The Doula row went. On call still lists "Lena Vasquez, On call Jan 24, 2027 to Feb 28, 2027" ([LV-G15, #2270](https://github.com/markgoho/doula-cloud/issues/2270)) |
| 8.1-a | `manual` | **falsified**, cell corrected | Her open page reloaded to "Page not found". `GET .../clients` lists Reese Bishop only. **Your offers** still reads "Accepted, Fee $550.00" with no end ([LV-G15, #2270](https://github.com/markgoho/doula-cloud/issues/2270)) |

**Re-marked**: 1.3, 2.2-a, 3.1, 3.2, 3.3, 5.3 and 8.1 move from `missing-feature` to `manual`, because their gaps are built (LV-G1, LV-G4, LV-G6 and LV-G7 under ADR-0008, and the owed half of LV-G3). 7.B-screens moves from `manual` to `missing-feature (LV-G3)`, on Tasha Bell's TB-G6 precedent: its gap closed without a build, and there is no screen to measure. 7.1 stays `missing-feature (LV-G3)` and now cites [#2271](https://github.com/markgoho/doula-cloud/issues/2271). These were **falsified** and corrected: 2.2, 2.3, 2.3-a, 4.2, 4.2-a and 8.1-a. These were corrected without a mark change: 1.1, 1.2, 2.3-b, 4.1-a, 5.2, 5.2-a, 6.1, 6.2 and 6.3. The Header and Preconditions are rewritten to the built model. The stage-3 narrative on the journey map is left to [#1434](https://github.com/markgoho/doula-cloud/issues/1434). The Marks summary is recounted from 9 / 44 / 0 / 8 to 9 / 50 / 0 / 2, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted**: this plan's budget notes name the decisions only on stage 3 ("Accept or decline") and stage 6 ("when, which kind, what was covered, and the save"), and say nothing about groups. So this walk uses the rule from Renata Alvarez's, Dee Whitlock's and Priya Raman's walks: count fields, options, and links or buttons that move her somewhere or change what happens, if they are enabled and in view at once in the page body. A radio group, a select and a date-time field are one decision each, and **Cancel** counts. Not counted: **Back** links, the skip link, **Show password**, `/login`'s secondary links, a Privacy Policy link, and the Staff top bar and Practice switcher. **Time** is measured in the page: a script presses one control and polls each animation frame until the result has settled on screen. It records when the control showed it was busy, or for a move, when the page body first changed.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens | 2 | **3**: the account question, "Check your details", an Overview (Rooted's, not the Practice she joined) | **no**, [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), cited; built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) |
| 1.B-decisions | 2 | **4** on the account question (the radio group, Email address, Password, **Continue**); "Check your details" 2 (**Change where you work**, **Accept invite**) | **no**, [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), cited; built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) |
| 1.B-confirmation | yes | **no**. It lands on Rooted's Overview, which names Rooted and says nothing about Ridgeline, joining, the role or the employment type | **no**, [LV-G10 (#2265)](https://github.com/markgoho/doula-cloud/issues/2265); built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) |
| 1.B-cold | yes | yes, in a fresh context. The account question says who each answer is for ("You already sign in to DoulaCloud for another practice.") | yes |
| 2.B-screens | 2 | 2: `/login`, the Overview of her last-used Practice. Reaching the other Practice is the switcher on that Overview, not a screen | yes |
| 2.B-decisions | 3 | `/login` 3. The Overview's body **5** (four Quick links, **See all activity**), **7** with an Offer open | **no**, [RA-G11 (#2228)](https://github.com/markgoho/doula-cloud/issues/2228), cited |
| 2.B-memory | yes | yes. The switcher names each Practice, so she need not recall which is which. It does not say what she is at each ([LV-G11, #2266](https://github.com/markgoho/doula-cloud/issues/2266)); built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) | yes |
| 2.B-time | 400 ms, 100 ms | **Sign in** **410 ms** and **412 ms** in two fresh contexts, busy within 7 ms; the switcher opens in 2 ms | **no**, [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), cited |
| 3.B-screens | 1 | 1: the Overview holds the Offer, and **Your offers** holds it too | yes |
| 3.B-decisions | 2 | The Offer card 2 (**Accept**, **Decline**), and **Your offers** 2. But the Overview she meets it on holds **7** at once | **no**, [RA-G11 (#2228)](https://github.com/markgoho/doula-cloud/issues/2228), cited |
| 3.B-memory | yes | yes. Initial, area, due date, fee and terms are on the card she decides on | yes |
| 3.B-time | 400 ms, 100 ms | **Decline** 68 ms, **Accept** 75 ms, each busy within 2 ms | yes |
| 3.B-confirmation | yes | **no**. A decline removes the card with no message and no word that the Practice was told. An accept turns the card into "Accepted" with no Client, no Engagement and no link | **no**, [LV-G13 (#2268)](https://github.com/markgoho/doula-cloud/issues/2268) |
| 3.B-cold | yes | yes, in a fresh context from `/`: **Staff sign in**, **Sign in**, and the Offer is on the first screen with its two choices explained. Nothing told her to go and look ([LV-G12, #2267](https://github.com/markgoho/doula-cloud/issues/2267)); **built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264))**: an email now tells her, and its link opens **Your offers** at that Practice | yes |
| 4.B-screens | 1 | 1: Clients | yes |
| 4.B-memory | yes | yes. "Your fee" marks each of hers, and only hers are listed | yes |
| 4.B-time | 400 ms, 100 ms | **Clients** from the top bar 67 to 73 ms over three warm presses, the body changing within 33 ms | yes |
| 5.B-screens | 1 | 1: the Engagement | yes |
| 5.B-memory | yes | **no**. The fee she accepted is on the Engagement, and the terms are not. **Your offers** has the terms, but not which Client they were for | **no**, [LV-G14 (#2269)](https://github.com/markgoho/doula-cloud/issues/2269) |
| 6.B-screens | 1 | **2** to read the Birth Plan (the Engagement, then **Print for the hospital**). 1 for the Visit and the message | **no**, [DW-G14 (#2243)](https://github.com/markgoho/doula-cloud/issues/2243), cited |
| 6.B-decisions | 4 | 3 on **Add a Visit**; 4 on a Visit's **Change** (the date and time, Notes, **Save changes**, **Cancel**); 4 on **Write a message** (Message, Attachment, **Send**, **Cancel**) | yes |
| 6.B-time | 400 ms, 100 ms | **Add a Visit** 9 ms; **Save Visit** 46 ms; **Save changes** 155 ms; **Write a message** 4 ms; **Send** 38 ms; each busy within 5 ms. The Engagement from her Clients row 366 to 485 ms (stage 5's act, recorded here; acknowledged within 16 to 87 ms) | yes |
| 6.B-confirmation | yes | yes. "Your Visit is added.", "The Visit for Lena Vasquez is saved.", "Message sent.", each with the row or message showing | yes |
| 7.B-screens | 1 | Not measured: no screen exists | `missing-feature (LV-G3)` |
| 8.B-screens | 1 | 1: Renata's Engagement page holds the outcome and the completion, and Lena's Clients list shows the result | yes |
| 8.B-confirmation | yes | **no**. Renata reads "The care phase is now Completed." Lena reads "Page not found" on the address she had, and nothing says the job ended or when | **no**, [LV-G15 (#2270)](https://github.com/markgoho/doula-cloud/issues/2270) |

The 320px budget steps are `automated (route-continuum.svelte.spec.ts)` and were not re-measured. **Failed**: 1.B-screens, 1.B-decisions, 1.B-confirmation, 2.B-decisions, 2.B-time, 3.B-decisions, 3.B-confirmation, 5.B-memory, 6.B-screens, 8.B-confirmation.

#### Moment of truth — the verdict, against the first run's

**Stage 3, the offer, before she has said yes.** First run ([#238](https://github.com/markgoho/doula-cloud/issues/238), 2026-08-23): it had no screen at all. `offer` and `decline` appeared nowhere in the schema or the app, and every step of the stage was a hole. **This run: the moment of truth holds, and it is overturned from absent to working.** Renata offers the February birth from the Engagement in one form. Lena reads enough to take or refuse it while still an outsider: the Client's initial, the area, the due date, the fee and the on-call terms, with no name and nothing else of the agency's book. She declines in one press, and Renata reads "Declined" on the Engagement. A second Offer, accepted, puts her on the birth and gives her that Client and nothing more. It falls short of the budget in three places, and none of them stops her. Nothing tells her an Offer is waiting ([LV-G12](https://github.com/markgoho/doula-cloud/issues/2267)). The Overview she meets it on carries five other choices beside **Accept** and **Decline**, and prints the due date as `2027-02-14`. And neither answer confirms what it did ([LV-G13](https://github.com/markgoho/doula-cloud/issues/2268)).

#### Is the journey completable end to end?

**No. Step 7.1 stops it** ([LV-G3, #2271](https://github.com/markgoho/doula-cloud/issues/2271)). Against "a pass means":

- **"The Engagement she took is finished."** Yes. Renata completed Adaeze Nwosu's Engagement, and Lena's reach on it ended with it.
- **"She can point to what she agreed."** Yes. Fee and terms are on **Your offers**, and the fee is on the Engagement while it is open (the terms are not, [LV-G14](https://github.com/markgoho/doula-cloud/issues/2269)).
- **"…and what she was paid."** No. Nothing records that she was paid. [#225](https://github.com/markgoho/doula-cloud/issues/225) closed without it, and [#391](https://github.com/markgoho/doula-cloud/issues/391) ruled out the payout. Whether a record of one is built is now [#2271](https://github.com/markgoho/doula-cloud/issues/2271).
- **"She never saw a Client who was not hers."** Yes. Her Clients list, **See everyone**, a direct address and the API all held to the two Engagements she accepted, and the Billing and Stripe status reads refused her.

Every other stage is completable, and the first run's leaks, an outsider spending the agency's Credits and pricing its Clients' care, are gone.

**Findings filed**: [LV-G10 (#2265)](https://github.com/markgoho/doula-cloud/issues/2265), [LV-G11 (#2266)](https://github.com/markgoho/doula-cloud/issues/2266), [LV-G12 (#2267)](https://github.com/markgoho/doula-cloud/issues/2267), [LV-G13 (#2268)](https://github.com/markgoho/doula-cloud/issues/2268), [LV-G14 (#2269)](https://github.com/markgoho/doula-cloud/issues/2269) and [LV-G15 (#2270)](https://github.com/markgoho/doula-cloud/issues/2270), each on the [journey map](../journeys/contractor-doula.md#gaps-found), with its budget line quoted where a budget failed. Also filed: LV-G3's open half as [#2271](https://github.com/markgoho/doula-cloud/issues/2271), and two bugs, [#2272](https://github.com/markgoho/doula-cloud/issues/2272) (switching Practice keeps the previous Overview) and [#2273](https://github.com/markgoho/doula-cloud/issues/2273) (an empty Overview offers a contractor **Add your first Client**). Cited, with a comment on each: [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), [RA-G11 (#2228)](https://github.com/markgoho/doula-cloud/issues/2228), [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), [DW-G14 (#2243)](https://github.com/markgoho/doula-cloud/issues/2243), [#2230](https://github.com/markgoho/doula-cloud/issues/2230) and [#2236](https://github.com/markgoho/doula-cloud/issues/2236).

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 2, 3, 4, 5, 6, 7, 8 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **34 budget steps**, 8 `automated`, 26 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 2.1 | `staff-login.e2e.ts` | pass |

**1 automated steps: all pass.**

### 2026-09-10 — narrative reconciliation ([#685](https://github.com/markgoho/doula-cloud/issues/685))

A desk pass, not a walk, over this plan's Add Client, Visits and Contract cells — the ones [#318](https://github.com/markgoho/doula-cloud/issues/318) left alone. Nothing was re-walked and no mark moved.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 4.1-a | "Nothing refuses her" -> she is refused by name, twice over, and there is no credit left on that path to spend | `CreateHandler`'s `IsAmbientContractor` refusal in `api/internal/client/create.go`, the `clients_insert` RLS policy behind it, and ADR-0017 on when a Credit locks. [LV-G9](https://github.com/markgoho/doula-cloud/issues/292) is closed |
| 5.2 | "she gets the money, on every Engagement in the Practice, not only hers" -> she reaches only her own work, and the price is stripped for her there | ADR-0008 and `CanAccessEngagement` in `api/internal/staffauth/access.go` for the reach; `priceForReader` in `api/internal/contracts/contract.go` for the money |
| 5.2-a | "Nothing refuses her" -> she never reaches another Client's Engagement at all; [PR-G8](https://github.com/markgoho/doula-cloud/issues/282) survives as an inside-the-Practice finding | The same `CanAccessEngagement` check, and `api/internal/contracts/mount.go`, where the Contract write is still `AnyStaff` |
| 6.2 | "no date, no type, no note" -> the Visit row carries all three, and she may log only her own | The Visits table's Type / Date / Notes columns, and `resolveAssignee` in `api/internal/visit/roles.go`. [MO-G1](https://github.com/markgoho/doula-cloud/issues/250), [MO-G2](https://github.com/markgoho/doula-cloud/issues/251) and [PR-G6](https://github.com/markgoho/doula-cloud/issues/281) are closed |

**No step is re-marked.** Both new specs run as a founding Owner inside her own Practice, which is the one thing every cell above is not about.

**Left alone on purpose.** 4.2's LV-G5 reading — "one business reading another's book" — is contradicted by the same `CanAccessClient` rule, but it is a Clients-list step rather than one of the three this ticket reviewed, and re-walking it belongs to [#329](https://github.com/markgoho/doula-cloud/issues/329).

### 2026-08-23 — manual walk ([#238](https://github.com/markgoho/doula-cloud/issues/238))

`bun run dev:full` in `app/`, walked as Lena Vasquez at 1280x900. Her journey names no device, so the desktop default stands. This plan's one `automated` step was **not** re-run; `/login` was driven anyway, because 2.2 sits behind it.

Preconditions built as the plan requires, and in the order the plan requires — stage 1 is only a test while the bypass does not yet exist. **Willow Bend Doulas** (Beth Alvarado, Owner) invited Lena and she accepted, which is what put her `identity_uid` on a `staff` row and made her the person this plan is about; Beth then gave her `doula`. **Rooted Birth Collective** (Renata Alvarez, Owner) was signed up separately with two Clients — **Adaeze Nwosu**, the February birth offered to Lena, and **Tabitha Nunes**, who is not hers — a filled Birth Plan and Care Plan on Adaeze's Engagement, a Contract priced `$2,400` and `sent`, and Adaeze's portal invite accepted. Renata then invited Lena to Rooted, and that invitation **is** steps 1.1 and 1.2, so it was not provisioned away.

**The fixture bypass, and the answer the plan asked it for.** Postgres took the insert: `INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES (<rooted>, <lena's existing staff id>, '{doula}')` returned one row. `00002`'s own comment is **true of the schema and false of the API** — the table's only relevant constraint is `UNIQUE (practice_id, staff_id)`, which a second Practice does not touch. `roles` was the session's call, not the plan's: `{doula}` is what her Persona says she is, 2.3 expects a non-owner's tiles, and `visit/roles.go:40` would 403 her out of 6.2 without it. Recorded here rather than asked.

**Four steps were appended by the walk** — 2.3-a and 2.3-b (the cell says Billing is not her business; whether she can actually read it is the finding), 4.1-a and 5.2-a (both stages test her *reads* and neither tested what she can write). Dee's 7.2-a and Priya's 5.2-a are the precedent.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `/accept-invite?token=…` renders `Accept your Staff invite`, an Email box, a Password box, an **Account mode** radio pair and one **Accept invite** button. `scrollWidth` equals `clientWidth`. The tab title is empty ([DW-G8](https://github.com/markgoho/doula-cloud/issues/276)) |
| 1.2 | `manual` | as expected | **`409 POST /api/staff/accept-invite`**, printed on the screen as `a staff account already exists for this identity`. Exactly the refusal `accept.go:104` is written to give, from the unique violation on `accept.go:99`'s `UPDATE staff SET identity_uid` |
| 1.2-a | `missing-feature (LV-G2)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | **falsified, and it mints a gap** | A second membership **does** exist — `practice_memberships` carries a Rooted Birth Collective row for Lena with `roles = '{}'`, because `invite.go:56` and `:66` write the pending `staff` row and its membership at **invite** time, in a different request from the one that fails. The rollback of the failed accept cannot reach them. So the 409 is clean and the data is not: Renata's Staff screen prints **two `Lena Vasquez` rows on the same email**, one `no roles yet` and one `doula`, and the only control under **Actions** on either is **End sessions everywhere**. The phantom is indistinguishable from a real invitee who has not been given roles yet (Priya's acceptance leaves exactly that state), and no route in `api/main.go` deletes a membership or a `staff` row. New gap **[LV-G8](https://github.com/markgoho/doula-cloud/issues/291)**. LV-G2 stands: she still has no membership she can reach |
| 1.3 | `missing-feature (LV-G1)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable against the live table: `practice_memberships` is `id, practice_id, staff_id, roles, created_at` and nothing else. No column holds an employment type |
| 2.1 | `automated (staff-login.e2e.ts)` | not re-run | Already green in the 2026-08-22 suite run |
| 2.2 | `manual` | as expected — **and it is the first time this screen has ever rendered** | Signing in at `/login` lists **`Rooted Birth Collective`** and **`Willow Bend Doulas`** under `Choose a Practice`, each linking to its own `/practices/{id}`. Renata's 1.2, Dee's 1.3 and Priya's 3.2 each recorded "there is nothing to choose"; the picker was never missing, it needed two memberships, and only the bypass can produce them. One detail no plan names: it renders **on `/login` itself**, below the still-visible Email, Password and **Log in** controls, rather than on a page of its own |
| 2.2-a | `missing-feature (LV-G1)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. Two bare links, Practice name only. Nothing says what she is at either — not a role, and not the employment type that does not exist |
| 2.3 | `manual` | **falsified** | **Three** tiles, not two: `Clients`, `Billing`, **`Payments`**. Invite, Staff, Plan Templates and Contract Template are gone. Payments is outside the owner block — [RA-G9](https://github.com/markgoho/doula-cloud/issues/267), now seen from a fifth membership and the first one belonging to a person outside the business |
| 2.3-a | `manual` | **new** | `200 GET .../billing`. She reads `Credit balance: 1` and the whole ledger — `signup_bonus +3` and two `consumption -1` rows with timestamps. An outside contractor reads what the agency has bought and every time it has spent. **Buy credits** renders `disabled` ([DW-G4](https://github.com/markgoho/doula-cloud/issues/272)) |
| 2.3-b | `manual` | **new** | `200 GET .../payments/connect`. `Stripe Connect status: Not connected`, then `Ask a Practice Owner to connect Stripe.` and no button. The refusal is a hidden control, as Priya's PR-B5 found — but the *status* of another business's payment processing is handed to her before it ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)) |
| 3.1 | `missing-feature (LV-G6)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable, and confirmed absent rather than hidden: `offer` and `decline` appear nowhere in `api/db/migrations`, `api/internal` or `app/src` except as English in comments and test names. There is no screen to open |
| 3.2 | `missing-feature (LV-G7)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. `engagements` is `id, client_id, practice_id, status, created_at` — no staff column at all, so there is no *offered* state for a read rule to describe |
| 3.3 | `missing-feature (LV-G6)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. Nothing records a decline, so silence and "no" remain the same thing to the Practice |
| 4.1 | `manual` | as expected | `200 GET .../clients`. The list renders for a non-owner |
| 4.2 | `manual` | as expected | **Two rows, both of them Rooted Birth Collective's: `Adaeze Nwosu` and `Tabitha Nunes`.** One is hers. [LV-G5](https://github.com/markgoho/doula-cloud/issues/225) confirmed against a Practice built to test it: one business reading another's book |
| 4.2-a | `manual` | as expected | The table is two columns, `Name` and `Status`. Nothing marks which is hers; picking it out means remembering the name ([RA-G4](https://github.com/markgoho/doula-cloud/issues/225)) |
| 4.1-a | `manual` | **new — and the worst result on this plan** | Lena, holding `doula` at a Practice she does not belong to, opened **Add a Client**, entered `Wanda Peel`, pressed **Add Client** and got `201`, landing on a new Engagement. **The agency's credit balance went 1 -> 0** and a `consumption -1` row appeared in Renata's ledger. `POST .../clients` is mounted behind `staffauth.Middleware` alone (`api/main.go:169`) and `engagement/create.go:97` consumes the credit inside the same transaction, so an outsider spends the Practice's money and no role check stands anywhere between the two. New gap **[LV-G9](https://github.com/markgoho/doula-cloud/issues/292)** |
| 5.1 | `manual` | as expected | Adaeze's Engagement opened on the first try. Seven `GET`s, all `200`: the Engagement, Visits, Messages, both plans, the Contract and the Invoices. No read path is role-checked |
| 5.2 | `manual` | as expected | She gets the money: `Price` reads `$2,400`, and scope, both dates and both names are filled. [PR-G2](https://github.com/markgoho/doula-cloud/issues/277) confirmed from its other half — ADR-0006 says Lena *should* read this on her own work and Priya should not, and the read cannot tell them apart |
| 5.2-a | `manual` | **new** | On **Tabitha Nunes** — a Client of the agency who is not hers — Lena pressed **Create Draft Contract** (`201`), set `scope_of_service` to `Lena wrote this` and `price` to `$99` (`200 PUT`), and **sent it to the Client** (`200 POST .../contract/send`). [PR-G8](https://github.com/markgoho/doula-cloud/issues/282) confirmed and escalated: Priya's version is an employee writing money inside her own Practice; this is an outside business putting a priced agreement in front of another business's Client |
| 5.3 | `missing-feature (LV-G3)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. Nothing on the page holds her rate. The `Price` field is the Client's fee, and the Invoices section bills the Client |
| 6.1 | `manual` | as expected, and it corroborates two of Priya's | `200 GET .../plans/birth_plan` renders the snapshot — `Birth center`, `Partner`/`Doula`/`Midwife` ticked, `Low light, my sister on the phone. Ask before any exam.`, photos consented. The **Birth Plan** heading sits at y=567 of a 1445px page at 1280x900, and the page renders **0** `<a>` elements, so there is no deep link and no way out ([PR-G5](https://github.com/markgoho/doula-cloud/issues/280), [PR-G9](https://github.com/markgoho/doula-cloud/issues/283)) |
| 6.2 | `manual` | as expected | **Add a Visit** -> `201`, and the Visits table gained `Lena Vasquez`, `8/23/2026`. The `Date` column is `created_at` ([MO-G1](https://github.com/markgoho/doula-cloud/issues/250)); no type and no note anywhere ([MO-G2](https://github.com/markgoho/doula-cloud/issues/251), [PR-G6](https://github.com/markgoho/doula-cloud/issues/281)). The only other control on the row is `Reassign to Staff id` ([RA-G10](https://github.com/markgoho/doula-cloud/issues/268)) |
| 6.3 | `manual` | as expected | `201 POST .../messages`, appended to the one Engagement thread as `Lena Vasquez (staff) — 8/23/2026, 9:12:13 AM`. Nothing about it differs for a contractor — which is the finding stage 6 was written to get |
| 7.1 | `missing-feature (LV-G3)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable against the live schema. `invoices` is `(practice_id, contract_id, stripe_invoice_id, status, amount_cents, currency, …)` — a Practice billing a Client. None of the database's twenty tables records a Practice owing a doula |
| 8.1 | `missing-feature (LV-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected, and the cell understated it | Confirmed unwalkable, and the workaround the cell names does not exist either: `api/main.go` mounts no route that removes a membership or a `staff` row. The one lever an Owner has on the Staff screen is **End sessions everywhere** (`DELETE .../staff/{staffId}/sessions`), which ends a sign-in, not a read. Renata pulled it: `204` |
| 8.1-a | `manual` | as expected | Lena signed in again immediately afterwards and read `Adaeze Nwosu` and `Tabitha Nunes`, `200`. After the job is done, and after the only lever the product gives an Owner, an outside contractor still reads the agency's whole book |

**17 `manual` steps walked (2.3-a, 2.3-b, 4.1-a and 5.2-a added by the walk); 9 `missing-feature` steps confirmed unwalkable; no `blocked` step on this plan.** Three expected results were falsified — 1.2-a, 2.3 and 8.1 — and none was re-marked: 2.3 is a performable step whose claim was wrong, and 1.2-a and 8.1 are still unwalkable, only for a sharper reason than the cell gave ([#235](https://github.com/markgoho/doula-cloud/issues/235)'s precedent both times). Two gaps minted on the journey map that owns the stages, **LV-G8** and **LV-G9**. No `journey-gap` issue was filed — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means": it does not pass, and it fails on all three clauses.** The Engagement she took cannot be finished — no status moves and nothing ends an attachment. She can point to what she *agreed* only by reading the Client's price, and to what she was *paid* not at all. And she saw every Client the agency has, before and after the job, plus its credit ledger and its Stripe status. Stage 3, her moment of truth, has no screen in it at all: she never weighed the job, because the product cannot offer one. What the walk adds to the map's argument is the direction of the leak — the map wrote her as someone the product tells too much, and 4.1-a and 5.2-a show it also lets her *do* too much, spending the agency's credits and pricing its Clients' care.

### 2026-09-20 — narrative reconciliation ([#1242](https://github.com/markgoho/doula-cloud/issues/1242))

A desk pass, not a walk, over this plan's stage-1 cells and Preconditions: LV-G2 and its adjacent gap LV-G8 both closed since the 2026-08-23 walk above, and the plan still told the old story. Nothing was re-walked and the dated entries above are untouched.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 1.2 | "Expected failure: ... a 409, 'a staff account already exists for this identity'" -> `staffauth`'s accept path resolves her existing `staff` row and succeeds with a `200` | `resolveStaff` in `api/internal/staffauth/accept.go:306`; [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) closed |
| 1.2-a | "None she can reach ... LV-G8" -> the second membership exists, inserted by the same accept call, and there is no pending `staff` row left over to leave a phantom member behind; re-marked `missing-feature (LV-G2)` -> `manual` | The `practice_memberships` insert at `accept.go:226`; `InviteHandler` no longer writes a pending `staff` row at invite time (`invite.go:54`); [LV-G8](https://github.com/markgoho/doula-cloud/issues/291) closed |
| Preconditions | "Fixture bypass for stages 2-8 ... Stage 1 cannot produce her second membership" -> no bypass is needed; step 1.2 produces the membership for real | Same as 1.2 above |

**One mark moves: 1.2-a, from `missing-feature (LV-G2)` to `manual`.** The Marks summary above is recounted with it, to 1 / 18 / 0 / 8, and `docs/test-plans/README.md`'s run-status row and Total move the same way.

**Left alone on purpose.** LV-G1's employment-type claim on step 1.3, LV-G4, LV-G6, LV-G7 and RA-G8's roles-on-an-invitation claim are the same shape of drift, spotted while rewriting the same stage-1 narrative, but reading each against the code closely enough to correct it is a wider job than this ticket's own scope. Held at [#1434](https://github.com/markgoho/doula-cloud/issues/1434).

