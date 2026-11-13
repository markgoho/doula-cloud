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

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
