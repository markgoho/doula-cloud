# The friction log

What a simulation run emits, and what an entry must carry to be believed. Settled on [#760](https://github.com/markgoho/doula-cloud/issues/760), under the map [Six months in a sandbox](https://github.com/markgoho/doula-cloud/issues/759).

This file is the instrument, and it stops at the log. What happens to the log afterwards — how a **Sighting** becomes a filed, deduplicated **Finding** — is [findings.md](findings.md), settled on [#766](https://github.com/markgoho/doula-cloud/issues/766). Who is walking, and what they walked into, is the **World** — [Rooted Birth Collective](worlds/rooted-birth-collective.md), settled on [#761](https://github.com/markgoho/doula-cloud/issues/761). Where the walking happens, and the list of things that cannot be seen there, is the **sandbox** — [Where a run runs](environment.md), settled on [#763](https://github.com/markgoho/doula-cloud/issues/763). How much work there is, when it arrives and what goes wrong is the **calendar** — [Six months of an agency's work](calendar.md), settled on [#765](https://github.com/markgoho/doula-cloud/issues/765).

## Why this file exists first

An agent told to keep a friction log **will produce a friction log** — fluent, in character, well argued, and possibly about a screen it never struggled with. Left unconstrained, a run emits a large volume of confident prose whose relationship to the product is unknown, in a format that reads exactly like a diary. `docs/personas/README.md` already forbids citing the nine proto-personas as user research; that line gets much harder to hold once the output reads like lived experience.

So the instrument is settled before anything is built with it. Everything below exists to make one distinction survive contact with a reader: **what the product did**, which is measured and carries evidence, versus **what the persona made of it**, which is narration and carries none. Both are useful. Conflating them destroys the log.

**A friction log is heuristic evaluation, never user research.** No number in it describes users. It describes one scripted agent walking one seeded world once.

## Where a log lives

Committed to the repo, one directory per run:

```
docs/simulation/
├── README.md                       ← this file, the format
├── calendar.md                     ← the counts, the calendar, and what goes wrong
├── findings.md                     ← how a log becomes filed, deduplicated issues
├── worlds/
│   └── rooted-birth-collective.md  ← the World a run walks: the cast and what they arrive from
└── runs/
    └── 2027-01-04-1/
        ├── README.md               ← the run's own header (see below)
        ├── findings.md             ← what was filed, and the sightings each finding holds
        ├── extras.md               ← the Extras' observed acts, no narrated register
        ├── practice-owner.md       ← one log per persona, slug matching docs/personas/
        ├── employed-doula.md
        └── …
```

The run id is `YYYY-MM-DD-<n>`, dated by the day the run **started** in real time, with `<n>` distinguishing two runs begun on one day. A persona's log file keeps the persona's slug, so `docs/personas/practice-owner.md`, `docs/journeys/practice-owner.md`, `docs/test-plans/practice-owner.md` and a run's `practice-owner.md` all name the same person.

Every run is kept. The map calls itself a standing harness, and a run's whole point is to be compared against the run before it (see [Comparing one run against the next](#comparing-one-run-against-the-next)); a log that is overwritten cannot be opened beside its predecessor. The precedent is the dated **Run log** each test plan already carries.

### The run README

One file per run, written before the first act and closed after the last. It carries what makes the run reproducible and what makes its numbers readable: the seed, the product commit SHA the run walked, the simulated start and end dates, the jump schedule, the cast that was in play, the harness configuration, and — at the end — the entry counts and the findings filed. Anything a later reader needs to know to judge whether two runs are comparable belongs here, not in a persona's log.

## The unit: one act against one step

**An entry is one act by one persona against one journey step.** Not a session, not a stage, not a moment the agent found interesting.

An entry's id **is** the journey step's id, exactly as `docs/test-plans/README.md` already does it: `3.2` in Renata's log is Renata 3.2 in `docs/journeys/practice-owner.md`, so a journey map, a test plan and a run log read side by side. A run never renumbers a map.

Three ids do not come from a map, and each is marked so a reader can see it:

- **`3.2-a`** — an extra act inside a step the map does not name, appended exactly as a test plan appends a check.
- **`x1`, `x2`, …** — an act that belongs to no step on the map at all: the persona improvised, or the six months threw something at her the map never anticipated. These are the most interesting entries in a run, because they are what a single-instant walk against a fixture structurally could not see. They are numbered per persona per run and carry a plain-language note saying what she was trying to do.
- **`u1`, `u2`, …** — an act that happened but could not be observed, because the harness dropped the screenshot or lost the network log. These are a fact about the **harness**, never about the product, and they are numbered separately from `x` for exactly that reason: an improvised act and a failed capture are opposite kinds of thing, and a run whose harness failures were counted as improvisation reads as more interesting than it was. Every `u` entry also goes in the run README, where the harness is accounted for.

The same step walked twice in six months is two entries. An entry is an act, not a step.

## The two registers

Every entry has an **Observed** block. Some entries also have a **Narrated** block. They are two labelled blocks in the format, so telling them apart is never a reader's judgement call.

### Observed — the evaluator's register

Third person, past tense, no interiority. It says what was done and what the product did, and every claim in it is anchored. It carries five fields, and all five are mandatory:

| Field | What it holds |
| --- | --- |
| **Act** | What was done, through which control, on which URL. `Clicked "Send invitation" on /practices/{id}/staff/invite`. |
| **Result** | What the product did, as built. A refusal, an error, a raw enum on screen, or the thing working — whatever actually happened. |
| **Outcome** | One of four values, below. |
| **Timing** | Milliseconds, machine-captured. Always present, never estimated. See [Performance](#performance-is-an-entry-with-a-number-on-it). |
| **Evidence** | At least one anchor. See [Admissibility](#admissibility). |

**The four outcomes.** Unlike the four *marks* in `docs/test-plans/README.md`, which are claims read out of the code before a walk, these are observations recorded during one:

- **`completed`** — she did the thing she was trying to do, and nothing cost her.
- **`completed with friction`** — she did the thing, but it cost her: a retry, a guess, a back-navigation, a wait over 1 s, a second screen she had to visit to find out whether the first one worked.
- **`refused`** — the product deliberately said no. This is not a defect on its own; a refusal at the right boundary is the product working. The Narrated block is where it is said whether the refusal was intelligible to her.
- **`stuck`** — she could not do it at all. No screen, no control, no endpoint, or an error she could not get past.

An outcome of `completed with friction`, `refused` or `stuck` **requires** a Narrated block. `completed` forbids one — if there was something to say, the outcome was not `completed`.

### Narrated — the persona's register

First person, present tense, in the persona's own voice, and **short**: what she was trying to do, what she thought was happening, and what she did next. Read the persona file before writing a word in her voice.

A Narrated block carries no evidence and asserts nothing about the code. It is subordinate to its Observed block: it may only interpret an act that block records. There is no such thing as a free-standing Narrated entry, and a Narrated block that describes an act with no measured Observed block above it is **struck**, not softened.

Narration is the exception, not the running commentary — see [Silence](#silence-and-the-line-that-breaks-it).

## Admissibility

**Every Observed claim carries an anchor.** This is #203's rule, inherited by the map and extended with the two anchors a live run gets for free. One of these five, minimum:

1. A **screenshot**, committed beside the log at `runs/<run-id>/shots/<persona>-<entry-id>.png`.
2. A **`file:line`** into this repo.
3. An **HTTP exchange** — method, path, status, and the response body where it is short (`POST /api/practices/{id}/invitations → 409 conflict`).
4. A **schema reference** — table and column.
5. A **timing**, machine-captured (this one is always present anyway, so it never stands alone as the only anchor for a claim about behaviour — it anchors a claim about speed).

**An entry that cannot meet this is deleted, not weakened.** It is not rewritten as a hedge, not moved to a "possible issues" section, and not kept with a note that evidence was unavailable. A run that emits fewer, anchored entries is worth more than a run that emits many, and the deletion is the mechanism that keeps that true. Where an act genuinely could not be captured — the harness dropped the screenshot, the network log was lost — the honest record is a **`u`-numbered entry saying the act was not observable**, which is a fact about the harness and belongs in the run README too.

**An email is an artefact, and what it says is admissible.** A persona opens her mail in a browser at the sandbox mailbox ([environment.md](environment.md#how-a-persona-receives-her-email)) and follows the link out of the message, so the message page screenshots like any other screen and anchors a claim the same way. A wrong subject line, a link that goes nowhere, a body addressing her as `[object Object]`, a Reply-To that is `noreply` where ADR-0011 says it should be monitored — each is an ordinary entry with an anchor. What is *not* an artefact is anything about the message's journey: see the next section.

**Never soften a finding.** Inherited from #203 and it cuts both ways: an entry either clears the bar and says plainly what happened, or it does not exist.

## Performance is an entry with a number on it

`CLAUDE.md` makes performance under a real Practice's data a cross-cutting expectation, and a run is the first thing that will ever produce that data. So observed slowness is an ordinary entry, in her voice, with a number attached — not a separate performance report.

**Every act carries a timing, whether or not she noticed.** Recording only where the agent complains loses the number exactly where it is most useful: the act that was slow and went unremarked. The number is machine-captured by Playwright — the milliseconds from the interaction that triggers the act until the assertion that the resulting view is ready — never the agent's impression of how long something felt.

**Three bands decide what happens next, so no agent judges it.** They are [Nielsen's response-time limits](https://www.nngroup.com/articles/response-times-3-important-limits/), which is why the numbers are what they are:

| Timing | What it does to the entry |
| --- | --- |
| **under 1 s** | Recorded in the Timing field. Nothing else. The outcome stays `completed`. |
| **1 s – 10 s** | Outcome becomes `completed with friction`. A Narrated line is required, and it names the wait in her own words. |
| **over 10 s** | Outcome becomes `completed with friction`, a Narrated line is required, and the entry is filed as a finding — no discretion. Ten seconds is the limit past which a person stops attending to the task. |

The band is applied to the measurement, not argued with. An agent that thinks a 4 s wait was fine still records `completed with friction`.

**The timing is UI response time and nothing else.** It never measures how long an email or a notification took to arrive — see the next section.

**Every timing is a lower bound.** The BFF, Postgres and the browser share one machine, with no network between them, no CDN, no Cloud Run cold start and no Cloud SQL hop ([Where a run runs](environment.md#every-timing-is-a-lower-bound-and-the-bands-do-not-move)). So a run can prove an act **too slow** and can never prove one **fast enough**: an act inside the 1 s band locally is unproven in production, and a slow local reading is worse news than the same number would be from a deployed service.

## Claims that are never admissible

These are struck on sight, however well written, and no anchor rescues them. They are named here rather than left to a session's memory, because a session will not remember them.

- **"The notification arrived too late."** A run cannot see notification latency, and must not pretend otherwise. [#762](https://github.com/markgoho/doula-cloud/issues/762) settled why: nothing fires by itself locally — `tasknudge.FakeEnqueuer` records a nudge and issues no request — so the harness POSTs the thirteen `process-*` endpoints itself after each jump. That reproduces what Cloud Scheduler does deployed: [#481](https://github.com/markgoho/doula-cloud/issues/481) put all thirteen behind one `process-outbox-drain` job, so the harness's own drain loop and the job now run the same set. It still flattens the difference between a nudged notification arriving in seconds and an un-nudged one waiting for the scheduler's cadence. **Whether** a notification arrived, and **what it said**, are both observable and admissible. **When** it arrived is an artefact of the harness. Any entry claiming otherwise is measuring the drain loop.
- **Anything about an email's journey.** "It went to spam", "it took five minutes to arrive", "the sender looked untrustworthy to a mail client", "it rendered badly in Outlook". Mail in a run never leaves the machine: the sandbox mailbox holds it the instant the worker posts it, and there is no provider, no receiving domain and no mail client between the two ([environment.md](environment.md#how-a-persona-receives-her-email)). Deliverability is real and it is worth knowing, and a run is not the instrument that measures it. What the message *says* is fully admissible — see [Admissibility](#admissibility).
- **"This was fast enough."** A timing under 1 s is recorded and nothing more; it is never evidence that the act is quick in production. Every timing is a local lower bound, so only the slow readings carry outward. An entry that reports a fast act as a passing performance result is struck.
- **Anything about users.** "Doulas would find this confusing", "most people expect", "this is a common pattern in this market". Nine proto-personas are hypotheses to be falsified, not a sample. A run's output looks like a diary and must never be cited as one.
- **A claim about the code with no `file:line`.** "The endpoint probably validates this" is not an entry.
- **A minted gap ID.** See below.
- **A frequency or a rate.** "This happened most of the time" — say how many times out of how many, or say nothing. The entries are all there; count them.

## Silence, and the line that breaks it

**Narration only where there was friction.** An entry that went smoothly carries its Observed block and stops. Four hundred first-person sentences about screens nobody struggled with is a diary; a handful about the ones that cost her something is a finding.

Silence is therefore load-bearing, and it has one failure mode: an unremarkable stage and a stage the agent skipped look identical. So **every stage closes with one line** saying which it was:

> **Stage 4 — Assign the Doula to Engagements.** 11 acts, 0 with friction. Nothing to report.

That line is mandatory, it is written per stage per persona, and it carries the act count so the reader can see the stage was actually walked. Nine lines a run, against four hundred that would otherwise be there.

## An entry never mints a gap ID

`docs/test-plans/README.md` fixes the rule for a plan; it holds for a run. **One gap, one ID**, owned by the journey map that found it, and a run cites what exists rather than minting beside it. The nine walks already found 58 gaps; a run that re-mints them buries the tracker.

Where a run exposes something no map owns, the entry says so plainly and stops there. Turning that into a filed, deduplicated finding is the pipeline's job, and the pipeline is [findings.md](findings.md), settled on [#766](https://github.com/markgoho/doula-cloud/issues/766). **The log is the input to filing, never the filing itself** — and filing happens in one pass *after* the run closes, so nothing in a log is written with an issue number in it.

A run may also **re-mark or narrow an existing claim** — the run settles what the code does, and re-marking is not minting. The gap's wording is corrected on the map that owns it.

## Fixed structure of a persona log

1. **Header** — persona link, journey link, run id, the product commit SHA, and what this persona was doing in this run.
2. **Stages** — one section per journey stage, in map order, each holding its entries and closing with its mandatory stage line.
3. **Off-map acts** — the `x`-numbered entries, which by definition belong to no stage, and the `u`-numbered ones, which belong to the harness.
4. **Counts** — entries by outcome, and the acts that could not be observed.

Entries are tables where they fit and blocks where the Narrated register makes a table unreadable. The Narrated block always sits directly under the Observed block it interprets, indented as a blockquote so no reader can mistake one for the other.

## Worked example

Renata Alvarez, walking [Stage 2 — Invite a new Doula](../journeys/practice-owner.md), in simulated March.

**This example is illustrative and is not a record of a walk.** No run has happened yet, so nothing below is a claim about what the product does. The statuses and timings are shaped like real ones so the format is legible, and the one `file:line` is deliberately written as `<package>/<file>.go:<line>` rather than a real path — a document whose central rule is that an unanchored code claim gets struck must not illustrate the anchor with a citation nobody checked. The first real log replaces all of it.

> ### Stage 2 — Invite a new Doula
>
> **2.1** — `completed` · 240 ms
> **Act**: Clicked **Invite Staff** on `/practices/{id}/staff`.
> **Result**: The invitation form rendered with Email, Roles and Employment type.
> **Evidence**: `shots/practice-owner-2.1.png`
>
> **2.2** — `completed with friction` · 3,910 ms
> **Act**: Submitted the form for `jo@rootedbirth.example` with the Doula role, employment type `contractor`.
> **Result**: `POST /api/practices/{id}/invitations → 201 created`. The roster returned and showed the invitation as pending. The wait fell in the 1–10 s band.
> **Evidence**: `POST /api/practices/{id}/invitations → 201`; `shots/practice-owner-2.2.png`
>
> > I hit send and then sat there. It's a form with three fields — I couldn't tell whether it had gone through or whether I'd need to do it again, and doing it twice is exactly the sort of thing that ends up with Jo getting two emails and asking me which one is real.
>
> **2.2-a** — `refused` · 180 ms
> **Act**: Submitted the same form again for the same address, to see what a duplicate does.
> **Result**: `POST /api/practices/{id}/invitations → 409 conflict`, rendered as "Something went wrong" with no field named.
> **Evidence**: `POST /api/practices/{id}/invitations → 409`; `api/internal/<package>/<file>.go:<line>`; `shots/practice-owner-2.2-a.png`
>
> > It stopped me, which is right, but it didn't tell me it was because I'd already invited her. I don't know if the problem is the email address or something else, so now I'm going to go and look at the roster to work out what I actually did.
>
> **Stage 2 — Invite a new Doula.** 4 acts, 2 with friction.

The 2.2 entry shows the performance band doing its work: the agent did not decide 3.9 s was worth mentioning, the band did. The 2.2-a entry shows a `refused` outcome that is not a defect in the refusal and is a defect in the message.

## Comparing one run against the next

Entry ids are journey step ids, and journey step ids are stable across runs. So run three's `practice-owner.md` and run one's line up on the ids, and an ordinary `diff` between two run directories reads as: which steps got faster, which outcomes moved from `stuck` to `completed`, which stage lines went from "3 with friction" to "nothing to report", and which `x`-numbered acts are new.

Three rules keep that diff readable:

- **Walk the stages in map order**, always, even where a persona could plausibly do them in another order. A reordered log diffs as a rewrite.
- **The timing bands never move.** A band change would silently reclassify every entry in every past run.
- **Two runs are comparable only if their run READMEs say so** — same seed, same cast, same jump schedule. A run that changed the world's shape is a new baseline, and its README says which run it supersedes rather than pretending to be a delta.
