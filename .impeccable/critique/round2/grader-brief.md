You are an independent grader of one screen of DoulaCloud, the Staff Engagement record (one Client's care). You have NOT seen any earlier score, history or other grader. Grade strictly; be skeptical; a 4 needs evidence for every check. You score TWO rubrics.

Read only inside `/home/mark/github/doula-cloud/.claude/worktrees/feat-2755-engagement-record-hill-climb` (call it ROOT).

Before you read the surface brief: having read only this brief and the rubric's jobs, and having looked at the screen, write down every thing you expected to be able to do on a screen of this kind and could not, each with the product or design system that taught you to expect it. Write that list to your report file first. Only then read the surface brief.

Read first (your only standard):
- Usability (/40): `ROOT/.impeccable/rubrics/engagement-record.md`
- Craft (/20): `ROOT/.impeccable/rubrics/craft.md`, with `~/.claude/skills/impeccable/reference/craft-floor.md` as its floor. The committed direction is `ROOT/.impeccable/surfaces/practices-practiceid-engagements-engagementid-page-svelte.md` and PRODUCT.md's Brand Commitments (the category standard at the craft level of Linear, Stripe Dashboard and GOV.UK services). Grade craft against that bar.
- Constraints: ADR-0021 (GOV.UK service patterns, docs/design/govuk-alignment.md), ADR-0024 (intrinsic layout, no viewport queries), the Voice section of docs/design/brief.md, WCAG 2.2 AA, ADR-0018 (all amendments), ADR-0022, ADR-0036, ADR-0008, DESIGN.md.

The screen:
- Source: `app/src/routes/practices/[practiceId]/engagements/[engagementId]/` (+page.svelte, EngagementScreen.svelte, EngagementStatusLine.svelte, sectionOrder.ts, birth-plan/), `app/src/lib/components/templates/RecordDetail.svelte`, and the components they import. Siblings: the Clients, Invoices, Schedule, On call and Contracts lists under `app/src/routes/practices/[practiceId]/`.
- Live: dev origin `http://localhost:5473`. Practice `/practices/e77019d1-9d1c-4e83-aec4-fa3d3bc74c62`. Camille Boyd's record: `http://localhost:5473/practices/e77019d1-9d1c-4e83-aec4-fa3d3bc74c62/engagements/de41308c-4240-4421-9931-7513c94a9f38`. The others (Grace Whitfield, Imani Brooks, Mira Castellanos, Tamsin Reyes, Jamie Bishop, Morgan Grier, Ingrid Larsen, Willa Sandoval, Imogen Okafor, Talia Brandt) from the Clients list. The browser is signed in as Renata Alvarez (Owner, Admin and a Doula). Read only, except: opening and cancelling an in-place form, pressing "Load more" or "Show earlier Messages", choosing an Activity filter, toggling a disclosure, and `emulate networkConditions:"Offline"` on your own page for T10. Never create, send, save or delete.
- Walk act logs (persona-walker, blind, signed in as each person): `ROOT/.impeccable/critique/round2/acts-priya.md` (T1, T3, T4 at 375×740, this round), and from round 1 `ROOT/.impeccable/critique/round1/acts-dee.md` (T5, T6 at 1280×800) and `acts-lena.md` (T7 at 375×740). Use them as the evidence for those jobs; you may check any claim on the page. Screenshots they cite are in `ROOT/.impeccable/critique/round2/shots/`.

Use ONLY chrome-devtools. Load tools in ONE call: ToolSearch "select:mcp__chrome-devtools__list_pages,mcp__chrome-devtools__new_page,mcp__chrome-devtools__navigate_page,mcp__chrome-devtools__emulate,mcp__chrome-devtools__evaluate_script,mcp__chrome-devtools__take_snapshot,mcp__chrome-devtools__take_screenshot,mcp__chrome-devtools__click,mcp__chrome-devtools__press_key,mcp__chrome-devtools__close_page,mcp__chrome-devtools__list_console_messages". It is one browser that every grader shares; every page tool takes a required `pageId`. The rules:
- Open your own page with `new_page` on the screen's URL. Never pass `isolatedContext` (it starts signed out). Never sign out or in.
- Mark your page at once (`evaluate_script`: `window.__grader='<your letter>'`) and check the mark before you trust a `pageId`.
- `emulate` replaces all settings each call; pass them together: `emulate {pageId, viewport:"375x740x1", forcedColors:"active"}`. 640px (`"640x900x1"`) stands in for 200% zoom at 1280px (say so). The other theme is `colorScheme:"dark"`/`"light"`.
- Read `innerWidth` on your own page; ignore the emulation line in a tool's output header.
- Before you measure an element, call `el.checkVisibility()`; if false, report "not rendered at this width".
- Screenshots: save to files under `ROOT/.impeccable/critique/round2/` with your letter in the name. Retry one that times out after `emulate`.
- If your page lands on /login, stop live work, say so in your report, and grade the rest from source (do not sign in).
- Close only your own page at the end.

Read the dark and light token values in `app/src/lib/styles/tokens.css` as the contrast source.

Scoring rule for what you cannot reach: a check you could not verify is "unverified" — it is not a fail, and it does not cap the heuristic. Score the heuristic on the checks you did verify (all verified checks pass → 4). Say which checks were unverified.

Evidence the orchestrator gathered for checks you are told not to act on (saving is forbidden to you): on this build, signed in as Renata on Imani Brooks's record (then reseeded), recording a $950.00 bank-transfer payment changed the Owed fact to "Nothing owed" and the section to "Nothing owed." in place; "Message the Client" opened the compose with focus in the Message field, and sending changed the Messages fact to "Answered" in place; adding a Visit for Nov 16, 9:30am changed the Next Visit fact and the Visits state line in place. Lena Vasquez on Morgan Grier sees no Owed fact; the only money on her page is her own $450.00 fee. On rubric 1a: the dev server does not finish loading under DevTools' Slow 4G (it serves hundreds of unbundled modules), so judge 1a's skeleton timing as unverified and its no-shift half from the page under no throttling.

Walk every task T1–T10 you can (from Renata's session, plus the act logs) and count actions. Then score.

Write your report to `ROOT/.impeccable/critique/round2/grader-<letter>.md` as you go, and return EXACTLY:
1. Usability table: heuristic # | 0–4 | failed checks (rubric letters) | evidence for passes. Total /40.
2. Tasks T1–T10: pass/fail/unverified, actions, evidence.
3. Craft table: C1–C5 | 0–4 | failed checks | evidence. Total /20.
4. Findings: every failed check from both rubrics, ranked by how much it hurts a real person's job, each with a concrete fix.
5. Expected and missing: your list from before the surface brief, each with its source; then, against the brief's Category baseline, mark each built, declined (quote the reason; say whether it holds for a reader those products trained), or not in the baseline.

Do not edit repository files other than your report and screenshots. Do not spawn agents. Plain single shell commands only. Budget 25 minutes. At 25 minutes stop gathering and write the report from what you have; an unreached check is "unverified", not a fail.
