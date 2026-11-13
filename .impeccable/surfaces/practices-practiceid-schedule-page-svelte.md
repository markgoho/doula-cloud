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
- **No total yet.** The BFF answers a page and `hasMore`, never a count of the whole range. The category baseline below asks for the count (#2867).
- **The way in is the Client's name, to the Engagement.** A Visit has no page of its own; Reassign lives on the Engagement.
- **The Visit has no type, place, length or status on this endpoint** (#330, #428). The screen does not invent them.

## Category baseline

Decided 2026-11-13 (#2866). Where the conventions were learned: Stripe Dashboard's payments, invoices and customers lists; Linear's issue lists and Display menu; the GOV.UK Design System; the MOJ Design System's sortable table and "filter a list" patterns; Google Calendar's Schedule view; Calendly's and Acuity's upcoming lists. Only MOJ's sortable-table page was read on 2026-11-13; the rest is from memory. The test for a sortable column: a column is a sort when the reader wants the extremes of a range; it is a filter or a search when the reader wants every row of one value. An order the BFF does not serve is never drawn by sorting loaded rows in the browser: the list loads 30 rows at a time from a cursor.

| Convention | Verdict | Reason |
| --- | --- | --- |
| Filters, with the view in the address | Have it | the rubric's H3 |
| Empty, loading and failed states | Have it | the rubric's H1, H9, T8 |
| A deep link to a row | Have it | the Engagement at `#visit-<id>` |
| Paging | Have it | "Load more"; the result line counts the rows shown and says whether more exist |
| Bulk selection and bulk actions | Declined | the rubric's H5: nothing on a list changes money or a Visit in one press; no bulk job on record |
| Density and column choice | Declined | no job on record; each control is a Tab stop against T7's budget |
| Keyboard shortcuts | Declined | no job on record; T7 is met by Tab and "Skip to results" |
| A list-level export, print or copy | Declined | Settings exports the whole Practice; no job for a list's CSV |
| Row actions without opening the record | Declined | H5; the Engagement holds Reassign, one action from the row |
| Sort by When (latest first) | Declined | soonest first serves T1 and T5; no job on record reads past Visits newest first; a chosen past range reads in time order, as a calendar does |
| Sort by Doula | Declined | every row of one Doula is a narrowing; the Doula select and "Mine" do it (T2, T4) |
| Sort or search by Client | Declined | the Clients list's Name search and the Engagement's Visits do it; a search here would be a POST kept by history, as on the siblings (#1829) |
| Date navigation | Have it | "Next 7 days", "Next 30 days", "Choose days"; previous and next week declined: "Choose days" does it within the Tab budget |
| Grouping by day, with a today marker | Build (#2867) | the category's schedule view; T5 and `personas/employed-doula.md:17` |
| Relative days ("Today", "Tomorrow") | Build, in the day heading (#2867) | the cell keeps `VisitWhen` so the Schedule and the Engagement read the same; ADR-0022's "relative under seven days" is the ledger's rule for past entries |
| Grouping by person | Declined | the Doula select does it |
| A week or month calendar beside the list | Later (#330) | a Visit has no length or end time, so no grid can be drawn |
| A total count | Build (#2867) | the result line says "the first 30 Visits" and no total, where the Invoices list says "the first 30 of 35"; the BFF answers a page and whether more exist, so the count of the days chosen is a new figure on the read. Declining it because the endpoint lacks it would be declining it because it is work |
| Time zone shown | Declined | one Practice's Staff share a zone; times are the reader's own; the 7 and 30 day windows are the reader's days |
| Search by name or reference | Declined | as "Sort or search by Client" |

If an order is ever built: a `FilterBar` control that exists at every width with its value in the address, the ordering column's `th` mirroring it with `aria-sort`, and the BFF serving it with a cursor tied to that order. A header-only button vanishes in the record cards at 320px (ADR-0024).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
