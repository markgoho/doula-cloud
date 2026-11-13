---
version: 1
slug: "practices-practiceid-clients-page-svelte-300821a3"
primary_target: "app/src/routes/practices/[practiceId]/clients/+page.svelte"
related_targets: ["app/src/lib/styles/tokens.css","app/src/lib/components/organisms/DataTable.svelte","app/src/lib/components/atoms/Badge.svelte"]
---

Scope: the Staff app, first surface the Clients list. Mode: Operate. Audience and jobs: `.impeccable/rubrics/clients-list.md` (Renata, Priya, keyboard, screen reader, phone). Grading: that rubric (40) and `.impeccable/rubrics/craft.md` (20), three blind graders, median.

## Direction contract

THESIS: The category standard, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services: the familiar arrangement with nothing out of place. It refuses both the costume world and the half-finished default (gray screen, one accent, loose spacing).

OWN-WORLD: White working ground and a near-black ink in light; a deep blue-gray ground in dark (3am scene). One blue for links and the primary action, nothing else. Neutral hairlines, a tinted table header, filled state badges (amber owes, red overdue, blue pending) that mean one thing app-wide. Hanken Grotesk at a small, strict scale; tabular figures for every date and amount.

STORY: A doula or owner opens Clients, knows where everything is before noticing the look, narrows to the job in two actions, and trusts the list because every row, badge and number lines up.

FIRST VIEWPORT: Top bar with the nav; title "Clients" with "Find or add a Client" as a start button beside it; the filter bar (checkboxes, Name, Doula, Apply) above the result line; the table (Name, Status, Due, Doula) starting in the first screen at 1280px and 375px.

FORM: The category standard (canon), taken over two dealt hands; signature component candidate: the care timeline, kept only if it beats a plain Due column on the rubric. seed:73d6e1a1be987328

## Category baseline

Decided 2026-11-13 (#2866). Where the conventions were learned: Stripe Dashboard's payments, invoices and customers lists; Linear's issue lists and Display menu; the GOV.UK Design System; the MOJ Design System's sortable table and "filter a list" patterns; Google Calendar's Schedule view; Calendly's and Acuity's upcoming lists. Only MOJ's sortable-table page was read on 2026-11-13; the rest is from memory. The test for a sortable column: a column is a sort when the reader wants the extremes of a range; it is a filter or a search when the reader wants every row of one value. An order the BFF does not serve is never drawn by sorting loaded rows in the browser: the list loads 30 rows at a time from a cursor.

| Convention | Verdict | Reason |
| --- | --- | --- |
| Filters, with the view in the address | Have it | the rubric's H3 |
| Empty, loading and failed states | Have it | the rubric's H1, H9, T8 |
| A deep link to a row | Have it | the Client's page |
| Paging | Have it | "Load more"; the result line counts the rows shown and says whether more exist |
| Bulk selection and bulk actions | Declined | the rubric's H5: nothing on a list changes money or a Visit in one press; no bulk job on record |
| Density and column choice | Declined | no job on record; each control is a Tab stop against T7's budget |
| Keyboard shortcuts | Declined | no job on record; T7 is met by Tab and "Skip to results" |
| A list-level export, print or copy | Declined | Settings exports the whole Practice; no job for a list's CSV |
| Row actions without opening the record | Declined | H5; the Client's page and its Engagement hold the actions, one step from the row |
| Sort by due date | Have it | the default order, soonest due first, stated in the result line |
| Sort by Name, A to Z | Declined | finding one name is T5 and the Name search does it; the closest call of the three: the first question for a real owner in research (`docs/research/`) |
| Sort by Money or by Doula | Declined | every Client who owes, or every Client of one Doula, is a narrowing; "Who owe money" and the Doula select do it (T1, T3) |
| Search by name | Have it | Name (T5; #1829) |
| Grouping by Doula | Declined | the Doula column and the Doula select |
| A count | Have it, rows shown only | the result line counts the rows shown and says whether more exist ("Showing the first 30 Clients"); it states no whole-list total, and no verdict has been recorded on one |
| Date navigation, a calendar view, relative days | Declined | a roster is not read by day; the Date column says weeks to or since the birth |
| Time zone shown | Declined | days only, no times |

If an order is ever built: a `FilterBar` control that exists at every width with its value in the address, the ordering column's `th` mirroring it with `aria-sort`, and the BFF serving it with a cursor tied to that order. A header-only button vanishes in the record cards at 320px (ADR-0024).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
