---
version: 1
slug: "practices-practiceid-on-call-page-svelte"
primary_target: "app/src/routes/practices/[practiceId]/on-call/OnCallScreen.svelte"
related_targets: ["app/src/routes/practices/[practiceId]/on-call/+page.svelte","app/src/routes/practices/[practiceId]/on-call/+page.ts","app/src/routes/practices/[practiceId]/settings/on-call/+page.svelte","app/src/lib/onCall.ts","api/internal/oncall/roster.go","api/internal/oncall/settings.go","app/src/lib/components/organisms/DataTable.svelte","app/src/lib/components/molecules/FilterBar.svelte","app/src/lib/components/molecules/DateRangeFields.svelte","app/src/lib/components/templates/FormPage.svelte","app/src/lib/components/atoms/Badge.svelte","app/src/lib/styles/tokens.css"]
---

Scope: the Staff app, fourth surface, On call (#2748): the roster (`on-call/`) and the rule that makes it (`settings/on-call/`). One brief and one rubric for both, because they are one task for the person using them (the issue's own grouping); a heuristic's checks read both routes, which makes the score stricter. Mode: Operate. Audience and jobs: `.impeccable/rubrics/on-call.md` (Renata, Dee, Priya, Lena, keyboard, screen reader, phone). Grading: that rubric (40) and `.impeccable/rubrics/craft.md` (20), three blind graders, median.

## Direction contract

THESIS: The category standard, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services (ADR-0063): the familiar arrangement with nothing out of place. On call is the Schedule's sibling and reads as the same product: same title, filter bar, result line and table, with births and who is on call for them in place of Visits.

OWN-WORLD: The Staff world as `DESIGN.md` records it. One blue for links and the primary action. A day is said the way a person says it; tabular figures for every day and time. One filled badge, amber, only on a birth that needs cover and a Doula who holds two births on one day, because those need someone to act (One Meaning Per Hue); a covered birth is plain words.

STORY: At 2 a.m. an owner opens On call on a phone and reads, on the first screen, who is on call tonight and whether any birth has nobody. At a desk an office manager filters to the births that need cover, opens one at its On call section, covers the gap, and comes back to the same place. A Doula sees her own births and her days. When the Practice changes how early it goes on call, the rule is stated where the roster is read and changed one press away.

FIRST VIEWPORT: Top bar with the nav; title "On call"; the filter bar above the result line; the first birth in the first screen at 1280px and 375px.

FORM: The category standard (canon). No signature component: a list is not a record (ADR-0063).

## Where these screens differ from the climbed lists, and why

- **The roster is one whole answer, never a page of one.** The BFF returns every birth on call in the days chosen, bounded at 92 days, because the two things the screen totals (the births needing cover and each Doula's births) are counted over the whole answer (`api/internal/oncall/roster.go`). There is no "Load more", no cursor and no "first 30 of". A total is the count itself.
- **The row is a birth, and its facts are days, not instants.** An On-call window is derived on every read from the due date and the Practice's rule (`GLOSSARY.md:45`); only a Coverage gap has a clock (`GLOSSARY.md:47`).
- **It is the Practice's live book, not its history.** A completed birth and a cleared gap leave it, so it offers no day before today; the Activity on an Engagement holds what happened.
- **Three tables, one screen.** The births on call, the births with no days yet, and the Doulas. The Doulas table is the by-person reading of the same days.
- **What a reader holds differs by role in the BFF, and the screen hides nothing.** A contractor Doula's roster holds only the births she is on, and a colleague's birth count is absent for her (`roster.go`). An employed Doula reads the whole Practice.
- **The settings route is a form, not a list.** Its sibling is the Timezone screen (`FormPage`), and it starts from that arrangement.
- **Nothing records who is at a birth now** (#428, RA-G18), and the screen does not invent it: a Doula who "cannot be reached" has said so herself with a Coverage gap.
- **The days chosen are the Practice's** (ADR-0036; built in #2872, PR #2878), and a reader in another zone is told so once.

## Category baseline: the roster

Read on 2026-11-14 by an Opus agent, each claim from the product's own page. The verdicts were then read by a second Opus agent and by Gemini (which timed out part way); the rows they rejected are builds, and a decline they accepted for a weak reason carries the better one. Unread: Deputy's pages; Opsgenie's override page; PagerDuty's account-wide "who is on call now" page; any product's contact-from-rota; a keyboard-shortcut or density page for a rota.

| Convention | Verdict | Reason |
| --- | --- | --- |
| Who is on call now, shown first, with a today marker | Build | PagerDuty: "The user currently on call will be listed with a Now pill to the right of their name." (https://docs.pagerduty.com/incident-management/schedules/schedule-basics); OneUptime's red "Now" line (https://oneuptime.com/docs/en/on-call/schedule-timeline); the journey's 2 a.m. job (`journeys/practice-owner.md:158`). The births on call tonight come first under their own heading, as the Schedule groups a day, each says who holds tonight, and "Tonight" is a choice of days |
| The reader's own shifts | Build | PagerDuty's My On-Call Shifts (https://docs.pagerduty.com/incident-management/schedules/my-on-call-shifts); When I Work's My Schedule; the Schedule's "Mine", which the nav's link turns on for a Doula while the bare address stays the whole Practice (#2870); Priya comes "to know where she has to be" (`personas/employed-doula.md:17`) |
| A timeline or calendar grid beside the list | Later (#330) | PagerDuty, OneUptime and When I Work all draw one; it is deferred with #330's calendar, with the Schedule's month view (#2888 takes that reason up); a birth is on call for five weeks, so a list carries it meanwhile |
| Earlier and later, and back to today | Build | PagerDuty: "Click to move back an iteration of the selected time period, or click to move forward." and "Click Today" (schedule-basics); the Schedule's "Earlier" and "Later" (#2870). "Earlier" stops at today: the roster is "a Practice's live book, not its history" (`roster.go`), so a past day would leave out births since completed and gaps since cleared and answer "was she covered that night" (`GLOSSARY.md:47`) wrongly |
| Presets for the days | Build | PagerDuty's time period list (1 Day to 30 Days); the Schedule's "Days" radios, with "Tonight" added because this screen's first question is tonight |
| Filter by person | Build | PagerDuty: "To filter by user, click the User dropdown and search for your preferred user."; the Schedule's Doula select |
| A filter to what needs cover, with its count | Build | RosterElf: "A warning counter in the roster header displays the total shifts with warnings." and "Check specific warnings to focus on the affected shifts." (https://www.rosterelf.com/support/knowledge-base/scheduling-and-rostering/roster-warnings); the Schedule's "Needing a new Doula" box (#2863); the count is on the result line today, the filter is not |
| A gap shown as a gap | Have it | the warning Badge, "Nobody on call" with its days, "nobody covering" |
| A birth with no Doula yet, on the roster | Build | the birth that needs cover most never reached the roster (`roster_build.go` kept only the no-due-date reason); the Engagement says "Nobody is on this birth yet." (`journeys/practice-owner.md:51`). It is listed and counted as needing cover, for an Owner, Admin or employed Doula |
| A flag on a person booked twice at once | Build | Sling: "a clear indication that an employee is already scheduled elsewhere." (https://support.getsling.com/en/articles/1821074-conflicts). The flag is for two births on one day, on the Doula's row and beside her name on each birth; today it counts every birth in the range, sits in the Doulas table only, and the result line says "all covered" (`test-plans/practice-owner.md:132`, whose cell changes with this build). A Doula carries "three or four Engagements at a time" (`personas/employed-doula.md:11`), so a count over 30 days flags nearly everyone |
| Cover edited from the roster itself | Declined | PagerDuty creates an override from a shift. The control that does the job is the Engagement's On call section, one action from the row and opened with the control in view (built here): a Coverage gap's covering Doula "must already hold a granted Attachment on that Engagement" (`GLOSSARY.md:47`), so the act needs the Engagement's facts |
| The by-person reading (a Doula's births) | Build | When I Work: "Use users view if you prefer to think about schedule coverage based on who is scheduled."; the Doulas table names each Doula's births for a reader who may hold them; the test plan already expects "Both births are listed under the one Doula" (`test-plans/practice-owner.md:132`) |
| A reader-chosen order | Build | the Schedule gives its reader the order (#2869), and both readings of this table rejected "a filter does it". Tonight's births, then due date soonest first, is the default; "Needing cover first" is the other. The BFF serves both. The first draft declined it |
| The due date on the row | Build | the days on call come from it, the roster's response carries it, and no column showed it |
| Search by Client | Declined | no job on record asks for one birth's cover by name (Dee's calls are about Visits, which the Schedule's search answers), and the roster is one whole answer on one page, so the browser's own find reaches every row |
| Paging | Declined | one whole answer bounded at 92 days (`roster.go` records the departure from `docs/api-design.md` section 4) |
| Bulk selection and bulk actions | Declined | no job on record; every act on a birth's cover is on its Engagement |
| A primary and a backup named as roles | Declined | the model holds each Doula's days, not a role (`GLOSSARY.md:49`): "the primary carries the window up to a day, the backup from the next one" |
| Counts | Have it | the result line: births on call, and how many need cover |
| Empty, loading and failed states | Build | measured 2026-11-14: the empty result is said twice, and a To before From or a range over 92 days draws the full "Sorry, there is a problem" page that blames "our end" |
| The view in the address | Have it | From and To are in the address today (a GET form); each filter built here joins them |
| Applied filters as tags, and Clear | Build | MOJ: "The selected filters are displayed at the top to let users see what they've selected and remove them easily." (https://design-patterns.service.justice.gov.uk/components/filter/); #2734's On call half |
| Time zone shown | Have it | the Practice's days, said once where the reader's zone differs (#2872); no switcher, because a Practice's days are its own (ADR-0036) |
| The longest range stated | Build | PagerDuty: "The calendar will only display shifts up to 90 days in the future."; the BFF's 92 days is unsaid until it is refused |
| The rule in force stated on the roster, with a way to its settings | Build | no product page was read for this; the reason is the product's own: the days are derived from the rule (`GLOSSARY.md:45`), and today the roster neither states it nor links its settings. The roster's response carries the rule for every reader (the settings read is Owner and Admin only) and marks a birth with its own start rule; the link is drawn for an Owner or Admin |
| A calendar feed (.ics) | Later (#330) | PagerDuty, Opsgenie and RotaCloud each export one; with the Schedule's feed |
| Print or export | Later (#2871) | RotaCloud's "Print rota"; decided with the lists' export |
| A notice when a gap opens | Have it | an unstaffed Coverage gap sends the Owners and Admins a Notification (`api/internal/oncall/outbox.go`) |
| Calling or messaging the Doula on call from the roster | Later | the model holds no phone number for a Staff member (only a Client has one), and Rooted covers by "a group text thread for coverage, and Renata's phone for everything else" (`docs/simulation/worlds/rooted-birth-collective.md:111`). No product page was read. Whether Staff phone numbers enter the product is the founder's decision, raised on #2748 |
| Who is at a birth now | Later (#428) | nothing records it (RA-G18) |
| Keyboard shortcuts | Later | one decision for the whole app, as on the Schedule |
| Density and column choice | Declined | no job on record |
| A whole-row click target | Have it (#2905) | `DataTable` opens the row's link from a click anywhere on a row that sets `rowHref` |
| The free Doulas offered where a gap's cover is chosen | Declined | When I Work and Deputy offer available staff for an open shift (unread; one grader's note, 2026-11-14). A covering Doula "must already hold a granted Attachment on that Engagement" and a gap is "never a back door to putting somebody on a birth" (`GLOSSARY.md:47`), so the list is the birth's own Doulas; who is free tonight is the Overview's block and the Doulas table |
| One place to say "I cannot be reached" across all of a Doula's births | Later | Priya's blind walk reached for it (2026-11-14). A gap is about one birth (`GLOSSARY.md:47`), and the birth's row reaches its form in one action; one interval written to several births at once has no job on record. A ticket carries it |
| Sort by a column heading | Declined | two graders named it in round 1 (Stripe's tables). The Order select is the reader-chosen order at every width; a heading-only button is gone where the births draw as cards (ADR-0024), which is the Schedule's recorded reason (#2869) |
| The births as a table at 1280px | Build | the sibling lists draw tables at 1280px; this roster drew cards until 1920px (measured 2026-11-14) |

## Category baseline: the rule's settings

| Convention | Verdict | Reason |
| --- | --- | --- |
| A usual answer and what it means | Have it | GOV.UK hint text; "37 is the usual answer…", "14 covers a post-dates birth" |
| A follow-up question revealed by its option | Build | GOV.UK radios' conditional reveal (https://design-system.service.gov.uk/components/radios/); the week shows only for "At a week of pregnancy" but is drawn after the whole group, not under its option |
| A warning before an act with consequences | Have it | GOV.UK warning text |
| A preview of the effect before saving | Build | PagerDuty previews a schedule before saving ("a preview of what the schedule will look like after saving changes"). Moving the week from 37 to 39 takes births off call tonight, and the warning did not say so, and said "every" window though a birth with its own start rule keeps it. The form says how many births on call now would stop being on call. The first draft declined it on one persona's silence |
| Confirmation after saving | Have it | a status under Save stating the saved rule (`govuk-alignment.md` records the in-place departure) |
| Who changed it, and when | Build | Linear: "Linear automatically tracks events related to … settings changes" (https://linear.app/docs/audit-log); Stripe's Activity Logs; the save is recorded (`settings.go`) and not shown; CLAUDE.md's audit trail |
| Every change to the rule, not only the last | Build | two graders named it in round 1 (Linear's audit log, Stripe's Activity Logs). The Practice's Activity already holds each change with who and when; its line gains the values ("from week 37 to week 38"), which Dee's walk looked for, and the route links to it |
| The bounds stated before a refusal | Build | the weeks (20 to 42) and days (0 to 42) are said only in the BFF's refusal |
| A pointer to the one-birth override | Build | the rule can be changed for one birth on its page, and the form does not say so |
| A way back that changes nothing | Build | GOV.UK: "Always include a Back link component at the top of question pages" (https://design-system.service.gov.uk/patterns/question-pages/), in whatever form the sibling settings screens take, built in the shared Template so they all gain it |
| Filled with the saved value | Build | the form drew 37 and 14 until the saved rule arrived, and kept them, with Save live, when the read failed, so a press wrote the usual answers over the Practice's own |
| An error summary | Have it | `ErrorSummary` |
| Reset to default | Declined | no job on record; the hints name the usual answers |
| A date the change takes effect | Declined | the days are derived on every read and never stored (`GLOSSARY.md:45`), so a rule has no past or future version to schedule |
| A guard on unsaved changes | Later | one decision for every `FormPage`, not one per form; unread |

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict and DESIGN.md
