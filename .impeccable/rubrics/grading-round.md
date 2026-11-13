# A grading round

One round scores one screen against its rubric (`clients-list.md` and `invoices-list.md` are the models). It is what turns "the critique says 34" into a number that means the jobs work.

## Before the baseline

1. **Seed first.** `dev:full`'s demo Practice holds a realistic book for this screen: enough rows to page, sort and scroll, and one row in every state the screen can show. Extend `app/scripts/demo-practice.ts` until it does. A round graded on three rows measures nothing. A seeded act has its own effects (naming a Doula on a Visit attaches her to the Engagement), so the seed build's brief lists the sibling screens' numbers that must not move, and the agent reads them on the rendered page before and after: the Schedule's first seed put five Doulas on one Client and the Clients list scrolled sideways.
2. **Read the siblings first.** Open every screen already climbed (the Clients list, the Invoices list) and start from its `FilterBar` and `DataTable` usage, its control sizes and its wording for a shared state. Where this screen must differ, write the reason in the surface brief. "Unlike the Clients list" was the Invoices climb's last lost point for four rounds.
3. **Settle a shared state once.** When two checks pull against each other (a state said twice against a state said the way a sibling says it), write the rule in `DESIGN.md` before building either.
4. **Have the rubric draft read twice before the baseline.** One Opus agent judges each job against the record lines it cites and each check for whether two honest graders would score it alike; `agy` with Gemini looks for checks that pass without the job working. Each found about a dozen defects in the Schedule's draft, and none overlapped much.
5. **Decide the whole arrangement before the first build.** After the baseline, one Opus agent reads the rubric, the findings, the siblings and the records, and returns each control, its words, its address parameter and the shared option it uses. The first build then carries the arrangement in one piece. The Schedule went from 25 to 39 in one round this way.

## The round

1. **Walks first.** For each task check (T1–T8), `persona-walker` walks it blind on `dev:full`, under the profile the rubric names (375px for T4, keyboard only for T7, accessibility snapshot only for T6, the BFF refusing for T8), with a brief built as `docs/agents/persona-walk.md` says. The task is given as the persona's goal, never as the path. Each walk returns its act log. Record `git rev-parse HEAD`. A job that belongs to a role the graders' session is not (a Doula with no Owner or Admin role) is always walked, signed in as that person. A walker's brief tells it to apply `emulate` to its own page only. Read each act log before any grader spawns: what a walk shows wrong is fixed first and the walk repeated, which takes about two minutes, so the graders and the logs describe one screen.
2. **Freeze the tree.** From the first grader's spawn to the third grader's report, the worktree holds still: the dev server hot-reloads, so an edit changes the page under a grader. Builds start when the third report is in.
3. **Sign the chrome-devtools browser in.** Graders check width and forced colors through chrome-devtools (see `grader-brief.md`), and its profile starts empty. Once, before any grader spawns, the orchestrator does `new_page` to `<origin>/login`, fills "Email address" with `renata.alvarez@sim.doula.cloud` and "Password" with `demo-password-local-only` (`dev:full` prints both), and clicks "Sign in". No second factor is asked; the two-factor banner on the Overview is optional. Then a plain `new_page` on the screen's URL must land signed in; close that page. Every later plain `new_page` shares the session, and graders never sign out or in.
4. **Three graders, blind.** Spawn three graders in parallel, each briefed from `grader-brief.md` with its placeholders filled. Each gets the rubric, the screen's URL and source paths, and the act logs. None gets an earlier score, this branch's history, the other graders' output, or any word on what changed. Each scores all ten heuristics strictly by the rubric's rule (4 = every check passes with evidence) and returns, for each heuristic, the score, the checks that failed, and the evidence behind each pass.
5. **Median per heuristic.** The round's score is the sum of the ten medians. A check two of three graders fail is a finding; one grader's lone fail is a note. A note that names no failed check is built only after the brief lists every rubric check that reads the same element and how the fix keeps each one passing: on the Schedule a comma hidden to answer such a note failed two checks in the next round.
6. **Record.** Write the round with `impeccable critique-storage write` (meta: target, total, max 40, P0/P1 counts), with the per-heuristic medians, the failed checks and the walked SHA. Confirm `git status` lists the new snapshot; if git reports it ignored, stage it with `git add -f`.
7. **Plateau rule.** Two rounds in a row at the same score, on different failed items, end the climb's own judgment: bring the founder the score, the items, and the choice to accept or continue.

## Each build between rounds

- **Fix by the job, not the grader.** Every fix names the check it makes pass and is verified the same way the check is graded. A change that moves a score without changing what a person can do is reverted.
- **Hold-still table.** The build brief lists the measurements that must stay as they are (column positions at 1280px and 1920px, the first screen's row count, each accessible name the rubric reads, the sibling screens' layout). The build agent measures them before and after on the rendered page and returns both as a table. A moved number is a regression until the brief says it was the goal.
- **Shared component first.** A fix a sibling list also needs goes into `FilterBar`, `DataTable` or the shared component the first time, and the sibling is checked on the rendered page in the same build.
- **Commit each build through the gate** before the next one starts. An agent stopped mid-build is not committed. The gate does not run the duplication check: the orchestrator runs `bun run dupes:check` from the repo root after each build.
- **A build agent runs in the foreground.** Its brief forbids background commands and monitors, the commit included: a build that left monitors behind kept waking and sending its report again. The brief names the reading it requires (`impeccable context`, `craft-floor.md`, the Voice section) as required, and the agent's report says which it read.
- **A second model reviews, three graders score.** For a rubric draft or a large build, `agy` with Gemini reviews from a diff file: `git diff <a> <b> --output=<file> -- <paths>`, under about 100KB, one concern per run, `--mode plan --add-dir <dir>`. Plan mode reads files and runs no command. Its findings are review; the score stays the three graders' (ADR-0063).

## The rubric

The rubric changes only when a check proves wrong about the person's job or contradicts a recorded decision, never because it is hard to pass, and the change is recorded with its date and reason at the foot of the rubric.

## Closing the climb

Before `gh pr create`, `git push` runs `scripts/hooks/pre-push` (the duplication check, and the API message gate when Go changed). Run the Go tests for every `api/` package the branch touched as well; the hook leaves those to CI because they start Postgres.
