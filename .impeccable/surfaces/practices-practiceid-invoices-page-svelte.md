---
version: 1
slug: "practices-practiceid-invoices-page-svelte"
primary_target: "app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte"
related_targets: ["app/src/routes/practices/[practiceId]/invoices/+page.svelte","app/src/lib/invoice.ts","app/src/lib/components/organisms/DataTable.svelte","app/src/lib/components/molecules/FilterBar.svelte","app/src/lib/components/molecules/SummaryList.svelte","app/src/lib/components/atoms/Badge.svelte","app/src/lib/styles/tokens.css"]
---

Scope: the Staff app, second surface, the Practice-wide Invoices list (#2746). Mode: Operate. Audience and jobs: `.impeccable/rubrics/invoices-list.md` (Renata, Dee, Maya, keyboard, screen reader, phone). Grading: that rubric (40) and `.impeccable/rubrics/craft.md` (20), three blind graders, median.

## Direction contract

THESIS: The category standard, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services (ADR-0063): the familiar arrangement with nothing out of place. The Invoices list is the Clients list's sibling and reads as the same product: same title, filter bar, result line and table, with money in place of care.

OWN-WORLD: The Staff world as `DESIGN.md` records it. One blue for links and the primary action. Filled state badges only where a state needs chasing: amber is owed, red is overdue with its days and written off (failed, DESIGN.md's Signal Red); payment on its way, paid, void and refunded are plain words on a list. The Money cell is the Clients list's (`MoneyAmount`), so a paid row says "$1,500.00 paid" there, once, and its Payment cell says only the day (DESIGN.md, "Money is said once"). Tabular figures for every amount and date; amounts end on one edge.

STORY: An owner or office manager opens Invoices, reads what Clients owe and how much of it is late before reading a row, narrows to the Invoices to chase in two actions, opens the one a check came in for, and trusts the totals because every row under them agrees.

FIRST VIEWPORT: Top bar with the nav; title "Invoices"; what Clients owe and what is overdue, stated compactly; the filter bar above the result line; the list starting in the first screen at 1280px and 375px.

FORM: The category standard (canon). No signature component: a list is not a record (ADR-0063).

## Category baseline

Decided 2026-11-13 (#2866). Where the conventions were learned: Stripe Dashboard's payments, invoices and customers lists; Linear's issue lists and Display menu; the GOV.UK Design System; the MOJ Design System's sortable table and "filter a list" patterns; Google Calendar's Schedule view; Calendly's and Acuity's upcoming lists. Only MOJ's sortable-table page was read on 2026-11-13; the rest is from memory. The test for a sortable column: a column is a sort when the reader wants the extremes of a range; it is a filter or a search when the reader wants every row of one value. An order the BFF does not serve is never drawn by sorting loaded rows in the browser: the list loads 30 rows at a time from a cursor.

| Convention | Verdict | Reason |
| --- | --- | --- |
| Filters, with the view in the address | Have it | the rubric's H3 |
| Empty, loading and failed states | Have it | the rubric's H1, H9, T8 |
| A deep link to a row | Have it | the Engagement at `#invoice-<id>` |
| Paging | Have it | "Load more"; the result line counts the rows shown and, while more exist, the whole ("the first 30 of 35") |
| Bulk selection and bulk actions | Declined | the rubric's H5: nothing on a list changes money or a Visit in one press; no bulk job on record |
| Density and column choice | Declined | no job on record; each control is a Tab stop against T7's budget |
| Keyboard shortcuts | Declined | no job on record; T7 is met by Tab and "Skip to results" |
| A list-level export, print or copy | Declined | Settings exports the whole Practice; no job for a list's CSV |
| Row actions without opening the record | Declined | H5; the Engagement holds Record payment, one action from the row |
| Sort by Sent | Have it | "All" is newest first |
| Sort by Payment due | Have it | "Unpaid" orders by earliest payment due first and "Overdue" by longest overdue first (T2) |
| Sort by amount | Declined | no job on record chases the largest first; "Unpaid" and the totals answer how much |
| The order said to assistive technology (`aria-sort`) | Build (#2867) | the view orders by a column a screen reader is not told |
| Search by reference or Client | Have it | "Reference or Client" (T4; #1829) |
| A total count | Have it | the four totals, and the whole in the result line while more exist ("the first 30 of 35") |
| Date navigation, grouping, a calendar view, relative days | Declined | a book of Invoices is not read by day; the two days in a row are facts of the row |
| Time zone shown | Declined | days only, no times |

If an order is ever built: a `FilterBar` control that exists at every width with its value in the address, the ordering column's `th` mirroring it with `aria-sort`, and the BFF serving it with a cursor tied to that order. A header-only button vanishes in the record cards at 320px (ADR-0024).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
