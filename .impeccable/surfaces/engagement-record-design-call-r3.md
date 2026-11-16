# Engagement record (#2755): design call before round 3, 2026-11-15

Round 2: 38/40, craft 19/20 (lost: 8c 5512 > 5400; T3 at 375 under 7 for B and C; C4 composition, 3/3). One Opus agent, from the three round-2 reports, their screenshots, `build/1440-full.jpeg`, acts-priya r2 and the source at f4cacd20. It amends the r2 call; nothing reopens ADR-0018's 2026-11-07 amendment (no rail, tabs, sticky). Heights are **estimates** from the round-2 measures (arithmetic shown); the build reports the real ones.

## E1. One right edge for structure, the measure for running text (C4, the founder's complaint)

Measured at round 2: main track 903px at 1280 (x 32–935), 1024 at 1440 (32–1056); facts 975–1235 / 1102–1374; prose, SummaryLists and Notices stop at 647 / 689 (`--measure`, ~615 / 657px). The empty band is 935 − 647 = **~290px** at 1280 and 1056 − 689 = **~370px** (to the facts, ~410) at 1440, through every SummaryList on the first screen.

**Decision: rows take the main column; reading text keeps the measure; the grid is unchanged.** A key/value row (key, value, action) spans the main column as Stripe's detail rows and GOV.UK's full-width summary list do: key `30%`, value the rest, action `max-content` at the column's right edge, the value's own text capped at `--measure` (it never reaches it: ~510px at 1440). Prose (state lines, the window line, Care Plan text, Notices, WarningText, `EmptyState`, the Message thread, every in-place form) stays at `--measure`. Tables keep "shrink to their columns" (Visits and Invoices exceed the column, so they fill it; Activity stays at its 661px max-content).

Applies to (all `SummaryList` today, verified): On call's days and gap rows (`EngagementScreen.svelte:3949`, `:4035`), Contract's facts (`molecules/ContractStatus.svelte`), and the Birth Plan's and Care Plan's answered-only view (`molecules/PlanInstanceView.svelte:124`, `isStacked`). The cap is `SummaryList`'s own (`dl { max-inline-size: var(--measure) }`, `:158`, #2021), which is why rows end at 678–693, past the prose's 647. Its row is already GOV.UK's (key `flex: 1 0 30%`, value basis 20rem, action `margin-inline-start: auto`), so `width="column"` only drops the `dl` cap. `PlanInstanceView` stacks on purpose (#2021: a question is a sentence that ribbons in a 30% key); with `width="column"` it un-stacks only where its key column is ≥ 16rem (a ≥ 53rem container, by container query), where "Preferences for atmosphere (music, lighting, etc.)" takes 2 lines beside a 2-line answer; below that, and on every phone, stacked as today (T1 unchanged).

