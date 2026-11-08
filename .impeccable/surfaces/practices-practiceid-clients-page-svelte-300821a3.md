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

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
