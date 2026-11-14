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

- **The order is time, and the weekday leads.** The Clients and Invoices lists are books a reader narrows; the Schedule is read forward from now, and "her Thursday" is the unit (`journeys/practice-owner.md:120`). A Visit's day reads as ADR-0022's 2026-10-24 amendment writes it ("Thu Sep 4, 2:00pm"), here and on the Engagement, by one formatter in `app/src/lib/dates.ts`. Rows stay rows: `DataTable` groups them under a day's heading row and adds no other kind, and every row, the heading included, is the row height (DESIGN.md).
- **A three-column list keeps its content measure.** Its table is narrower than the siblings' five-column tables, and the Doula track is held so the table is the same width in every filter state (DESIGN.md, Data table: a table stops where its columns are satisfied).
- **The narrowing always has a range.** A bare address means "Next 30 days", today and the 29 days after it (`visitSchedule.ts`), so "no filter" on this screen is the default range, and the way back from an empty result is to that range, not to "everything".
- **A Client's name is searched here, and #1829 binds.** The address carries days, a Staff id and the order only; the typed name rides in the history entry's state and the BFF gets it in a POST body (`POST /visits/list`), as on the Clients and Invoices lists, so a name is in no URL, request log or Referer. A reload or a shared link drops it.
- **The total comes with the first page.** The BFF counts the Visits of the days and the Doula chosen from the same predicate as the rows, so a contractor's total is her own.
- **The way in is the Client's name, to the Engagement.** A Visit has no page of its own; Reassign lives on the Engagement.
- **The Visit has no place, length or status** (#330, #428), and the screen does not invent them. Its type is derived by the BFF and is the one thing about it the row says (#2870).

## Category baseline

Decided 2026-11-13 (#2866); the sort verdict was reversed the same day (#2869). Read on 2026-11-13: Linear's display options page (https://linear.app/docs/display-options) says "Ordering is allowed in most list views and lets you order issues", by status, priority, last created, last updated, due date and more, and "You can also reverse the sort order", beside its filters and grouping. Stripe's dashboard search page (https://docs.stripe.com/dashboard/search) says list views show "column headings, some of which provide sorting options". The MOJ Design System's sortable table (https://design-patterns.service.justice.gov.uk/components/sortable-table/) says "Use this component when sorting is needed to help find data within a large table of data", combined with the filter pattern. GOV.UK's table component has no sort. Still unread: Stripe's own Payments, Invoices and Customers lists' default order and column editor; Google Calendar's Schedule view grouping events under a heading per day (its help page says only "To find a list of all your events or tasks by day, choose Schedule"); Calendly's and Acuity's lists. An order the BFF does not serve is never drawn by sorting loaded rows in the browser: the list loads 30 rows at a time from a cursor.

The declines were read on 2026-11-13 by an Opus agent and by Gemini; the rows they rejected are builds with their tickets, and a decline they accepted for a weak reason carries the better one.

Round 4's graders (2026-11-13) each wrote what they expected before reading this table; the rows they named that it lacked were added the same day.

| Convention | Verdict | Reason |
| --- | --- | --- |
| Filters, with the view in the address | Have it | the rubric's H3 |
| Empty, loading and failed states | Have it | the rubric's H1, H9, T8 |
| A deep link to a row | Have it | the Engagement at `#visit-<id>` |
| Paging | Have it | "Load more"; the result line counts the rows shown and says whether more exist |
| Bulk selection and bulk actions | Later | "Priya is ill" for a week is several Visits to move (`journeys/practice-owner.md:120`); it follows a row-level Reassign, which is declined below, so it waits with it |
| Density and column choice | Declined | four columns leave nothing to choose |
| Keyboard shortcuts | Later | one decision for the whole app, not one per list; Dee is "the fastest typist in the Practice" (`personas/non-doula-admin.md:13`) |
| A list-level export, print or copy | Later (#2871) | decided with the Invoices list's export (#2871); two of three graders found "no record of a printed cover sheet" weak for an office manager who prints a day's sheet |
| Row actions without opening the record | Declined | Reassign needs the Engagement's own facts (who is attached, who may take the Visit), and the row opens it at that Visit with Reassign in view; one reading asked for Reassign on the row, the other accepted the decline |
| A reader-chosen order (When: soonest or latest first) | Have it (#2869) | Linear and Stripe give the reader a chosen order beside the filters; soonest first stays the default; built 2026-11-13, an Order select in the bar and `aria-sort` on When |
| Sort by Doula or by Client | Declined | a schedule is read in time, and ordering by person scatters each day; the Doula select, "Mine" and the Client search (#2870) find one person's Visits |
| Search by Client | Have it (#2870) | a "Client" field, first in the fields row, with a tag and a "Look for … a year back and a year ahead" way on from an empty result; both readings rejected "another screen does it": the Invoices list has its own "Reference or Client" search though the Engagement holds the Invoices too; Dee starts every call with a name (`personas/non-doula-admin.md:13`, `journeys/non-doula-admin.md:84`) |
| Date navigation: the three choices | Have it | "Next 7 days", "Next 30 days", "Choose days" |
| Date navigation: previous and next | Have it (#2870) | "Earlier" and "Later" step the window by its own length (built 2026-11-13); the first decline ("Choose days does it within the Tab budget") was circular, the budget being the rubric's own line; Google Calendar's help says "Swipe left or right to go to other months" and "tap Jump to today" (https://support.google.com/calendar/answer/6110849) |
| Grouping by day, with a today marker | Have it | a heading row per day over its Visits, "Today, Fri Nov 13" marking today (#2867); T5 and `personas/employed-doula.md:17` |
| Relative days ("Today", "Tomorrow") | Have it | "Today" and "Tomorrow" lead the day heading (#2867); the cell keeps `VisitWhen` so the Schedule and the Engagement read the same; ADR-0022's "relative under seven days" is the ledger's rule for past entries |
| Grouping by person | Declined | the rows are bookings; who is free depends on births, which are not Visits (#330); the Doula select shows one Doula's |
| A month calendar beside the list | Declined (#2888) | the screen's jobs are a day or a week read in time (`.impeccable/rubrics/schedule.md` T1–T4), and any one date is one action away through Choose days or Later; a month grid at 320px gives a cell about 40px wide, which cannot hold a Client, a time and a Doula per Visit, and a Doula reads this screen on a phone in a hospital corridor (`personas/employed-doula.md:13`), so the grid would be a second view for the desk only (ADR-0024 makes every screen complete from 320px); Calendly's host view of scheduled events is a list grouped by day, the shape kept here; a month's question about room, "Priya has room in March" (`journeys/non-doula-admin.md:89`, Dee putting a Doula on an Engagement), is about due dates, which the Clients list's Due narrowing answers once #2871 is built |
| A week calendar beside the list | Later (#330) | a week grid places a Visit by its start and its length, and a Visit has no length yet (#330) |
| A whole-row click target | Have it (#2888) | a click anywhere on a row or card opens its Engagement, as Stripe's lists do; the row keeps its one link for the keyboard and a screen reader; a click while text is selected or after a drag, and a double-click, open nothing extra, so a name can still be selected and copied; a modified or middle click opens the same link in a new tab, as Stripe and Linear readers expect; the row's own controls are not part of the target |
| A total count | Have it | while more exist the result line says "the first 30 of 42 Visits", the count of the days and Doula chosen, which the first page carries (#2867) |
| Time zone shown | Have it | days and times are the Practice's (ADR-0036), and a reader in another zone is told so once under the result line (#2867) |
| Search by name or reference | Have it (#2870) | as "Search by Client" |
| Clear all filters | Have it | "Clear filters" beside two or more tags, or beside one tag and a chosen order; one tag's × clears it; a chosen order draws no tag, so "Clear filters" is how it is undone with the fold closed |
| A next step from an empty result | Have it | "Show the next 30 days", "Show every Doula's Visits" |
| A saved or default view | Declined | the address is the saved view; Linear saves "personal preferences or … the default" (https://linear.app/docs/display-options) |
| Filters kept across visits | Declined | the address and the history keep the view |
| A count beside each filter option | Declined | each count is another read, and GOV.UK's filters show none |
| Column resize | Declined | it fights the intrinsic layout (ADR-0024) |
| A calendar feed (.ics) | Later (#330) | with the calendar work |
| Last read at, and refresh | Later | decided with the offline store (ADR-0059), where a book read without signal must say when it was read |
| Inline status change | Declined | a Visit has no status (#330) |
| The Visit's type on the row (prenatal, birth, postpartum) | Have it (#2870) | a Type column beside the When in the table and "Prenatal · Priya Raman" on the card, in the Engagement's words from the BFF's one derivation; the Engagement's Visits showed it and this list's endpoint did not send it; Priya asked "is it a prenatal or the birth?" (walked 2026-11-13); all three graders of round 4 named it |
| A Visit's length and place on the row | Later (#330) | the model holds neither |
| A narrowing to the Visits whose Doula is no longer on the work, with their count | Have it (#2863, #2888) | a "Doula no longer on this work" box in "Show only" for an Owner or Admin (`?needsDoula=true`), in the badge's own words, a tag, and the count on the result line ("2 have a Doula no longer on this work."); four blind walks as Dee reached for it |
| A reader's own Visits first ("Mine" on already for a Doula) | Have it (#2870) | decided by its design call for this list and the Clients list together; Priya, both walks: "This is everyone's"; Linear opens a member on "My issues" (unread) |
| Filter by the Visit's type | Have it (#2888) | a "Type" select ("Prenatal", "Birth", "Postpartum") in the fold with the Doula, `?type=`, a "Type: Birth" tag, and the type in the one WHERE the rows and the count read; Acuity's appointment type and Calendly's event type taught it (round 5, all three graders) |
| Filters that take effect when changed | Have it (#2888) | a box, a radio and a select apply when changed, after a 250ms settle, with focus kept on the control and the result line announcing the list; typed fields (the Client, the days) still wait for Apply or Enter, and Apply stays drawn for them and for no script; all three graders and both walks of round 5 waited on an untyped control; recorded in DESIGN.md's Filter bar entry and `govuk-alignment.md`'s filter-bar row |
| Booking a Visit from the Schedule | Later (#330) | a Visit is added on its Engagement, where its Client and its Doulas are; booking from a calendar comes with #330's calendar work; all three graders named it |
| A Doula picker that takes several | Declined | no job on record asks for two Doulas' Visits at once, and "Any Doula" with the day headings shows everyone's; two graders named it |
| A way to past Visits | Have it (#2870) | "Earlier" steps back a window at a time; "Choose days" reaches any day |
| The filter bar on a phone | Have it (#2870) | below a 42rem bar "Show only", the Doula select and Order fold under "More filters and order"; Days, Earlier, Later, the Client search and Apply stay in view; the first screen holds a card or two instead of the bar |

### Round 5's smaller notes (#2888)

Each was raised by one grader or one walk. Each is fixed or declined with its reason here.

| Note | Verdict | Reason |
| --- | --- | --- |
| "Needing a new Doula" (the box) and "No longer on this work" (the badge) name one condition twice | Fixed (#2907) | the box, the tag and the result line take the badge's words, "Doula no longer on this work", which H2 holds fixed; `?needsDoula=true` stays, because an address does not need to read as copy |
| An empty search's "Look for … in the next 12 months" looks forward only | Fixed (#2907) | the step looks a year each way ("Look for "Camille" a year back and a year ahead"), today − 365 to today + 364 in the Practice's zone, and the result line states the days |
| An empty search leaves about 50px blank under its tag | Fixed (#2901) | `FilterBar` draws no ruler while the list is empty, on all three lists; the gap from the tag to the empty state went from 65px to 45px at 375×740 |
| "Earlier" from the opening 30 days steps 30 days, where Priya wanted last week | Declined | T10's job is to move the days shown by their own length, and a 7-day step out of a 30-day window would land on days the reader was never shown; since the radios apply when changed, last week is two taps ("Next 7 days", then "Earlier"), and so is the week after |
| A search carries into Earlier and Later | Declined | a Client who calls about "my visit last week" (T9 with T10, Dee's call) needs the search kept through the step, as #2870's E6 decided; the empty state names what was looked for, the "Client: …" tag is in view, and its × is one action to every Visit in those days |
| In the table the comma after the day sits about 20px from the time | Declined | the When cell is read by H2 (ADR-0022's form, comma included), H4 (one shared formatter with the Engagement), H6 (weekdays at one x), H8, T2, T4/T5 (the card runs it on as one phrase), T6, craft C4 (weekdays at one x and clocks on one edge, measured) and DESIGN.md's "A Visit's When"; closing the gap means a day track sized to each day or a right-aligned day, and either fails H6 and C4; hiding the comma failed H2 and H4 in round 2 (#2747's retro, point 5). A column is read down, not across |

The order is built in #2869: one control at every width with its value in the address, the ordering column's `th` saying it with `aria-sort`, and the BFF serving each order with a cursor tied to it. A header-only button vanishes in the record cards at 320px (ADR-0024).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
