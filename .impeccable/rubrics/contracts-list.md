# Contracts list: what a 4 means

One route, `contracts/`, with two work lists on it: the Contracts, and the void requests nobody has decided. A heuristic's checks read both, and a failure on either costs the point.

Each heuristic's checks are lettered, one claim to a letter. A heuristic scores 4 only when **every** lettered check passes, each with evidence a grader can point to: a screenshot, a measured value, the accessible text, or a walk's act log. A check with no evidence fails. 3 = one lettered check fails, 2 = two, 1 = three or more, 0 = the heuristic's core job is absent. A job (T1–T13) costs its point under the heuristic that names it: T1–T5, T11, T12 and T13 under 7, T6 and T7 under 5, T8 and T9 under 4, T10 under 9.

Graders are three independent agents per round. None sees an earlier score, this branch's commits, or another grader's output. The score per heuristic is the median of the three.

In this rubric the **queue** is the void requests nobody has decided, which an Owner or Admin reads; a **view** is a choice of Contract states.

Seed: the demo Practice under `dev:full` (Rooted Birth Collective, 14 Doulas). A book of 31 or more Contracts across every state: three or more not sent yet, at least two of them with every merge field filled and a Client who can reach the portal, so "Send Contract" draws; six or more sent and not signed, waiting from today to more than 40 days; signed ones enough to page the whole book; at least two voided, one of them on an Engagement that holds a newer Contract; three void requests nobody has decided, asked by two or more Doulas on different days; one request declined and one that ended in a void. At least one Contract not signed on an Engagement Priya is on; Lena with a granted attachment on an Engagement that holds a Contract. Widths: 1440px, 1280px, 768px, 375px; 200% zoom at 1280px.

## The jobs

Each is a hypothesis from the persona records (`docs/personas`, `docs/journeys`, `docs/test-plans`), not research (ADR-0045). The records name the jobs; the action budgets are ours. A press of Apply is an action. A row's link lands at the Engagement's Contract section, as the Invoices list's lands at its Invoices section (#2240).

