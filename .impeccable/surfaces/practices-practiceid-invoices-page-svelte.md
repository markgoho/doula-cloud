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

Decided 2026-11-13 (#2866); the sort verdict was reversed the same day (#2869). Read on 2026-11-13: Linear's display options page (https://linear.app/docs/display-options) says "Ordering is allowed in most list views and lets you order issues", by status, priority, last created, last updated, due date and more, and "You can also reverse the sort order", beside its filters and grouping. Stripe's dashboard search page (https://docs.stripe.com/dashboard/search) says list views show "column headings, some of which provide sorting options". The MOJ Design System's sortable table (https://design-patterns.service.justice.gov.uk/components/sortable-table/) says "Use this component when sorting is needed to help find data within a large table of data", combined with the filter pattern. GOV.UK's table component has no sort. Still unread: Stripe's own Payments, Invoices and Customers lists' default order and column editor; Google Calendar's Schedule view grouping events under a heading per day (its help page says only "To find a list of all your events or tasks by day, choose Schedule"); Calendly's and Acuity's lists. An order the BFF does not serve is never drawn by sorting loaded rows in the browser: the list loads 30 rows at a time from a cursor.

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
| A reader-chosen order (sent, payment due, amount; each either way) | Build (#2869) | Linear and Stripe give the reader a chosen order beside the filters; each view's own order stays its default ("All" newest first, "Unpaid" earliest due first, "Overdue" longest overdue first) |
| The order said to assistive technology (`aria-sort`) | Build (#2867) | the view orders by a column a screen reader is not told |
| Search by reference or Client | Have it | "Reference or Client" (T4; #1829) |
| A total count | Have it | the four totals, and the whole in the result line while more exist ("the first 30 of 35") |
| Date navigation, grouping, a calendar view, relative days | Declined | a book of Invoices is not read by day; the two days in a row are facts of the row |
| Time zone shown | Declined | days only, no times |

The order is built in #2869: one control at every width with its value in the address, the ordering column's `th` saying it with `aria-sort`, and the BFF serving each order with a cursor tied to it. A header-only button vanishes in the record cards at 320px (ADR-0024).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
