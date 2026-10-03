# Usability sessions with the pilot agency: the protocol

The protocol for the five sessions [#1685](https://github.com/markgoho/doula-cloud/issues/1685) asks for, written before the first one. The founder runs the sessions. This file holds what he takes into each one: the recruiting note, the consent line, the seeded Practice, the five task cards, the observer sheet, and the debrief questions. It also says where each sheet is filed and how what the sessions find reaches the tracker.

## Why this exists

The Aesthetic-Usability row of [The Laws of UX](../design/brief.md#the-laws-of-ux) says a considered interface masks usability problems, so a screen looking good is never evidence that it works, and that evidence comes from watching somebody use it. Nobody has been watched. The nine personas are proto-personas, built from the founder's knowledge and the schema ([`docs/personas/README.md`](../personas/README.md)). The journey walks and the simulation ([#759](https://github.com/markgoho/doula-cloud/issues/759)) put an agent at the keyboard. All three are hypotheses. These five sessions are the first thing that can falsify them against a person.

The sessions run before the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)) starts and before 2026-12-01, so a finding has a build window before the January 2027 launch.

## The rules of a session

- **Five sessions, five people, all from the pilot agency**: the owner, the non-doula admin, and three doulas, one of them a contractor. One person per session, never two at once.
- **The founder observes.** He does not help, does not explain, and does not defend the product. When the person asks a question or stops, he says one line, and only that line: "What would you try next?"
- **Under an hour**, in this order: the consent line, the opening script, the invitation (about 10 minutes), the card (up to 25 minutes), the debrief (about 15 minutes). A card ends when the person reaches the goal, or says they would give up, or has made no progress for five minutes. A card that ends without the goal is recorded as not reached; it is never rescued.
- **The person's own device**: the phone or computer they would use for this work, in the browser they already use, signed in as themselves.
- **The deployed app**, at its real address, in a Practice seeded for the sessions ([The seeded Practice](#the-seeded-practice)). Never the local stack, and never the pilot agency's real data.
- **The person thinks aloud.** The opening script asks for it once. The founder does not prompt it again, except with the one line above.
- **No name or identifying detail of a person enters the repo.** Not the person's name, not their email address, not the name of the pilot agency, and not a screenshot from outside the browser's page (a status bar, a notification, a home screen). The pilot agency is "the pilot agency" in every file. A sheet names the person by the card's role only.

### The opening script

Said aloud, after the consent line, before anything is opened:

> Thanks for doing this. I'm testing DoulaCloud, not you, so nothing you do here can be wrong. As you go, please say out loud what you're looking for and what you think is happening. I won't help, because I want to see what happens when I'm not there. If you get stuck, tell me what you'd do next, and we'll stop where you'd stop.

## The recruiting note

The founder asks the owner first, in person or by text, and the owner introduces the admin and the three doulas. The note is the same for all five, two sentences, with no link:

> Would you give me under an hour before the end of November to try DoulaCloud on your own phone or laptop while I watch? I'm testing it, not you, and the more it trips you up, the more useful your hour is.

Each sheet records how the person came to the session: chosen by the owner, or offered themselves. People who opt in are the ones most likely to do well, and [ADR-0045](../adr/0045-a-doula-is-heard-before-she-is-chosen.md) names that selection bias beside each call rather than pretending it away.

## The consent line

Said aloud before the opening script, and the answer recorded on the sheet as yes or no:

> I'd like to record your screen and your voice while you try this, so I can check afterward what happened. Only I'll watch it, it won't go anywhere public, and I'll delete it once I've written up what I saw. Is that OK?

The recording is the device's own screen recording with the microphone on. At the end of the session the person sends the file to the founder and deletes their copy. The founder keeps it in his private storage, never in the repo, and deletes it when every finding from that session is filed. A person who says no is still watched; the session runs without a recording, and every number on the sheet that needs one is written "not measured".

## The seeded Practice

One Practice on the deployed app, shaped like the pilot agency and holding no real person's data except the five participants' own accounts.

**What makes it look like theirs.** The pilot is a 14-doula agency with an owner, a non-doula admin, and doulas, some of them contractors. The seeded Practice takes the World's Rooted Birth Collective as its shape, because that World was sized to this agency ([`docs/simulation/worlds/rooted-birth-collective.md`](../simulation/worlds/rooted-birth-collective.md)): 15 Staff, of whom 14 hold the Doula role and the Admin does not, and 4 of the 14 are contractors. The five participants take five of those seats in their own roles, so each one meets the Practice from where they sit in their own agency. The other ten Staff and every Client are fictional. The Practice may carry the pilot agency's own name and its own Birth Plan questions, if the owner shares the form the agency uses today, because both live only in the deployed database.

**What it holds.** A live book of birth and postpartum Engagements with Visits on real dates and an on-call roster, so a list is a real Practice's length and not a fixture's. On top of that, what each card needs to be walkable:

| Card | What the Practice holds for it |
| --- | --- |
| 1, owner | Two birth Engagements, June Ferris's and Rosa Lindqvist's, both in labor on the session's date and both with the same doula attached, who is already at June Ferris's birth, and an on-call roster that leaves one answer to who is free for Rosa Lindqvist |
| 2, admin | Ada Walker's signed Contract, with an Invoice for $1,200 that is unpaid |
| 3, doula | Maria Esposito's birth Engagement, with that participant attached and a filled Birth Plan that answers what Maria Esposito wants for pain relief |
| 4, doula | Nia Thompson's birth Engagement, with that participant attached and a filled Birth Plan that answers who Nia Thompson wants in the room |
| 5, contractor | A pending Offer to that participant, of Grace Okafor's birth Engagement due in February, carrying the dates, the on-call terms and the fee |

The records each card touches are separate, so one Practice serves all five sessions, as long as card 1's two births are dated to that session's day.

**How it is made.** There is no command for this yet. Every seed in the repo stops at the local stack: `bun run --cwd app seed:staff-session` (`app/scripts/seed-staff-session.ts`) signs up through the Firebase Auth emulator and inserts its Engagement with raw SQL (`seedEngagement`, `app/e2e/stack.ts`), and the World's provisioning (`app/e2e/simulation/provision.ts`) signs people in through the emulator and reads each invitation token from the local mailbox. None of those reaches Cloud Run, Identity Platform or Cloud SQL. [#1729](https://github.com/markgoho/doula-cloud/issues/1729) is the ticket for the deployed mechanism, and this paragraph names the command when it lands.

**The participant's account.** Each participant gets a real invitation to the seeded Practice, carrying their role and employment type, and accepts it at the start of their own session, on their own device. Acceptance is each map's Stage 1, so it is observed and written on the sheet, but it is not the card. The owner also enrolls a second factor here, because an Owner cannot reach a Practice screen without one. Then the founder attaches the participant to the card's Engagement, or sends the card's Offer, from his own device, since neither can exist before the participant's membership does. Then the person closes the browser, and the card starts from the device's home screen.

## The five task cards

One card per session. Each is the moment of truth the journey map names for that person's role, given as a goal and never as steps: it says what the person needs, and never which screen, tab or button gets them there. The founder reads the card aloud and hands it over on paper, so the person can read it again without asking.

The Client names on the cards are the seeded Practice's fictional Clients. They are named, never "her" or "him", by the same rule product copy follows ([Voice](../design/brief.md#voice)).

Each card lists the Budget of the stage it exercises, with the values its map gives. Each line becomes one Budget row of the [observer sheet](#the-observer-sheet), where the session records what it measured against it. If a map's Budget changes before a session, the map wins and the card is corrected.

### Card 1 — the owner

- **Map**: [`practice-owner.md`](../journeys/practice-owner.md), Stage 8, "Coverage, at 2 a.m. — moment of truth"
- **Moment of truth**: two Clients go into labor the same night, and the owner needs to know within a minute who is free.
- **Reached when**: the person names, from what DoulaCloud shows, a doula who is free to go to Rosa Lindqvist's birth.

> It's the middle of the night. June Ferris and Rosa Lindqvist have both gone into labor, and the doula on both births is already at June Ferris's. Find out who on your team is free to go to Rosa Lindqvist's birth.

**Budget**: Screens 2 · Decisions 2 · Memory yes · Time 400 ms, 100 ms · Confirmation yes · Cold yes · 320px yes

### Card 2 — the non-doula admin

- **Map**: [`non-doula-admin.md`](../journeys/non-doula-admin.md), Stage 9, "Record the Payment — moment of truth"
- **Moment of truth**: recording a Payment that arrived outside Stripe, which is the normal case for a small practice.
- **Reached when**: DoulaCloud shows Ada Walker's Invoice as paid by bank transfer, and the person can say where it shows that.

> Ada Walker paid the $1,200 balance by bank transfer this morning. Make sure DoulaCloud shows that the money came in.

**Budget**: Screens 2 · Decisions 4 · Memory yes · Time 400 ms, 100 ms · Confirmation yes · Cold yes · 320px yes

### Card 3 — a doula

- **Map**: [`employed-doula.md`](../journeys/employed-doula.md), Stage 6, "Read the Birth Plan — moment of truth"
- **Moment of truth**: the Birth Plan, on a phone, in a hospital corridor. The walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) moved the failure from the scroll to the way in, which is why the card starts from the home screen.
- **Reached when**: the person reads aloud, from the Birth Plan, what Maria Esposito wants for pain relief.

> Your client Maria Esposito is in labor, and you're in the hallway outside the room. The nurse asks what Maria wants for pain relief. Find the answer in DoulaCloud.

**Budget**: Screens 1 · Decisions 1 · Memory yes · Time 400 ms, 100 ms · Confirmation yes · Cold yes · 320px yes

### Card 4 — a doula

- **Map**: [`employed-doula.md`](../journeys/employed-doula.md), Stage 6, "Read the Birth Plan — moment of truth", the same moment as card 3 with a second person and a second Client.
- **Moment of truth**: as card 3.
- **Reached when**: the person reads aloud, from the Birth Plan, who Nia Thompson wants in the room.

> Your client Nia Thompson is in labor, and you've just walked into the hospital. The nurse at the desk asks who Nia wants in the room. Find the answer in DoulaCloud.

**Budget**: Screens 1 · Decisions 1 · Memory yes · Time 400 ms, 100 ms · Confirmation yes · Cold yes · 320px yes

### Card 5 — the contractor doula

- **Map**: [`contractor-doula.md`](../journeys/contractor-doula.md), Stage 3, "The offer — moment of truth"
- **Moment of truth**: the offer, before the contractor has said yes. It must say enough to take or refuse the job (who, when, on-call terms, fee) without opening the agency's book to someone who has agreed to nothing.
- **Reached when**: the person has decided, from what the offer shows and without calling anyone, and DoulaCloud has their answer.

> The agency has offered you a birth in February. Decide whether you'll take it, and give the agency your answer in DoulaCloud.

**Budget**: Screens 1 · Decisions 2 · Memory yes · Time 400 ms, 100 ms · Confirmation yes · Cold yes · 320px yes

Cards 1 to 5 are not an order. Sessions run in whatever order the five people can come.

## The observer sheet

One sheet per session, filled from the founder's notes and the recording on the day of the session, and filed at `docs/research/usability-sessions/<YYYY-MM-DD>-card-<n>.md`: the session's date and the card's number, and nothing that names the person.

**Two columns, and they never mix.** *What the product did* is the evaluator's register: third person, past tense, what was on screen and what happened, with the path and the recording's time. *What the person said* is the person's own words, inside quotation marks, verbatim from the recording, or empty. A paraphrase goes in neither column, and the founder's own reading of a moment goes in neither column either. It is the same split the friction log holds between Observed and Narrated ([`docs/simulation/README.md`](../simulation/README.md#the-two-registers)): what the product did is the evidence, and what the person made of it interprets a row and never stands on its own. An opinion ("this is nice", "I'd use this") is written down, because it was said, and it is never a finding.

The person's own words for things are worth the most. A doula who says "my client" where the screen says "Engagement" has given the map's Words table a row, and [Voice](../design/brief.md#voice) rule 7 puts the words a doula said in place of ours.

**Budget rows.** One row per Budget line of the card's stage, holding a number or a yes-or-no, never prose. A session on a person's own device cannot use DevTools, so each line is read as below, and what cannot be read is written "not measured", never estimated:

| Line | What the session records |
| --- | --- |
| **Screens** | Distinct DoulaCloud screens crossed, from the recording. Two numbers: from the home screen to the goal, and from the stage's own entry screen as the map defines it, which is the number the budget holds. |
| **Decisions** | The most fields, options and buttons that change what happens next, visible at once on one screen, counted from the recording. |
| **Memory** | `yes` if no later step needed something from an earlier screen that was no longer shown. `no` if the person went back to look, or said they had to remember it. |
| **Time** | The slowest routine act, from the input to the result settled on screen, read frame by frame off the recording, in milliseconds. The 100 ms acknowledgment is "not measured": a screen recording cannot show it honestly. |
| **Confirmation** | `yes` if the screen the card ends on says what happened and what is next, or, on a read, names whose it is and when it last changed. |
| **Cold** | `yes` if the person reached the goal with no help. The founder gives none, so a person who gave up is `no`. |
| **320px** | The device's width in CSS pixels, and `yes` if every screen was complete and usable at that width, with no sideways scroll and nothing cut off. On a device wider than 320px this does not measure the 320px line, and the row says so; the continuum sweep holds that line ([ADR-0025](../adr/0025-layout-is-verified-across-the-continuum.md)). |

**The sheet**, as it is filed:

```markdown
# Session <YYYY-MM-DD>, card <n>

- **Card**: <n>, <role>, [<map>.md](../../journeys/<map>.md) Stage <s>
- **Came to the session**: chosen by the owner / offered themselves
- **Recording**: yes / no
- **Device**: phone / tablet / computer, <width> CSS px wide, <browser>
- **Moment of truth reached**: yes / no

## Before the card

| What the product did | What the person said |
| --- | --- |
| <the invitation, the acceptance, the second factor: path, recording time, what happened> | "<verbatim>" |

## The card

| What the product did | What the person said |
| --- | --- |
| <one row per act: path, recording time, what was on screen, what happened> | "<verbatim>" |

## Budget

| Line, from the map | What the product did | What the person said |
| --- | --- | --- |
| **Screens**: <n> | <from the home screen>; <from the stage's entry> | |
| **Decisions**: <n> | <largest count> | |
| **Memory**: yes | yes / no | |
| **Time**: 400 ms, 100 ms | <slowest act> ms; 100 ms not measured | |
| **Confirmation**: yes | yes / no | |
| **Cold**: yes | yes / no | |
| **320px**: yes | <width> px: yes / no | |

## Debrief

| Question | What the person said |
| --- | --- |
| <the question as asked> | "<verbatim>" |

## Findings

| Row | Issue | Route |
| --- | --- | --- |
| <which row> | #<n> | new / sighting on an open issue / regression |

## Revised

<each persona or journey map this session falsified, and the commit that revised it, or "none">
```

Every card exercises a moment-of-truth stage, and a moment-of-truth stage carries all seven lines ([the Budget line](../journeys/README.md#the-budget-line)), so every sheet has all seven rows.

## The debrief

After the card, with the recording still on. The questions follow [_The Mom Test_](books/the-mom-test.md), which is on the values track and synthesized in [`who-it-is-for-and-how-the-product-learns.md`](books/themes/who-it-is-for-and-how-the-product-learns.md): ask about the person's life and not about the product, ask for a specific past case and not an opinion about the future, and talk less than they do (ch. 1, p. 13). Three questions per card, in the same shape: what they did last time, what it cost, and what they tried.

**Never asked**, in any form: whether they like it, whether they would use it, whether they would pay for it or how much, whether it is better than what they use now, or what they would add. Each one invites a polite answer, and a polite answer is narration, not evidence (ch. 1, p. 16; ch. 2, p. 25). When the person offers an opinion anyway, it goes on the sheet in quotation marks, and the next question asks for the last time it happened (ch. 2, pp. 30–31).

**Card 1, the owner**

1. Tell me about the last night two of your clients were in labor at the same time. How did you find out who could go?
2. What did that night cost you: the calls, the waiting, the sleep?
3. What have you tried, to make that easier? What happened to it?

**Card 2, the admin**

1. Think of the last payment that came in by check or bank transfer. Walk me through what you did with it.
2. How long did that take, and what happened the last time one got missed?
3. What have you used to keep track of who has paid? What did you stop using, and why?

**Cards 3 and 4, a doula**

1. Tell me about the last time someone at a hospital asked you what your client wanted. Where did you look?
2. What happened the last time you couldn't find it fast?
3. What have you tried, for keeping a client's birth plan with you? What happened to it?

**Card 5, the contractor**

1. Tell me about the last birth an agency offered you. How did the offer reach you, and what did you need to know before you said yes?
2. What did it cost you, the last time a job turned out different from what you were told?
3. What have you used to keep track of what you agreed to on a job?

A follow-up is allowed and is always the same move: back to the specific case. "When was that?", "What did you do next?", "Who did you call?"

## After the session

### Findings go through the #766 pipeline

What a session finds reaches the tracker the way a simulation run's does ([`docs/simulation/findings.md`](../simulation/findings.md), settled on [#766](https://github.com/markgoho/doula-cloud/issues/766)), in one filing pass over the sheet after the session, never during it.

- **A finding is a row in the product column.** A row where the product did the wrong thing, did not exist, or carried the person through at a cost. A row with only words in it is not a finding, and neither is an opinion. A Budget row that missed its number is a finding.
- **Three labels, and no fourth.** `journey-gap` for a capability that does not exist, `bug` for something built that did the wrong thing, `enhancement` for something that carried the person through and cost them. A Budget row that missed goes to the map that owns the stage as a `journey-gap`, with the budget line quoted, as [`docs/test-plans/README.md`](../test-plans/README.md#budget-steps) says. Each finding also carries the `journey:<persona>` label of the map its card came from (`journey:renata-alvarez`, `journey:dee-whitlock`, `journey:priya-raman`, `journey:lena-vasquez`).
- **One finding, one issue, deduplicated.** Two rows are one finding when the same change would answer both. Before anything is filed, the filer reads the Gaps found table of the card's stage on its map, then searches the open and closed tracker. An open issue that already has it gets a comment naming the session, and nothing new is filed. A closed issue that the session met again becomes a new `bug` named as a regression, linking the closed one. Only what nothing owns is a new issue.
- **The session is named as a sighting**: `usability <YYYY-MM-DD> · card <n> · <row>`, with a link to the sheet. Two doulas meeting the same thing on cards 3 and 4 are two sightings on one issue.
- **Every anchor is re-made from the deployed app**, because the recording is deleted: a screenshot of the seeded Practice's page, an HTTP exchange, or a `file:line`. Nothing from the person's device is committed.
- **No ranking.** No severity, no priority, and no order between findings. Pre-launch, everything found is fixed. The number of sessions that met a finding is a fact inside the issue and never a label, a field, or a title.
- **Every issue carries an acceptance-criteria checklist**, Status `Needs triage`, and is listed on #1685.

### A session can falsify a persona or a map

The personas and maps are hypotheses, and [`docs/personas/README.md`](../personas/README.md) says that where real evidence contradicts a persona, the persona is wrong and not the finding. When a session contradicts a claim in a persona file or a journey map (a moment of truth that was not the moment, a word the person does not use, a Thinking line the person's own words disprove), that file is revised in its own commit, and the revision names the session: `usability <YYYY-MM-DD> · card <n>`, linking the sheet. The sheet's Revised section names the commit.

### What #1685 records at the end

A resolution comment on #1685, one line per session: the card, the map, and whether the person reached the moment of truth.
