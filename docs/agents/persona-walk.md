# Persona walk: a blind walker and a sighted evaluator

The recipe the main session follows to walk one persona through the running local stack. Two agents share the work ([#2352](https://github.com/markgoho/doula-cloud/issues/2352)):

- **`persona-walker`** (`.claude/agents/persona-walker.md`) has browser tools only. It cannot read the code, the journey maps or the test plans, so it finds its way by the screen alone, as the persona would. It returns a raw act log with a timing it measured and a screenshot for every act.
- **`walk-evaluator`** (`.claude/agents/walk-evaluator.md`) has the repo. It turns the act log into a friction log in the format of [`docs/simulation/README.md`](../simulation/README.md).

The split is the point. A walker that has read `app/src` or a test plan already knows where the button is, and every walk it takes is contaminated without anyone seeing it. The brief below is the walker's whole knowledge, so it is built to carry what the persona knows and nothing the code says.

A persona's skill and patience live in what her brief says she does, never in a cheaper model: both agents run on Opus, and the model is passed on each spawn. The output is heuristic evaluation, never user research ([`docs/personas/README.md`](../personas/README.md)).

## 1. Start the stack

Start `dev:full` from the worktree as [`docs/testing/walk-a-screen-locally.md`](../testing/walk-a-screen-locally.md) says, in the background. Note the dev origin and the sandbox mailbox URL it prints. Put `SEED_DEMO=0` in front for a persona whose journey starts from an empty product (Maya Okonkwo and Tasha Bell sign up cold); leave the seed on for anyone who works inside Rooted Birth Collective, and take her account from that doc's table.

A Client persona signs in to the portal by magic link, so her brief gives `/portal/login` as the start URL and the mailbox URL, and she asks for the link and follows it herself. Of the three, only Camille Boyd has a seeded account. Hannah Sorensen and Nadia Haddad each need a Practice that has added her as a Client first. Make that through a Staff walk or the seed, and give her only her email, never a link lifted from the seed's output.

Record `git rev-parse HEAD`: the evaluator needs the walked SHA.

## 2. Choose the conditions profile

One profile per walk, held for the whole walk, and named in the walker's brief and in the log header. Default is the first row of each.

| Condition | Values | `emulate` / how |
| --- | --- | --- |
| Viewport | `320x640x2,mobile,touch` (ADR-0024's floor), `390x844x3,mobile,touch`, `1280x800x1` | `viewport` |
| Network | none, `Slow 4G`, `Slow 3G`, `Offline` partway (ADR-0059) | `networkConditions` |
| CPU | 1, 4 | `cpuThrottlingRate` |
| Input | pointer, keyboard only, screen reader (the accessibility snapshot only) | stated in the brief; the walker keeps to it |
| Display | none, `prefersReducedMotion: reduce`, `forcedColors: active` | `prefersReducedMotion`, `forcedColors` |

Choose from what the persona file says about her, and quote the line that settles it: Maya "is on call. Anything that needs a laptop and twenty quiet minutes will not get done" is a phone walk. A throttled walk's timings still take the 1 s and 10 s bands, and are compared only with a walk under the same profile.

Timezone is not a condition a walk can set: `emulate` has no timezone option, so the walking browser reports the machine's zone, UTC on the dev box. A value the product pre-fills from the browser's timezone, such as the one on `/signup/where`, is the harness's zone, not the persona's. The evaluator records it and never grades it.

## 3. Build the brief

Run each of these and paste its output into the brief under its own heading. `<slug>` is the persona's file slug.

- **Who you are**, the persona file without its `Watch for` section, which by rule holds the claims about the code:

  ```sh
  sed '/^## Watch for/,$d' docs/personas/<slug>.md
  ```

- **Your words**, the journey map's Words table:

  ```sh
  sed -n '/^## Words/,/^## Stages/p' docs/journeys/<slug>.md
  ```

- **Your stages**, each stage's heading and its Thinking line, and nothing else from the map. Steps, Budget, Pain points and the moment of truth are what the walk is checking, so they stay out:

  ```sh
  sed -n -E '/^### Stage/p; s/^(\*\*Thinking\*\*: .*) \*\*Pain points\*\*.*/\1/p' docs/journeys/<slug>.md
  ```

  Count the stages printed against the map's `### Stage` headings; a Thinking line written differently does not print, and is added by hand.

Then add, in plain lines: the persona slug, which names her shots; the start URL on the dev origin; her email and the password `demo-password-local-only` when she has an account; the sandbox mailbox URL; the conditions profile; the stages to walk, by number, all or a named few, so any stage printed above and not named is outside her brief; any limit her persona states on her patience; and the shots directory, an absolute path in the scratchpad.

Read the finished brief once for anything the persona would not know: a route other than her start URL, a button's name, a step. Take it out.

## 4. Walk, then evaluate

1. Spawn `persona-walker` with the brief, `model: "opus"`, in the foreground. One walk at a time.
2. Save its final message to `<scratchpad>/<slug>-acts.md`.
3. Spawn `walk-evaluator` with `model: "opus"`, naming: the act log, the shots directory, the slug, the conditions profile, the walked SHA, and the output file. A walk that is part of a simulation run writes to `docs/simulation/runs/<run-id>/<slug>.md`, and the evaluator copies each shot it cites into that run's `shots/` as `<slug>-<entry-id>.png`; any other walk writes to the scratchpad and cites the walker's `<slug>-act-<n>.png` as it is.
4. Close the walker's pages and stop the stack.

The walk is admissible when the evaluator reports zero `u` entries. A `u` is a harness failure, so fix the capture before walking again rather than writing around it. Turning entries into filed issues is [`docs/simulation/findings.md`](../simulation/findings.md)'s pass, after the walk.
