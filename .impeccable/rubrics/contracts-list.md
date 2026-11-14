# Contracts list: what a 4 means

One route, `contracts/`, with two work lists on it: the Contracts, and the void requests nobody has decided. A heuristic's checks read both, and a failure on either costs the point.

Each heuristic's checks are lettered, one claim to a letter. A heuristic scores 4 only when **every** lettered check passes, each with evidence a grader can point to: a screenshot, a measured value, the accessible text, or a walk's act log. A check with no evidence fails. 3 = one lettered check fails, 2 = two, 1 = three or more, 0 = the heuristic's core job is absent. A job (T1–T10) costs its point under the heuristic that names it: T1–T5 under 7, T6 and T7 under 5, T8 and T9 under 4, T10 under 9.

Graders are three independent agents per round. None sees an earlier score, this branch's commits, or another grader's output. The score per heuristic is the median of the three.

In this rubric the **queue** is the void requests nobody has decided, which an Owner or Admin reads; a **view** is a choice of Contract states.

Seed: the demo Practice under `dev:full` (Rooted Birth Collective, 14 Doulas). A book of 31 or more Contracts across every state: three or more not sent yet, at least two of them with every merge field filled and a Client who can reach the portal, so "Send Contract" draws; six or more sent and not signed, waiting from today to more than 40 days; signed ones enough to page the whole book; at least two voided, one of them on an Engagement that holds a newer Contract; three void requests nobody has decided, asked by two or more Doulas on different days; one request declined and one that ended in a void. At least one Contract not signed on an Engagement Priya is on; Lena with a granted attachment on an Engagement that holds a Contract. Widths: 1440px, 1280px, 768px, 375px; 200% zoom at 1280px.

## The jobs

Each is a hypothesis from the persona records (`docs/personas`, `docs/journeys`, `docs/test-plans`), not research (ADR-0045). The records name the jobs; the action budgets are ours. A press of Apply is an action. A row's link lands at the Engagement's Contract section, as the Invoices list's lands at its Invoices section (#2240).