**Details in the aside (grader A's second C4 failure):** at 15–17rem `SummaryList` stacks the key, the value takes the line and "Change" falls alone to the far edge ("Care" / "Birth" / "Change"). The aside copy draws its rows in the facts' own `dl` shape: muted label above, value and its action on the value's line (`cluster-l`, `--space-3`): "Care" / "Birth  Change". Email: the address on its own line (`overflow-wrap: anywhere`), no action. The one-column copy keeps `SummaryList`.

Edges after the build (deliberately two):

| Width | Rows, Visits, Invoices right edge | Prose / thread / Notice / Activity right edge | Facts and Details |
|---|---|---|---|
| 1280 | 935 (one edge, = main track) | ~647 (Activity 693) | 975–1235 |
| 1440 | 1056 | ~689 (Activity 693) | 1102–1374 |

The band is left only beside prose a reader reads at a measure (a 2-line state line, the thread); every bordered row ends 40–46px from the facts, so the main column and the facts read as one composition.

**The facts get the GOV.UK related-items treatment, not a box**: a `--border-active` top rule in `--color-border` and `--space-3` above the first label, on the facts and on the Details aside. No fill, no radius (8g: no box). The rule's top is the facts' top (4b hold-still).

Rejected, with the numbers: **facts moved in to the measure's edge, tables beneath** (record ~930/975px wide): Visits is the first section before on-call and postpartum, Invoices on a completed record, so a table lands beside a ~900px facts + Details column on most phases and either drops ~600px below its own state line or turns to cards; any CSS that avoids that (float with a 55rem table floor, grid row spans) is content-dependent. **Narrower main track** (55rem at 1440): moves the band from 370 to ~220px and squeezes Visits further under its 1208px max-content. **Framed panel alone**: frames the band, does not remove it.

Files: `molecules/SummaryList.svelte` gains `width="column"` (drops the `dl` cap; consumers: the Engagement record, the Client record #2754 named second, ADR-0018's bar); `templates/RecordDetail.svelte` replaces the "every direct child at the measure" selector (`:779`) with the measure on the text blocks listed, the facts/aside top rule; `organisms/PlanInstanceView` (or its answered-only view) the ≥ 40rem row layout; `EngagementScreen.svelte` passes `width="column"` on the four places. Docs: ADR-0018 dated amendment (the width rule now reads "rows and tables take the column, running text the measure"; corrects the 2026-11-13 amendment's sentence for a record); DESIGN.md's width rule; `govuk-alignment.md`'s Summary list row ("caps its rows at the reading width" becomes true only outside a record).

## E2. Visits: the last Visit in the state line, Earlier before Not scheduled yet, Change into Notes (T3, 6d on Mira)

- State line: "Next: Wed Nov 18, 11:00am, Priya Raman." then, on the same line where it fits, the link **"Last Visit: Sun Nov 1"** (`#visit-<id>`, the existing `landOnVisitId` path, which opens the Earlier cap). No next: "No Visit scheduled. Last Visit: Oct 12." No logged Visit: the link is absent.
- Groups: **Coming up, Earlier, Not scheduled yet** (what is next, what happened, what has no day). The Earlier cap (newest 1) stays.
- **Change on an Earlier Visit puts focus in Notes, caret at the end**; the form keeps its heading as its label. A Coming-up Visit's Change keeps focus on the heading (its first job is the day). Cancel still returns focus to that row's Change.
- Budget at 375×740: read = Jump to → Visits (1), "Last Visit" (2), notes on screen; write = + Change (3), typing. Build reports notes top ≥ 0 and bottom ≤ 740 after action 2, and the focused element after 3 is the Notes `textarea`.
- Mira (6d, B's #2): her Visits holds a logged Visit, (`demo-practice.ts:425–434`), so it is not empty and the loss order stands; what read empty was "No Visit scheduled." alone, which the Last Visit link answers. No order change. Unverified, for the orchestrator: that a follow-up Visit after a loss is care Add a Visit should lead to; `docs/personas/loss-client.md` has no line on it (`:32` names Visit reminders as a hazard, which this adds none of).

## E3. Height: ≥ 200px of real waste at 1440×900 (8c)

| Cut | Est. | Why it is waste |
|---|---|---|
| Birth Plan answers question-beside-answer (E1) | −80 | 3 of 4 answers lose their question line (the atmosphere question is 2 lines beside a 2-line answer); nothing hidden |
| Invoices: the "All of this Contract is billed…" Notice folds into the sentence, "Of the $1,500.00 Contract price, all is billed, so there is nothing left to invoice." | −75 | the same fact said twice (sentence + Notice) |
| Visits: When, Type and Doula one line (`columnWidths="one-line"`, Notes the `grows` column) | −54 | "Priya / Raman" wraps on two rows today at 1024px |
| Invoices: Actions cell on one line (`white-space: nowrap` on the actions run; Note grows) | −50 | C: "Record payment Void / Write off" wraps with room left |
| Messages: "Show earlier Messages" on the state line's line (`cluster-l`, as Visits' Add a Visit) | −45 | a control alone on a line |
| **Total** | **≈ −304 → ≈ 5210** (band ±150) | Offers/Birth outcome (shared `EmptyState`) untouched; no section behind a new disclosure |

E1 changes no other height: the Care Plan is capped (15rem), and On call's and Contract's rows already hold key, value and Change on one line at 646px. Hold: each section ≤ its r2 height; 3d's Back to top threshold (2700) still applies.

## E4. The Client's phone: a fact, last in the facts, `tel:` (4e, row 27)

- Rejected in the header: a `tel:` link beside "Message the Client" is a fourth decision on the phone's first screen (8h, 3) and an extra Tab stop before Jump to (T9's count); in the status line it breaks ADR-0018's "the status line holds no control".
- **Facts gain "Phone", after Messages** (the two ways to reach her together): value `(585) 555-0142` as a `tel:` link named "(585) 555-0142, Phone, call"; none on file: **"Not given"**, a link to `#details` named "Not given, Phone, in Details" that focuses "Add Camille's details"; the Client read refused: "Could not be read", no link (T10's words). Erased: "Not given", no link.
- `factsRows` always holds the Phone row, so nothing moves at 375 when the Client read lands (1a).
- Details drops its Phone row (said once); keeps Email.
- Seed: **yes**, Camille gets a phone (`app/scripts/demo-practice.ts`, a 555-01xx number, through the Client write the details journey uses); Grace keeps none, so both states are gradable.
- Records this changes, amended with it: ADR-0018's 2026-11-15 amendment says the facts "answer five questions with a link each" (now six; the sixth link calls rather than navigates, so "no navigation Jump to does not hold" still holds); rubric T8's "each at-a-glance link names its fact and section" gains "or, for Phone, its fact and the act". At a glance stays T2's eight (6a); Phone is not one.
- Measures: 1280 Messages fact bottom ≤ 800 with E1's top rule (est. ~724); Phone bottom reported, est. ~780 (not a 6a fact, so past 800 is not a failure); 375 On call tonight fact unmoved (±4).

## E5. Notes settled

- **1a, On call h2 moves 34px**: "Jump to" draws every link from the loading frame. The build finds which link lands late (B: 376→419ms) and derives it from the Engagement read and the reader's role; if one truly needs a later read, the strip reserves its two-line block size from the 73rem container. Measure: On call h2 top at first paint and after the last read, Δ ≤ 4px at 1280 (no throttle and Slow 4G).
- **T8, Details' h2 first in heading order**: the aside's title becomes the region's label, not a heading (`<aside aria-labelledby>` on a `<p>` styled as today's card heading). Heading order is then h1, the sections' h2s, in drawn order; the one-column copy keeps its h2 (it is last). An aside form keeps its h3 (a level skip only while open; not a T8 claim).
- **3b, no Cancel on the payment check step**: `RecordPaymentForm.svelte:176-216` gains a secondary "Cancel" on step `check`, nothing recorded, focus back to the row's "Record payment" (as step `form`'s).
- **6d, Mira**: E2.
- **C2, 17.9 vs 17.17px**: `--text-body-size` is `cqi`; `DataTable` re-resolves it against its own frame on purpose (`DataTable.svelte:788–795`, #544, so a record card sizes to its frame), so the record's `dd` (record body, ~1216px) and Visits' `td` (frame, 903px) differ by 0.7px. Changing it is app-wide (every list and card): **file it** against DESIGN.md's type ramp, as r2 filed the h1/h2 ramp; not built here (1 of 3 graders).
- **Invoice actions wrap**: E3.
- **"Write a message"**: opens the compose with focus in the Message `textarea` (as the header's move does). Cancel (and Send) return focus to the opener that opened it, the header's "Message the Client" or "Write a message"; the compose records its opener.

## Measurements the build reports

| Measure | R2 | Target |
|---|---|---|
| 1440×900 page height, Camille; each section | 5512 | ≤ 5400 (est. ~5210); each ≤ r2 |
| Right edges at 1280 / 1440: On call rows, Contract rows, Birth Plan rows, Visits, Invoices | 678 / 693, tables 935 / 1056 | 935 / 1056 (all) |
| Right edges: state line, thread, Notice, Activity | 647–693 | ≤ measure (unchanged) |
| Facts top = Jump to top; On call h2 top; first days row bottom (1280) | 382 = 382; 483; ~715 | equal; ±4; ≤ 780 |
| On call h2 Δ during load (1280) | 34 | ≤ 4 |
| 375 after Jump to → Visits → Last Visit: notes top / bottom; focus after Change | 1445–1569 | ≥ 0 / ≤ 740; Notes textarea |
| Heading order (snapshot, 1280 and 375) | Details before On call | h1, sections' h2s in drawn order |
| Phone: `a[href^="tel:"]` on Camille; Grace "Not given"; 1280 bottom | 0 | 1; present; ≤ 800 |
| Payment check step: Cancel present, nothing recorded, focus | none | Record payment |
| Compose: focus on open from each opener; focus after Cancel | heading; "Write a message" | textarea; the opener |
| Hold-still: 8a, 8b, 8h (3), 8i, T1 (624/740), T9 stops (4), 320 scroll 0, console 0, 2a find 0 | — | unchanged |

## Specs and e2e that change

`SummaryList` spec (`width="column"`), `RecordDetail` spec (measure selector, aside label not a heading, Jump to loading links), `PlanInstanceView` spec (row layout ≥ 40rem), `RecordPaymentForm` spec (check-step Cancel), Details aside rows (facts `dl` shape), `MessageThread`/compose spec (focus textarea, return to opener), `engagement-detail.svelte.spec.ts` (Visits Last Visit link and group order, Change into Notes on Earlier, Phone fact three states, Details without Phone, Invoices sentence without the Notice, Show earlier Messages placement), `on-call-section.svelte.spec.ts` (rows `width="column"`). `app/e2e`: grep "All of this Contract is billed", "Not scheduled yet", "Write a message", `details-aside-heading`, `getByRole('heading', { name: 'Details' })`. Seed: `demo-practice.ts` Camille's phone. Docs: ADR-0018 amendment (SummaryList `width="column"`, the record's width rule, aside label, six facts), DESIGN.md:255 ("each region caps itself: prose at `--measure`" gains "a record's key/value rows take the main column") and the facts entry near :301 (top rule), `govuk-alignment.md` Summary list row. Filed, not built: C2's container-relative body size. Rubric (orchestrator, dated): T8 Phone link name (E4); 4a "SummaryList at most `--measure`" → "a key/value row spans the main column, its value text at most `--measure`"; T3's budget unchanged.

## Wireframes

```
1280×800   x: 32                         647        935  975              1235
← Clients
Camille Boyd  [Message the Client]
Care phase Active  Due Dec 5, 2026, 37 weeks  Doulas …  [⚠ Nobody on call tonight]
Jump to On call  Birth Plan  Visits  Messages  Care Plan  … |══════════════════|  ← rule, y 382
        Activity  Details                                   | On call tonight  |
On call                                          (h2 483)   | Nobody           |
Nobody on call tonight: Priya Raman cannot be…  |measure    | Next Visit       |
On call from Sat Nov 14 to Sat Dec 19, from 37 weeks.       | Wed Nov 18, 11am |
Priya Raman    On call Sat Nov 14 to Fri Dec 4      Change |  | Contract Signed  |
──────────────────────────────────────────────────────────  | Owed $750, Nov 29|
Bethany Kroll  On call Sat Dec 5 to Sat Dec 19      Change |  | Messages ⚠ Unans.|
──────────────────────────────────────────────────────────  | Phone (585)…0142 |
When a Doula cannot be reached                              |══ Details ═══════|
Priya Raman    Sun Nov 15, 6pm–Mon 10am [Nobody covering]   | Care Birth Change|
               At a wedding four hours away   Change Clear  | Email, Portal …  |
Birth Plan  4 of 6 answered · …                             | Change the due … |
Pain relief    No epidural unless I ask twice.   (rows to 935)
Visits  Next: Wed Nov 18, 11:00am, Priya Raman. Last Visit: Sun Nov 1  [Add a Visit]
| When | Type | Doula | Notes (grows) | Actions |     table 32–935
  Coming up / Earlier / Not scheduled yet
```

```
375×740 (after Jump to → Visits → "Last Visit: Sun Nov 1")
Earlier
┌───────────────────────────────┐   ← landed, top ≥ 0
│ Sun Nov 1, 3:00pm             │
│ Type Prenatal · Doula Priya R.│
│ Notes  Third prenatal visit.  │
│ Walked the way to the hospital│   bottom ≤ 740
│ Change  Reassign              │   Change → focus in Notes
└───────────────────────────────┘
Not scheduled yet …

375×740 first screen (unchanged order): ← Clients · Camille Boyd · [Message the Client]
· phase / due / Doulas · Jump to (4 lines) · facts: On call tonight (y≈574) … Messages,
Phone (just below the fold; was ~9000px down in Details)
```
