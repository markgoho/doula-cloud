# Engagement record (#2755): design call, 2026-11-15

The arrangement the climb builds, decided after the baseline round by one Opus agent (grading-round.md, "Before the baseline" step 6), with the orchestrator's corrections after S1–S3. Nothing here reopens ADR-0018's 2026-11-07 amendment ("Jump to" under the title, no rail, tabs or sticky). Founder-facing decisions are recorded on #2755 (comment of 2026-11-15) and the order on #2146.

## 1. Arrangement (built in S1–S3)

- `RecordDetail` gains `facts` (Snippet), `factsRows` (its loading placeholder), `statusLine` (Snippet). With `facts` present it draws a `record-body` grid: one column narrow; from a 73rem container `minmax(55rem, 64rem) | minmax(15rem, 17rem)`, facts beside the main column, not sticky. DOM order Jump to → facts → sections. Without `facts`, every other consumer is unchanged.
- "Jump to": no box, no chips; a muted label and one wrapping list of plain links (`id="jump-to"`, `tabindex="-1"`), 24px targets.
- Header: name; moves "Message the Client" (to `#messages`) and "Add a Visit" (to `#add-visit`, opens the add form) as one list of links styled as secondary buttons (`outline` Link variant); status line: Care phase · Due with weeks · Doula(s) or "No Doula yet" · warning "Nobody on call tonight" (not after a loss or completion).
- Facts column: On call tonight → `#on-call`, Next Visit → `#visits`, Contract → `#contract`, Owed → `#invoices` (absent for a reader served no money), Messages → `#messages`. Each value a link with a hidden "label, in Section" suffix; a refused read says "Could not be read" with no link; a state is said once (icon, not icon plus Badge).
- **Correction for S4 (amended after S4b):** each fact stacks its label above its value (muted small label, ink value) where the facts' own container is narrower than 20rem (the wide layout's column); at full phone width the label sits beside the value, because stacking there pushed the first screen from 735 to 864 against the 740 target; Next Visit reads day and time on line one ("Wed Nov 18, 11:00am") and the Doula on a muted second line.
- The old summary card is the section "Details", last before Activity: Care, Due, Care phase, Doulas (Change opens put on / take off), Portal invite (row action "Send portal invite"), Created, the Client's Phone (`tel:`) and Email (`mailto:`); then "View Client" and "Add details".

### Width rule (rubric 4a)

Prose, `SummaryList`, Notice, EmptyState, the Message thread and the plan views are capped at `max-inline-size: var(--measure)`. Each `DataTable` is `inline-size: fit-content` (never wider than the main column). Nothing else spans the column.

## 2. Section order by care phase (built in S3, recorded on #2146)

See `sectionOrder.ts` and the comment on #2146.

## 3. Sections: first line, empty form, controls (S4)

Each section is an h2, then a **state line** (`Text`, body, at `--measure`), then content. An empty section is the h2 plus one line, its add control on the next line (rubric 8e).

