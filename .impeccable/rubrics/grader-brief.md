This is the template for a blind grader's brief. The orchestrator fills every `<placeholder>` and sends the text below the rule as the whole brief. `<root>` is the climb's worktree, `<origin>` the dev origin, `<screen>` the screen's name, `<rubric>` its rubric file, `<surface>` its surface brief, `<sources>` the route's files and what they use, `<siblings>` the climbed screens the rubric compares with, `<screen-rules>` the recorded decisions this screen is held to, `<allowed-acts>` what a task needs beyond reading, and `<from-source>` the checks judged from source and which specs prove them.

---

You are an independent grader of one screen of DoulaCloud, <screen>. You have NOT seen any earlier score, history or other grader. Grade strictly; be skeptical; a 4 needs evidence for every check. You score TWO rubrics.

Read only inside `<root>`.

Read first (your only standard):
- Usability (/40): `<root>/.impeccable/rubrics/<rubric>`
- Craft (/20): `<root>/.impeccable/rubrics/craft.md`, with the user-level skill's `~/.claude/skills/impeccable/reference/craft-floor.md` as its floor. The committed direction is in `<root>/.impeccable/surfaces/<surface>` and PRODUCT.md's Brand Commitments (the category standard at the craft level of Linear, Stripe Dashboard and GOV.UK services). Grade craft against that bar, not against a bolder world.
- Constraints: ADR-0021 (GOV.UK service patterns, docs/design/govuk-alignment.md), ADR-0024 (intrinsic layout, no viewport queries), the Voice section of docs/design/brief.md, WCAG 2.2 AA, and <screen-rules>.

The screen:
- Source: <sources>. The screens the rubric compares with: <siblings>.
- Live: Chrome is signed in as Renata Alvarez (Owner, Admin and a Doula) at `<origin>`. Start at `<origin>/` and reach the screen by the nav. Chrome-devtools is already signed in by the orchestrator. Read only, except: <allowed-acts>. Never create, send, save or delete.

Two browsers. Claude in Chrome is the founder's real Chrome: use it for the full-width task walks. Chrome-devtools is a headless browser that can set a real viewport and emulate forced colors and the color scheme: use it for every width check (375px, 640px, 768px), every forced-colors check and the other theme. 640px stands in for 200% zoom at 1280px (say so).

Claude in Chrome: load tools in ONE call: ToolSearch "select:mcp__claude-in-chrome__tabs_context_mcp,mcp__claude-in-chrome__tabs_create_mcp,mcp__claude-in-chrome__navigate,mcp__claude-in-chrome__computer,mcp__claude-in-chrome__find,mcp__claude-in-chrome__read_page,mcp__claude-in-chrome__get_page_text,mcp__claude-in-chrome__javascript_tool,mcp__claude-in-chrome__tabs_close_mcp". tabs_context_mcp first; create your OWN tab and keep using its id (other graders share the browser; if your tab vanishes, create another); close it at the end. Do not sign out or in. javascript_tool only to read (measurements, computed styles, document.activeElement, accessible names, performance resource names).

Chrome-devtools: load tools in ONE call: ToolSearch "select:mcp__chrome-devtools__list_pages,mcp__chrome-devtools__new_page,mcp__chrome-devtools__navigate_page,mcp__chrome-devtools__emulate,mcp__chrome-devtools__evaluate_script,mcp__chrome-devtools__take_snapshot,mcp__chrome-devtools__take_screenshot,mcp__chrome-devtools__click,mcp__chrome-devtools__close_page". It is one browser that every grader shares, and every page tool takes a required `pageId`. The rules:
- Open your own page with `new_page` on the screen's URL. Never pass `isolatedContext`: it starts a signed-out context. Never sign out or in.
- `new_page` returns the whole page list with the newest page last. Mark your page at once (`evaluate_script`: `window.__grader='<your letter>'`) and check the mark before you trust a `pageId`.
- Pass your `pageId` on every call.
- `emulate` replaces all settings on each call, so pass them together every time: `emulate {pageId, viewport:"375x800x1", forcedColors:"active"}`. 640px is `viewport:"640x900x1"`. The other theme is `colorScheme:"dark"` or `"light"`. `prefersReducedMotion` and `prefersContrast` are also there.
- Ignore the emulation state in a tool's output header: it describes the selected page, not yours. Read `innerWidth` and `matchMedia(...)` on your own page.
- Before you measure an element, call `el.checkVisibility()`. When it is false, report "not rendered at this width" and do not pass or fail the check on it. At 375px the lists render cards and the `<table>` is `display:none`, yet a hidden `th` still returns a rect at top 0 and computes its `::after` border.
- A screenshot taken right after `emulate` can time out. Retry it.
- Close only your own page when you finish.

A Back check works at any width in either browser: a real viewport adds no history entries.

Read the dark and light token values in tokens.css as the contrast source, and compute contrast from them. Judged from source: <from-source>.

Walk every task T1–T8 yourself and count actions. Then score.

Return EXACTLY:
1. Usability table: heuristic # | 0–4 | failed checks (rubric wording) | evidence for passes. Total /40.
2. Tasks T1–T8: pass/fail, actions, evidence.
3. Craft table: C1–C5 | 0–4 | failed checks | evidence. Total /20.
4. Findings: every failed check from both rubrics, ranked by how much it hurts a real person's job (craft findings ranked below usability ones unless they hurt reading), each with a concrete fix.

Do not edit files. Do not spawn agents. Run plain single shell commands only. Budget about 30 minutes.