- **T1 Dee, who to chase:** "Has she signed yet?" On arriving on Contracts, see every Contract that is not signed, with the count stated; in ≤ 2 actions see only those sent and waiting on the Client, the longest wait first, each row with the Client, the day it was sent, how many days it has waited and who sent it. The wait is counted from the day it was sent, never from the day the Draft was made. (`journeys/non-doula-admin.md:119`, `:185`, `personas/non-doula-admin.md:11`)
- **T2 Dee, get it out:** "Get it out today." In ≤ 2 actions see the Contracts made and not sent yet, with their count, none of them said to be waiting on a Client. In 1 action from a row the Engagement opens with its Contract section's heading and either "Send Contract" or the notice that says why it cannot be sent yet, in view at 1280×800. After sending, Back returns to the list with the view and position kept: in the not-sent view the Contract has left it and the count is one lower; in the not-signed view its row reads sent today. (`journeys/non-doula-admin.md:103`, `api/internal/contracts/awaiting.go`)
- **T3 Renata or Dee, a void to decide:** on arriving, read how many void requests wait for a decision and, for each, the Client, who asked, the whole reason and how long ago, before the Contracts list and without an action. In 1 action from a request the Engagement opens at its Contract section with the request ("… asked to void this Contract: …"), its "Decline" and the "Void this Contract" button in view at 1280×800. After a decline there, Back returns to the list and the request has left the queue. (#971, `test-plans/non-doula-admin.md:91`, `:92`, `:283`, `:284`)
- **T4 Renata, any Client's Contract:** find one Client's Contract in a book of 31 or more, whatever its state, by typing a part of the name: ≤ 3 actions from arriving (the whole book, the name, Apply; the search reads within the view, as on Invoices), no Client's name in the address (#1829), and Back from the Engagement returns with the view, the search and its results kept. A signed row says the day it was signed and, for a reader the PDF read serves, offers the Signed PDF in 1 action under the Engagement's own label for it; a voided row says so in the Engagement's words. (`journeys/practice-owner.md:4`, `:6`, `worlds/rooted-birth-collective.md:111`, `:166`, `GLOSSARY.md:59`)
- **T5 Renata, one Doula's Contracts:** in ≤ 2 actions the list shows only the Contracts on one Doula's Engagements, with a tag that names her and the choice in the address; one action clears it. The address with nothing chosen is the whole Practice, and the nav's link opens the whole Practice for Renata, who is an Owner. (`personas/practice-owner.md:11`, `:31`)
- **T6 Priya, phone (375px), her own:** signed in as Priya, an employed Doula with no Owner or Admin role (walked as her by `persona-walker`, never judged from Renata's session), the nav offers her Contracts (#2851, decided 2026-11-14: the nav, for every Staff member the read serves), and arriving by it the list opens on the Contracts of her own Engagements ("Mine", which the nav's link turns on for a Doula who is neither Owner nor Admin, as #2870 decided for the Clients list and the Schedule; a tag she can clear to read the whole Practice, which ADR-0008 gives her). The first Contract is whole on the first screen (375×740). 0 actions to hers, ≤ 2 to everyone's. She is shown no queue and no refusal. (#973: "she has no list of what she is waiting on"; #2851; `personas/employed-doula.md:13`)
- **T7 Lena, a contractor:** signed in as Lena (walked as her), the nav offers her Contracts and the list holds only the Contracts on Engagements she holds a granted attachment on; no other Client's name and no Contract's amount is on the screen or in the list's response; no Doula select and no "Mine" are drawn for her; no signed row offers her the Signed PDF, whose read refuses her; she is shown no queue; and where a view holds nothing for her the screen says so in words, never as an error. (ADR-0008, #973, #2851)
- **T8 Screen reader:** at 1280px each table's purpose and row count are announced, and a Contract's row reads as the Client, then each fact under its column's name once, with audible separators. At 375px a card reads the same facts in the same order, each with its label. A state is said in words, not by color or icon alone. A voided Contract and its replacement, and a Client's Contract and a void request on it, have links whose accessible names differ.
- **T9 Keyboard:** every control and row link is reachable in drawn order with a visible focus ring; "Skip to results" is offered before the filter bar and reaches the result line in ≤ 2 stops and the first Contract in one more; a radio group is one stop; nothing that is not a control takes a stop, and no control this reader is not offered does.
- **T10 Failure:** with the BFF refusing the Contracts, the screen says what happened in Voice-rule words, the view chosen is kept, and "Try again" recovers. With only the void requests refused, the Contracts list still stands and the queue says in place that it did not load. A refused "Load more" keeps the rows already shown and says so beside the button.

## Heuristic checks

1. **Visibility of system status**
   - a. Loading shows a skeleton within 300ms and nothing moves when the content lands.
   - b. Every filter change visibly changes the list or says that nothing matched.
   - c. The result line states how many Contracts are shown, and of how many while more exist.
   - d. The result line states the view and the order.
   - e. Every applied filter, and a search, is stated as a tag or in the result line.
   - f. For an Owner or Admin, the queue states its own count.
   - g. "Load more" states progress.
2. **Match with the real world**
   - a. Every label is a word an owner, an office manager or a doula uses.
   - b. No two labels on the screen can be read as the same thing.
   - c. A Contract's state uses the Engagement's words for it: "Draft, not sent yet", "Sent, waiting for the Client to sign", "Signed", "Voided, no longer active", or a shortening of each that keeps its first word. A Draft is never said to be waiting on anyone but the Practice.
   - d. Every day reads as ADR-0022 writes one, the year only when it is not this one.
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
8. **Aesthetic and minimalist design**
   - a. A badge is on a state, never across a row or a cell.
   - b. No row, cell or badge is red: no Contract state is a failure (an error notice under T10 is not a state).
   - c. Table rows have equal rhythm (row heights within 4px of each other, a wrapped reason excepted).
   - d. Cards keep one field order.
   - e. No element on the page carries no information.
   - f. No fact is said twice in a row, or between the intro, the result line and the table.
   - g. At 1280×800 as Renata, with the seed's three requests waiting, and at 375×740 as Priya, who has no queue, the first Contract (Client, state, and the day or the wait) is wholly on the first screen under the title and the filters.
   - h. With no request waiting, one line says so and no empty table or heading-and-paragraph block stands above the Contracts.
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