- **On call.** State: "Nobody on call tonight." (warning icon) or "Priya Raman is on call tonight." Then "On call Nov 21 to Dec 19, from 37 weeks." Empty: "On call starts at 37 weeks, on Nov 21." The days and gaps lists stay. Every form opener is secondary; "Add a gap" is primary only inside its open form.
- **Birth outcome.** Empty: "Recorded after the birth." + secondary "Record what happened". Filled: the outcome and its day.
- **Contract.** State: the Contract's words plus the day and who sent it: "Signed on Oct 2, 2026." / "Sent on Nov 3 by Renata Alvarez, waiting for the Client to sign." Then the facts `SummaryList`, "Download signed Contract (PDF)" (secondary), "Read the Contract text" (details), Request void (a contractor's). **Last subsection:** h3 "Void this Contract", `WarningText`, a **secondary** Button "Void this Contract"; the `ConfirmDialog`'s confirm is the only red fill (`ContractStatus.svelte:448`). Empty: "No Contract yet." + "Create the Contract" (or the rate block).
- **Visits.** State: "Next: Thu, Nov 19, 10:00am, Priya Raman." or "No Visit scheduled." Table at fit-content, grouped by `DataTable` `rowGroup` (its second consumer, after the Schedule) with groups "Coming up" (soonest first), "Not scheduled yet", "Earlier" (newest first). Columns When, Type, Doula, Notes (wraps). Row actions Change and Reassign (Owner/Admin) as link buttons. No opener under the table (the header's "Add a Visit" is it). Empty: "No Visits yet." + "Add the first Visit".
- **Birth Plan.** State: "4 of 6 questions answered · last changed Nov 12 by Priya Raman · Camille Boyd has not read it yet" (or "read it on Nov 13"), one line in place of the 3-row list. Then the **answered questions only**, in template order (`PlanInstanceView` with an `answeredOnly` option), inside a capped block (start at `max-block-size: 32rem`, `overflow: hidden`, a fade on the last line; the full text stays in the accessibility tree). Under it: Link "Read or print the whole Birth Plan" to `birth-plan/` (replaces "Print for the hospital"), then secondary "Change the Birth Plan". Camille's pain-relief answer ("No epidural unless I ask twice.") is her last answer; the build measures its bottom after Jump to → Birth Plan at 375×740 and raises the cap until it is on screen. The birth-plan page's back link returns to `#birth-plan`. Empty: "Not started yet." + "Start the Birth Plan". Postpartum or after an outcome: the existing `NO_BIRTH_PLAN_*` line.
- **Care Plan.** The same state line and capped answered-only block; under it secondary "Show the whole Care Plan" (`aria-expanded`, removes the cap in place), then "Change the Care Plan". Empty: "Not started yet." + "Start the Care Plan".
- **Invoices.** State: "$750.00 owed, due Nov 29." / "$400.00 overdue since Nov 1." (red, DESIGN.md:213) / "Nothing owed." For a contractor: "1 Invoice open, due Nov 29." with no money. Then one line in place of the 3-row billing list: "Of the $1,500.00 Contract price, $1,500.00 is billed and $0.00 is not billed yet." Then the table, which **must stay a table inside the main column at 1280 and 1440**: columns Invoice, Sent, Amount, Status (paid / refunded / paid by as a muted second line in the Status cell), Note only if a row has one, and the row actions. Then "Create Invoice". Empty before a Contract: "An Invoice is raised against a signed Contract."
- **Offers / Doula fees.** Unchanged inside; each gets a state line ("1 Offer open, sent Nov 10." / "Nothing paid yet.").
- **Messages.** State: "Unanswered since 9:22am today." (warning icon) or "Answered.", then `Latest: "…"`, one line in place of the 2-row list. Thread at `--measure`, oldest at the top, newest at the bottom beside the compose; the newest 5 shown, "Show earlier Messages" in place (then the existing load-older). Compose behind "Write a message". Empty: "No Messages yet." + "Write a message".
- **Activity.** A kind filter above the table: "Show: All · Care and Visits · Contract and money · Doulas and on call", each re-reading page 1 from the BFF with `?kind=care|money|doulas` (never filtering loaded rows), the current one `aria-current="true"`. Table When / What / Who, newest first, fit-content, every When a `<time datetime>`, relative under 7 days (ADR-0022). "Load more" in place keeps the kind. Empty: "Nothing has happened yet." (filtered: "Nothing of this kind yet.").
- **Back to top** (rubric 3d): after each section taller than 900px at 1440×900 and at the page's end; a link to `#jump-to` that moves focus to the Jump to list.
- **Failures** (rubric T10, 9a): every "Failed to …" message on the screen becomes Voice-rule words ("We could not load the Visits.") with "Try again" that re-reads that section; the matching fact says "Could not be read".

## 4. Words that change

| Where | Today | New |
|---|---|---|
| Birth Plan | "Print for the hospital"; Progress / Last edited / Client rows | "Read or print the whole Birth Plan"; one state line |
| Care Plan | — | "Show the whole Care Plan" |
| Visits | "Add a Visit" under the table | removed; empty: "Add the first Visit"; groups "Coming up", "Not scheduled yet", "Earlier" |
| Invoices | Contract price / Billed / Not yet billed rows | "Of the $X Contract price, $Y is billed and $Z is not billed yet." |
| Messages | Latest / Unanswered rows | "Unanswered since …" / "Answered." + `Latest: "…"`; "Show earlier Messages" |
| Activity | — | "Show:" "All", "Care and Visits", "Contract and money", "Doulas and on call"; "Nothing of this kind yet." |
| Contract void | red "Void this Contract" | secondary "Void this Contract"; red on the dialog's confirm only |
| Back to top | — | "Back to top" |

No third-person pronouns; names stand in ("Camille Boyd has not read it yet").

## 6. Measurements each build reports

| Measure | Baseline | Target |
|---|---|---|
| 1440 page height (Camille) | 6846 (8128 after S3) | ≤ 3600 (rubric 8c), every section still on the page |
| 1280: first section's h2 / state line / first row bottom | On call 1078 | ≤ 800 |
| Widths at 1440 | Visits 1265, Invoices 1215, Notice 1359, Messages 1359 | tables ≤ max-content + 32 and still `<table>`; the rest ≤ --measure |
| 375 (Priya): header + facts bottom | — | ≤ 740 with all five facts, targets ≥ 24px, text not shrunk |
| 375 after Jump to → Birth Plan: pain-relief answer bottom | full plan | ≤ 740 |
| 320: horizontal scroll | 0 | 0 |
| Console errors | — | 0 |

## 8. Build steps

- S1 template and header — 15e9541a. S2 facts and Details, S3 order — b12e2675.
- **S4a:** facts stacking and the Jump-to gap; Birth Plan and Care Plan; Visits groups; Invoices (state line, sentence, narrowed table).
- **S4b:** Messages; Contract void; Activity kind filter; On call / Offers / Doula fees / Birth outcome state lines; Back to top; failures with Try again; the width rule everywhere.
- **S5 (with S4b):** `DESIGN.md` (destructive Button rule, facts column, Jump to look, `outline` Link variant), `docs/design/govuk-alignment.md` (Button row, record contents/facts rows), ADR-0018 dated amendment (2026-11-15, #2755: `facts`, `factsRows`, `statusLine`, Jump to restyle, `outline` variant, `rowGroup`'s second consumer, each with consumers per ADR-0058; the facts column is content, not the contents rail).
