---
version: 1
slug: "practices-practiceid-schedule-page-svelte"
primary_target: "app/src/routes/practices/[practiceId]/schedule/ScheduleScreen.svelte"
related_targets: ["app/src/routes/practices/[practiceId]/schedule/+page.svelte","app/src/routes/practices/[practiceId]/schedule/+page.ts","app/src/lib/visitSchedule.ts","app/src/lib/components/organisms/DataTable.svelte","app/src/lib/components/molecules/FilterBar.svelte","app/src/lib/components/molecules/DateRangeFields.svelte","app/src/lib/components/atoms/Badge.svelte","app/src/lib/styles/tokens.css"]
---

Scope: the Staff app, third surface, the Practice-wide Schedule (#2747). Mode: Operate. Audience and jobs: `.impeccable/rubrics/schedule.md` (Renata, Dee, Priya, Maya, keyboard, screen reader, phone). Grading: that rubric (40) and `.impeccable/rubrics/craft.md` (20), three blind graders, median.

## Direction contract

THESIS: The category standard, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services (ADR-0063): the familiar arrangement with nothing out of place. The Schedule is the Clients list's and the Invoices list's sibling and reads as the same product: same title, filter bar, result line and table, with days in place of care or money.

OWN-WORLD: The Staff world as `DESIGN.md` records it. One blue for links and the primary action. A day is said the way a person says it, with its weekday; tabular figures for every day and time. One filled badge, amber, only on a booked Visit whose Doula is no longer on its Engagement, because that row needs someone to act (One Meaning Per Hue); every other row is plain words.

STORY: An owner or office manager opens Schedule, sees the coming days and who holds each Visit, narrows to one Doula or one week in two actions, opens the Visit that has to move, and comes back to the same place. A Doula on a phone sees where she has to be next on the first screen.

FIRST VIEWPORT: Top bar with the nav; title "Schedule"; the filter bar above the result line; the list starting in the first screen at 1280px and 375px.

FORM: The category standard (canon). No signature component: a list is not a record (ADR-0063).

## Where this screen differs from the climbed lists, and why

- **The order is time, and the weekday leads.** The Clients and Invoices lists are books a reader narrows; the Schedule is read forward from now, and "her Thursday" is the unit (`journeys/practice-owner.md:120`). A Visit's day reads as ADR-0022's 2026-10-24 amendment writes it ("Thu Sep 4, 2:00pm"), here and on the Engagement, by one formatter in `app/src/lib/dates.ts`. Rows stay rows: `DataTable` has no group row and every row is the same height (DESIGN.md).
- **A three-column list keeps its content measure.** Its table is narrower than the siblings' five-column tables, and the Doula track is held so the table is the same width in every filter state (DESIGN.md, Data table: a table stops where its columns are satisfied).
- **The narrowing always has a range.** A bare address means "Next 30 days", today and the 29 days after it (`visitSchedule.ts`), so "no filter" on this screen is the default range, and the way back from an empty result is to that range, not to "everything".
- **No Client's name is filtered here.** The address carries days and a Staff id only, so #1829 does not bind; nothing is added that would put a name in a URL.
- **No totals.** The BFF answers a page and `hasMore`, never a count of the whole range.
- **The way in is the Client's name, to the Engagement.** A Visit has no page of its own; Reassign lives on the Engagement.
- **The Visit has no type, place, length or status on this endpoint** (#330, #428). The screen does not invent them.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
