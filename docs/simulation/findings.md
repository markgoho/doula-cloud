# From friction log to filed finding

How a **Run**'s **Friction logs** become filed, deduplicated issues on the tracker. Settled on [#766](https://github.com/markgoho/doula-cloud/issues/766), under the map [Six months in a sandbox](https://github.com/markgoho/doula-cloud/issues/759).

[The friction log](README.md) is the instrument, and it stops at the log: *the log is the input to filing, never the filing itself*. This file is what happens next.

## Why this file exists

A Run walks about 1,526 acts across a cast of twenty-two over twenty-six simulated weeks ([the Calendar](calendar.md)). The map's destination requires what it exposes to be **filed and deduplicated**, and left unmanaged that ends one of two ways: several hundred near-identical issues nobody reads, or one summary paragraph that loses every finding it mentions.

Both failures come from the same missing distinction — between the **defect**, which is one thing however many people met it, and the **encounter**, which is one person at one moment. The log records encounters. The tracker holds defects. This file is the join.

## A Sighting and a Finding are different things

**A Sighting is one encounter**: `(run id, persona, entry id)`. Nothing else identifies it, and nothing else needs to. Entry ids are journey step ids ([the friction log](README.md#the-unit-one-act-against-one-step)), so `2027-01-04-1 · practice-owner · 3.2` names an act precisely enough that a reader can open the log and see it.

**A Finding is one defect**, and its identity is **the GitHub issue number**. There is no other id. A Run never mints a `-G` gap id — `README.md` forbids it, and the journey map that owns a stage stays the only thing that mints one. So the tracker carries two shapes of `journey-gap` issue, one with a gap id from the nine walks and one without from a Run, and that is intended: the id says which instrument found it, not how important it is.

**Two Sightings are the same Finding when the same act against the same surface would have to change to fix both.** Not "the same screen", not "the same feeling" — the same change. Fourteen doulas meeting a slow Client list is one Finding, because one query fixes all fourteen. Two doulas meeting the same 500 from different endpoints is two Findings if the endpoints have different causes, and one if the anchor shows one cause. Where the log cannot tell, the anchors decide: entries carrying the same `file:line`, or the same method-and-path with the same status, are one Finding; entries whose only common ground is a screenshot of the same page are two until something joins them.

A Finding carries **every** Sighting of it. Eleven encounters over six months is eleven lines in one issue, not eleven issues and not one issue that mentions the first.

## Filing happens after the run, in one pass

**No issue is filed while a Run is walking.** The filing pass reads the nine committed logs, plus [the Extras log](#the-extras-log), after the last act, and files everything at once.

This corrects the map's own earlier Note, which said a persona agent "writes it down, files it, and keeps walking". Three things settle it the other way:

- **Concurrency makes during-run dedup impossible.** Twenty-two cast members are interleaved on one clock, and no persona session can see another's freshly-filed issues. Two people meeting the same slow list in the same simulated week each open an issue for it, and the dedup job then runs over issues that already have numbers, labels, and possibly comments.
- **There is no hot context to lose.** The usual argument for filing in the moment is that the detail fades. `README.md` already removes that: **an entry that cannot meet the anchor bar is deleted, not softened**. Everything a filer needs is in the entry, or the entry does not exist.
- **Dedup wants the whole log.** "This hurt eleven times" is only knowable once all eleven are written down. Filing at the first Sighting files a Finding that does not yet know what it is.

The filing pass is a session of its own, after the Run closes and its logs are committed. It writes issues; it never edits a log.

**One exception, and it is the map's, not this file's.** A defect that makes the rest of the Run unobservable is fixed on the spot, on its own branch and PR. That is a product change during a Run and it is deliberately narrow. Its issue is still filed in the after pass like any other, and closed by the PR that already fixed it — so the Run's record of what it found stays complete.

## The four routes out of a log

Every entry in a Run's logs takes exactly one of these. The pass walks the logs in order and routes each entry; the routing is what the [findings file](#the-findings-file) records.

### 1. It stays in the log

The default, and by volume the largest route. An entry with outcome `completed` files nothing. An entry with friction that an existing Finding already covers adds a Sighting to that Finding and files nothing new. A `u`-numbered entry is a fact about the harness and goes to the run README, never to the tracker.

### 2. The tracker already has it, open — comment, never file

**How the join is made.** A Run's entry ids are journey step ids, and the 58 gap issues from the nine walks each carry an **Exposed by these test-plan steps** line naming persona and step (see [#262](https://github.com/markgoho/doula-cloud/issues/262)). So a Sighting at `practice-owner 3.2` lands on the gap issues whose exposure line names Renata 3.2, and the filer reads those before writing anything. Where the entry is `x`-numbered — off-map, which is where a Run earns its keep — there is no step id to join on and the filer searches the tracker on the surface and the anchor instead.

The Run then **comments once on that issue**, with every Sighting of it in the Run, and nothing else. This is the "known gap hurt eleven times and here is what it cost" case, and it is new information about an old issue rather than a new issue. One comment per known Finding per Run, batched at the end of the pass, so an issue met eleven times gets one notification and not eleven.

A Run may also **narrow or re-mark** an existing gap: the Run settles what the code does, and re-marking is not minting. The wording is corrected on the journey map that owns the gap, exactly as the nine walks did with TB-G7 and PR-G3, and the comment says so.

### 3. The tracker had it, closed — a new `bug`, named as a regression

A closed issue is a claim that the defect is gone. A Run that meets it again has falsified that claim, and reopening loses the history of the first fix. So it is a **new `bug`**, whose body links the closed issue and states plainly that the Run met it again, with the Sighting and its anchor. The closed issue gets one comment pointing at the new one.

### 4. Nothing owns it — a new Finding

The residue: the entry is anchored, no open issue covers it, no closed issue was falsified. This is a new issue, shaped as below. Most of these come from the `x`-numbered entries and from the acts a single-instant walk against a fixture could not reach — a second person on the same object, an Engagement six months old, a book of 58 rather than three.

## What never becomes a Finding

The log's own admissibility rules do most of this work before filing starts, and the pass adds nothing to them. Restated so the filer does not have to infer it:

- **Anything struck from the log.** `README.md`'s [inadmissible claims](README.md#claims-that-are-never-admissible) — notification latency, an email's journey, "this was fast enough", any claim about users, an unanchored claim about the code, a bare frequency — never reach the tracker, because they never reach the log.
- **A Narrated block on its own.** Narration interprets an Observed act; it is not evidence of one. A Finding whose only support is how a persona felt about a screen is not filed, and the entry it came from was already struck.
- **A repeat Sighting.** It goes on the Finding it belongs to.
- **A timing in the 1–10 s band.** `README.md` gives that band a narrated line and nothing else, and gives only **over 10 s** the "filed as a finding — no discretion" treatment. The silence is the decision, and the filer does not override it in either direction: a 9 s act is not filed because it felt bad, and a 12 s act is filed even if it did not.
- **A refusal at the right boundary.** `refused` is the product working. What can be filed is the *message*: a refusal the persona could not read is a Finding about the error, and the entry that supports it is the Narrated block under a `refused` act.

## Which label, and what the issue holds

**Three labels, and no fourth.** A UX improvement is not a separate kind of thing on this tracker:

| The defect | Label | Test |
| --- | --- | --- |
| A capability a person needs on a journey does not exist | `journey-gap` | The person cannot get through, and no screen or endpoint would let them |
| Something is built and does not work | `bug` | It exists, and it did the wrong thing |
| Something works and should be better | `enhancement` | It carried her through, and it cost her |

A **GOV.UK pattern departure** routes by [ADR-0021](../adr/0021-govuk-is-the-reference-for-service-patterns.md), which already decides this: *"a departure with no reason is the only kind of GOV.UK finding this project treats as a defect."* So an unrecorded departure is a `bug`; a recorded one is not a Finding at all; a screen answered from the ADR's source order is not a Finding either.

Every Finding also carries **`journey:<persona>`** for each Persona who sighted it — the nine labels already exist and already mean "noticed via this person". A Finding sighted by an Extra carries no journey label, because no journey map owns her.

**The body**, in this order:

1. **What the Run met**, in one or two sentences, in the Observed register. Never softened, and never a persona's voice.
2. **Sightings** — every one, as `<persona> · <entry id> · <simulated date>`, each with its anchor and its timing, linking the log file in the run directory.
3. **The Run** — run id, product commit SHA, and a link to the run README, so a reader can tell what world it was met in.
4. **Acceptance criteria** — a checklist, always. The close hook enforces it, and a Finding with no AC cannot be closed.
5. The standing footer the nine walks already use: the project is pre-launch, so this carries no severity and no priority.

## The findings file

The filing pass writes one file, committed with the Run:

```
docs/simulation/runs/2027-01-04-1/
├── README.md          ← the run's own header, and its harness accounting
├── findings.md        ← what was filed, and which Sightings each Finding holds
├── extras.md          ← the Extras' observed acts
├── practice-owner.md  ← one log per Persona
└── …
```

`findings.md` is a table, one row per Finding, holding the issue number, the route it took (new, comment, regression), and every Sighting. It is the only place the issue-to-Sighting mapping lives in full, and it is why run one and run three can be compared at all: `diff`ing two run directories shows which Findings are gone, which are new, and which are the same Finding met fewer times.

**A log is never edited after the Run closes.** The pass adds `findings.md` beside the logs; it does not go back and write issue numbers into entries. A log is a record of a walk, and a walk did not know an issue number.

### The Extras log

[The World](worlds/rooted-birth-collective.md) walks eleven Extras through their door acts and one stated reason each to open the app, and an Extra has **no Friction log in her voice** — that is what makes her an Extra. She is still observable, and a broken invitation acceptance is a defect whoever met it.

So Extras' walked acts go in one shared `extras.md` per Run, carrying **Observed blocks only** — the five mandatory fields, the anchor, the timing, the four outcomes, and no Narrated register at any outcome. An Extra lacks an interior life, not a screen. Her entries route through the four routes like anyone's, and a Finding sighted only by Extras is filed like any other, minus the `journey:` label.

## Mechanics

A Run files on the order of dozens of issues in one pass, plus comments on existing ones. That is enough to trip GitHub's **secondary** rate limit, which does not appear in `gh api rate_limit` and shows up as a refusal mid-loop.

- **Pace the writes.** Space issue creations and comments rather than firing them in a tight loop, and never run the pass as an unattended burst.
- **Use the node-id form for Project field writes.** Resolve the field and option ids once with `gh project field-list` / `gh project item-list`, then `gh project item-edit --id … --field-id … --single-select-option-id …`. `docs/agents/issue-tracker.md` carries the exact invocations, and they are not restated here.
- **Status is `Needs triage`**, matching every one of the 58 gaps the nine walks filed. A Run observes; it does not triage its own output. **Target date is left unset**, which [#653](https://github.com/markgoho/doula-cloud/issues/653) already requires of anything not in an eligible Status.
- **One parent per Run.** Findings are native GitHub sub-issues of a per-Run parent issue, the same shape [#328](https://github.com/markgoho/doula-cloud/issues/328) gave the first journey run. A body reference is not a substitute — it does not show in the sub-issue progress bar.

## No ranking, and the count that is not one

`CLAUDE.md` is unambiguous: pre-launch, everything found is fixed, and only dependency order is real. So the pipeline emits **no severity, no priority, and no ordering by pain**.

The Sighting count survives that rule because of what it is. "Met eleven times in six months" is a **fact about the Run** — how often that world put a person in front of that defect — not a claim that it matters more than a Finding met once. Two guards keep it from becoming a ranking by the back door:

- **`findings.md` and the run README list Findings in first-met order**, by simulated date. Never sorted by count, and never grouped into tiers.
- **The count lives inside the issue**, in its Sightings list, where it reads as evidence. It never becomes a label, a Project field, or a number in a title.

A Finding met once and a Finding met eleven times are both fixed before January.

## What the filing pass never does

- **Mint a gap id.** `README.md:146` fixes this: one gap, one id, owned by the journey map that found it. A Run cites and never mints beside.
- **Edit a journey map, except to narrow.** The one licensed edit is correcting the wording of a gap the Run settled — route 2 above.
- **Soften a Finding.** Inherited from [#203](https://github.com/markgoho/doula-cloud/issues/203) and it cuts both ways: it clears the bar and says plainly what happened, or it does not exist.
- **Close anything.** A Run reports. The one issue it may see closed is the mid-run unobservability fix, and the PR closes that, not the pass.
- **Re-run the nine test plans.** [Second run: walk all nine journeys again (#329)](https://github.com/markgoho/doula-cloud/issues/329) is a different instrument with a different question — whether the nine plans complete against a fixed product — and it mints gap ids where a Run does not. Neither supersedes the other.
