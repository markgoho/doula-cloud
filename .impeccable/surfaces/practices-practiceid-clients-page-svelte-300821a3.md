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

Decided 2026-11-13 (#2866); the sort verdict was reversed the same day (#2869). Read on 2026-11-13: Linear's display options page (https://linear.app/docs/display-options) says "Ordering is allowed in most list views and lets you order issues", by status, priority, last created, last updated, due date and more, and "You can also reverse the sort order", beside its filters and grouping. Stripe's dashboard search page (https://docs.stripe.com/dashboard/search) says list views show "column headings, some of which provide sorting options". The MOJ Design System's sortable table (https://design-patterns.service.justice.gov.uk/components/sortable-table/) says "Use this component when sorting is needed to help find data within a large table of data", combined with the filter pattern. GOV.UK's table component has no sort. Still unread: Stripe's own Payments, Invoices and Customers lists' default order and column editor; Google Calendar's Schedule view grouping events under a heading per day (its help page says only "To find a list of all your events or tasks by day, choose Schedule"); Calendly's and Acuity's lists. An order the BFF does not serve is never drawn by sorting loaded rows in the browser: the list loads 30 rows at a time from a cursor.

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
| A reader-chosen order (soonest due first; Name, A to Z or Z to A) | Build (#2869) | Linear and Stripe give the reader a chosen order beside the filters; soonest due first stays the default, stated in the result line |
| Sort by Money or by Doula | Declined | every Client who owes, or every Client of one Doula, is a narrowing; "Who owe money" and the Doula select do it (T1, T3) |
| Search by name | Have it | Name (T5; #1829) |
| Grouping by Doula | Declined | the Doula column and the Doula select |
| A total count | Have it | while more exist the result line says "Showing the first 30 of 58 Clients", the first page's count of the narrowing, from the same predicate as the rows, so a contractor's total is her own (#2867); an owner who "can no longer hold it in her head" (`personas/practice-owner.md:11`) is owed the whole |
| Date navigation, a calendar view, relative days | Declined | a roster is not read by day; the Date column says weeks to or since the birth |
| Time zone shown | Declined | days only, no times |

The order is built in #2869: one control at every width with its value in the address, the ordering column's `th` saying it with `aria-sort`, and the BFF serving each order with a cursor tied to it. A header-only button vanishes in the record cards at 320px (ADR-0024).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
