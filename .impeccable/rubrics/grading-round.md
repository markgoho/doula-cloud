# A grading round

One round scores one screen against its rubric (`clients-list.md` for the Clients list). It is what turns "the critique says 34" into a number that means the jobs work.

1. **Walks first.** For each task check (T1–T8), `persona-walker` walks it blind on `dev:full`, under the profile the rubric names (375px for T4, keyboard only for T7, accessibility snapshot only for T6, the BFF refusing for T8), with a brief built as `docs/agents/persona-walk.md` says. The task is given as the persona's goal, never as the path. Each walk returns its act log. Record `git rev-parse HEAD`.
2. **Three graders, blind.** Spawn three graders in parallel. Each gets the rubric, the screen's URL and source paths, and the act logs. None gets an earlier score, this branch's history, the other graders' output, or any word on what changed. Each scores all ten heuristics strictly by the rubric's rule (4 = every check passes with evidence) and returns, for each heuristic, the score, the checks that failed, and the evidence behind each pass.
3. **Median per heuristic.** The round's score is the sum of the ten medians. A check two of three graders fail is a finding; one grader's lone fail is a note.
4. **Record.** Write the round with `impeccable critique-storage write` (meta: target, total, max 40, P0/P1 counts), with the per-heuristic medians, the failed checks and the walked SHA.
5. **Fix by the job, not the grader.** Every fix names the check it makes pass and is verified the same way the check is graded. A change that moves a score without changing what a person can do is reverted.

The rubric changes only when a check proves wrong about the person's job, never because it is hard to pass, and the change is recorded with its reason at the foot of the rubric.