- **T1 Dee, who to chase:** "Has she signed yet?" On arriving on Contracts, see every Contract that is not signed (made or sent, never one voided), with the count stated; in ≤ 2 actions see only those sent and waiting on the Client, the longest wait first, each row with the Client, how many days it has waited (the day it was sent is in the card and said to a screen reader, and the table row gives its place to "Opened"/"Not opened", #2952) and who sent it. The wait is counted from the day it was sent, never from the day the Draft was made. (`journeys/non-doula-admin.md:119`, `:185`, `personas/non-doula-admin.md:11`)
- **T2 Dee, get it out:** "Get it out today." In ≤ 2 actions see the Contracts made and not sent yet, with their count, none of them said to be waiting on a Client. In 1 action from a row the Engagement opens with its Contract section's heading and either "Send Contract" or the notice that says why it cannot be sent yet, in view at 1280×800. After sending, Back returns to the list with the view and position kept: in the not-sent view the Contract has left it and the count is one lower; in the not-signed view its row reads sent today. (`journeys/non-doula-admin.md:103`, `api/internal/contracts/awaiting.go`)
- **T3 Renata or Dee, a void to decide:** on arriving, read how many void requests wait for a decision and, for each, the Client, who asked, the whole reason and how long ago, before the Contracts list and without an action. In 1 action from a request the Engagement opens at its Contract section with the request ("… asked to void this Contract: …"), its "Decline" and the "Void this Contract" button in view at 1280×800. After a decline there, Back returns to the list and the request has left the queue. (#971, `test-plans/non-doula-admin.md:91`, `:92`, `:283`, `:284`)
- **T4 Renata, any Client's Contract:** find one Client's Contract in a book of 31 or more, whatever its state, by typing a part of the name: ≤ 3 actions from arriving (the whole book, the name, Apply; the search reads within the view, as on Invoices), no Client's name in the address (#1829), and Back from the Engagement returns with the view, the search and its results kept. A signed row says the day it was signed and, for a reader the PDF read serves, offers the Signed PDF (`GLOSSARY.md`'s name for it) in 1 action, its accessible name the Engagement's own label for it; a voided row says so in the Engagement's words. (`journeys/practice-owner.md:4`, `:6`, `worlds/rooted-birth-collective.md:111`, `:166`, `GLOSSARY.md:59`)
- **T5 Renata, one Doula's Contracts:** in ≤ 2 actions the list shows only the Contracts on one Doula's Engagements, with a tag that names her and the choice in the address; one action clears it. The address with nothing chosen is the whole Practice, and the nav's link opens the whole Practice for Renata, who is an Owner. (`personas/practice-owner.md:11`, `:31`)
- **T6 Priya, phone (375px), her own:** signed in as Priya, an employed Doula with no Owner or Admin role (walked as her by `persona-walker`, never judged from Renata's session), the nav offers her Contracts (#2851, decided 2026-11-14: the nav, for every Staff member the read serves), and arriving by it the list opens on the Contracts of her own Engagements ("Mine", which the nav's link turns on for a Doula who is neither Owner nor Admin, as #2870 decided for the Clients list and the Schedule; a tag she can clear to read the whole Practice, which ADR-0008 gives her). The first Contract is whole on the first screen (375×740). 0 actions to hers, ≤ 2 to everyone's. She is shown no queue and no refusal. (#973: "she has no list of what she is waiting on"; #2851; `personas/employed-doula.md:13`)
- **T7 Lena, a contractor:** signed in as Lena (walked as her), the nav offers her Contracts and the list holds only the Contracts on Engagements she holds a granted attachment on; no other Client's name and no Contract's amount is on the screen or in the list's response; no Doula select and no "Mine" are drawn for her; no signed row offers her the Signed PDF, whose read refuses her; she is shown no queue; and where a view holds nothing for her the screen says so in words, never as an error. (ADR-0008, #973, #2851)
- **T8 Screen reader:** at 1280px each table's purpose and row count are announced, and a Contract's row reads as the Client, then each fact under its column's name once, with audible separators. At 375px a card reads the same facts in the same order, each with its label. A state is said in words, not by color or icon alone. A voided Contract and its replacement, and a Client's Contract and a void request on it, have links whose accessible names differ.
- **T9 Keyboard:** every control and row link is reachable in drawn order with a visible focus ring; "Skip to results" is offered before the filter bar and reaches the result line in ≤ 2 stops and the first Contract in one more; a radio group is one stop; nothing that is not a control takes a stop, and no control this reader is not offered does. The heading buttons (#2955) add 2 Tab stops (Status and Client, always both: the sorted Client in Client order is drawn unavailable with aria-disabled) between the result line and the first Contract; "Skip to results" is unchanged and the first Contract is reached in one stop plus those, a screen reader skips them by table navigation, and a button is operable with Enter or Space and keeps focus through the reorder.
- **T10 Failure:** with the BFF refusing the Contracts, the screen says what happened in Voice-rule words, the view chosen is kept, and "Try again" recovers. With only the void requests refused, the Contracts list still stands and the queue says in place that it did not load. A refused "Load more" keeps the rows already shown and says so beside the button.
- **T11 Renata, the Contracts of a period:** from arriving on Contracts, see the Contracts sent in October, or this year, in ≤ 2 actions (the `Sent` select applies when changed), with a tag in fixed words ("Sent in October 2026") that reads the same next month and the result line saying the period; a Contract is in a period by the day it was sent in the Practice's zone, so a signed one is in the period it was sent, and the Draft view, which holds nothing that was sent, is not offered the select; with chosen days, From and To are typed and wait for Apply. The select, its words and its address (`?sent=last-month`, or `?sentFrom=&sentTo=`) are the Invoices list's. (`personas/practice-owner.md:19`, `worlds/rooted-birth-collective.md:111`, #2954)
- **T12 Dee, a spreadsheet of what the list shows:** one press on "Download CSV", which an Owner or Admin sees, takes every Contract the applied view, search, order, Mine, Doula and period match, not the 30 loaded, in plain-word columns with no clinical field and no email, phone or address, the Fee included for a reader who may read it; a refusal or no connection is said in place in Voice-rule words with the button still there; each download is one Activity entry ("Contracts downloaded"). The button, its words and its status are the Invoices list's. (`worlds/rooted-birth-collective.md:111`, #2954)
- **T13 Dee, who has not even looked:** "Check she can get in, or call her." On the Sent view, in 0 actions beyond arriving there, every Sent row says either "Opened" with the day the Client first read it in the Practice's zone, or "Not opened" (the card: "opened Nov 12" or "not opened"), on the row's one line at 1280px in the not-signed, Sent and All views; the Engagement's Contract section says the same ("Opened" with the day, or "Not opened yet") for a Sent Contract and says nothing of opening for any other state; the Client's portal Contract page says in one line that her Practice can see she has opened it. A Staff read, a preview or a Signed or Voided Contract records nothing; only her first read counts. (#2952, decided 2026-11-15; `GLOSSARY.md`'s Contract entry)
- **T14 Dee, chase a Contract that waits:** "Nudge her." On a Sent row that was neither sent nor reminded today, in 1 action from arriving on the Sent view, press "Remind" (the card: "Remind the Client"); the screen says in place that the reminder was sent and the control goes, with no other row moving and every row still one line at 1280px in the not-signed, Sent and All views; the Engagement's Contract section offers the same button, says "Last reminded" with the day and who, and its Activity holds one entry naming who sent it. A Client already emailed about a Contract today (a reminder, or the send) is refused in plain words by the BFF and the control is not offered on that day. The day is the Practice's, not UTC's. The email is content-free (ADR-0009). Scheduled reminders and bulk remind are declined for v1. (#2951, decided 2026-11-15)

## Heuristic checks

1. **Visibility of system status**
   - a. Loading shows a skeleton within 300ms, and neither the queue, the filter bar nor the Contracts move when the content lands.
   - b. After every filter change the result line and the rows match what is applied, or the screen says that nothing matched.
   - c. The result line states how many Contracts are shown, and of how many while more exist.
   - d. The result line states the view and the order.
   - e. Every applied filter, and a search, is stated as a tag or in the result line.
   - f. For an Owner or Admin, the queue states its own count.
   - g. "Load more" states progress.
2. **Match with the real world**
   - a. Every label is a word an owner, an office manager or a doula uses.
   - b. No two labels on the screen can be read as the same thing.
   - c. A Contract's state uses the Engagement's words for it: "Draft, not sent yet", "Sent, waiting for the Client to sign", "Signed", "Voided, no longer active", or a shortening of each that keeps its first word. A Draft is never said to be waiting on anyone but the Practice.
   - d. Every day reads as ADR-0022 writes one, with the year only when it is not this one (`DESIGN.md`, "The Schedule's days"), and no clock time.
   - e. A wait is a count of days ("12 days"), never an instant, and "today" for one sent today.
   - f. The capital on Contract, Client, Doula and Practice follows the domain-noun rule for a Staff screen.
3. **User control and freedom**
   - a. Every filter is in the address (back, reload and a shared link keep it), except the search, which back and forward keep (#1829).
   - b. Any one filter is cleared in one action.
   - c. All filters are cleared in one action.
   - d. Leaving for a Contract and coming back by the browser's Back restores the view and the position.
   - e. Leaving for a void request and coming back does the same.
4. **Consistency and standards** (T8 and T9 cost their points here)
   - a. Title, intro, filter bar, result line and table are the Invoices and Clients lists' components in the same place (measured: same title x and size, same filter bar arrangement, same result line, same table header and row style).
   - b. Badge hues follow `DESIGN.md`'s One Meaning Per Hue: amber where the Practice must act (not sent yet; a void requested), blue where it waits on the Client (sent); signed and voided are plain words.
   - c. A control the sibling lists also have (Show, Doula, Mine, Order, the search, tags, "Clear filters", "Load more") has their name, their place in the bar and their behavior: a box, a radio, a select or the order applies when changed (#2888); the search waits for Apply.
   - d. A convention the category has taught this screen's readers (the surface brief's Category baseline) is on the screen, or the brief records its decline with a reason that holds for a reader those products have trained.
   - e. T8 passes.
   - f. T9 passes.
5. **Error prevention** (T6 and T7 cost their points here)
   - a. Nothing on the list sends, voids or declines in one press; each act is on the Engagement, with the Contract in view.
   - b. A filter that cannot narrow is not offered (Mine and the Doula select where the Practice has one Doula), and a combination that can never match is not offered.
   - c. No reader is shown a control, a link or a section whose read or act the BFF refuses that reader.
   - d. T6 passes.
   - e. T7 passes.
6. **Recognition rather than recall**
   - a. The order is stated and serves the job: in the view of what is not signed, the Contracts sent come longest wait first, and the brief's arrangement says where those not sent yet stand.
   - b. Filters are visible, not hidden in menus, at 1280px.
   - c. A row says, without opening it, whose step is next: one not sent yet says so and the day it was made; one sent says the day it was sent and the days waited.
   - d. A row names the Doula on the Engagement and the due date, and for a reader who may read it the Contract's amount.
   - e. A signed Contract somebody has asked to void says so on its row.
   - f. A void request says who asked and why in full, without opening it.
7. **Flexibility and efficiency** (T1–T5 cost their points here)
   - a. T1 passes within its budget.
   - b. T2 passes within its budget.
   - c. T3 passes within its budget.
   - d. T4 passes within its budget.
   - e. T5 passes within its budget.
   - f. T11 passes within its budget.
   - g. T12 passes within its budget.
   - h. T13 passes within its budget.
   - i. T14 passes within its budget.
8. **Aesthetic and minimalist design**
   - a. A badge is on a state, never across a row or a cell.
   - b. No row, cell or badge is red: no Contract state is a failure (an error notice under T10 is not a state).
   - c. Table rows have equal rhythm (row heights within 4px of each other, a wrapped reason excepted).
   - d. Cards keep one field order.
   - e. No element on the page carries no information.
   - f. No fact is said twice in a row or a card, and the intro repeats nothing the result line or a tag states.
   - g. With no request waiting (at 1280×800, and at 375×740 as Priya, who has no queue), the first Contract (Client, state, and the day or the wait) is wholly on the first screen under the title and the filters. With requests waiting, at 1280×800 as Renata the first screen holds every one of the seed's three requests whole, and the filter bar.
   - h. With no request waiting, one line says so and no empty table or heading-and-paragraph block stands above the Contracts (the seed always holds requests, so this is judged from the screen's spec).
9. **Error recovery**
   - a. T10 passes.
   - b. A view or search with no results says what was looked for and offers the way back to the whole book.
10. **Help and documentation**
    - a. Every filter's effect is clear from its label.
    - b. The intro says what the list holds and what a row opens, in one or two lines, and stays true in every view.
    - c. For an Owner or Admin, the queue says what deciding a request means and where it is done.
    - d. The empty state of each view says what was looked for and what to do.
    - e. No term needs a glossary.

## Cross-cutting (each failure costs a point from the heuristic named)

- 320px and 200% zoom: no horizontal page scroll, every control usable (4, 7).
- WCAG 2.2 AA: contrast, focus not obscured, target size 24px (4, 7).
- The constraints hold: ADR-0021 GOV.UK patterns, ADR-0024 intrinsic layout (no viewport queries), the Voice rules (4, 2).

## Changes to this rubric

- 2026-11-14, after round 2, on the founder's reading of the page: H8g held the first Contract to the first screen at 1280×800 with three requests waiting, and to pass it each request had been pressed onto one line of three tracks. The founder read that line on the rendered screen and said it was "very awkward to read" and that a request belongs in a card that groups its facts. A void is the one decision on this screen that is not undone, and the person deciding it reads who asked and why before anything else, so the check was wrong about that person's job: with requests waiting, the first screen holds the requests and the filter bar, and the first Contract is held to the first screen only when no request waits.
- 2026-11-14, after the baseline round and the design call: T4 asked for the Signed PDF "under the Engagement's own label for it". That label, "Download signed Contract (PDF)", is 30 characters; as a visible cell it widens the table past the 1280px frame, and the list would then draw cards where its siblings draw a table (H4a). The person's job is to get the PDF in one action and to hear what the button does, so the label is the button's accessible name, and a card shows it whole. H2d now names the rule it had stated without a source: the year only when it is not this one is the Schedule's and On call's (`DESIGN.md`), and a day here carries no clock time.
- 2026-11-15, #2954: two jobs added, as the Invoices rubric's T9 and T10 were by #2871, and the surface brief's baseline row for a period and an export turned to "Have it". **T11 Renata, the Contracts of a period** and **T12 Dee, a spreadsheet of what the list shows**, both costing under H7, each on the Invoices list's control and words. The three personas asked on 2026-11-15 had no job on record for either, so the build rests on the category (PandaDoc, HoneyBook, Stripe) and on Renata's habit of grouping Contracts by year; the pilot agency's owner and admin are asked in their usability sessions (`docs/research/usability-sessions.md`). H7 gained f and g.
- 2026-11-15, #2952: one job added, **T13 Dee, who has not even looked**, costing under H7 (H7 gained h). The founder decided on 2026-11-15 that a Practice is shown the first time a Client opens her Sent Contract, and that the Client is told on her own Contract page; the surface brief's baseline row turned to "Have it".
- 2026-11-15, #2951: one job added, **T14 Dee, chase a Contract that waits**, costing under H7 (H7 gained i). The founder decided on 2026-11-15 that a Client is emailed when a Contract is sent and that a Staff member can remind her by hand, once a day; scheduled reminders and bulk remind are declined for v1, and the surface brief's baseline rows say so. At 1280px the Status cell's three tracks leave room for the "Remind" word and not the day she was last reminded, so the row carries that day in its title text and for a screen reader, and the card and the Engagement show it.
