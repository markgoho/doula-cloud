You are an independent grader of one screen of DoulaCloud, the Staff Invoices list. You have NOT seen any earlier score, history or other grader. Grade strictly; be skeptical; a 4 needs evidence for every check. You score TWO rubrics.

`<root>` is the worktree the orchestrator names; `<origin>` is the dev origin it names. Read only inside `<root>`.

Read first (your only standard):
- Usability (/40): `<root>/.impeccable/rubrics/invoices-list.md`
- Craft (/20): `<root>/.impeccable/rubrics/craft.md`, with the user-level skill's `~/.claude/skills/impeccable/reference/craft-floor.md` as its floor. The committed direction is in `<root>/.impeccable/surfaces/practices-practiceid-invoices-page-svelte.md` and PRODUCT.md's Brand Commitments (the category standard at the craft level of Linear, Stripe Dashboard and GOV.UK services). Grade craft against that bar, not against a bolder world.
- Constraints: ADR-0021 (GOV.UK service patterns, docs/design/govuk-alignment.md), ADR-0024 (intrinsic layout, no viewport queries), the Voice section of docs/design/brief.md, WCAG 2.2 AA, ADR-0015 ("Payment due", never "Due date"), and #1829 (a Client's name never in a URL).

The screen:
- Source: app/src/routes/practices/[practiceId]/invoices/InvoicesScreen.svelte, +page.svelte and +page.ts, and what they use (app/src/lib/invoice.ts; app/src/lib/components/templates/ListPage, organisms/DataTable, molecules/FilterBar, molecules/SummaryList, atoms/Badge, atoms/Link; app/src/lib/styles/tokens.css and base.css). BFF: api/internal/payments/practice_invoices.go. The Clients list, the screen H4 compares with: app/src/routes/practices/[practiceId]/clients/+page.svelte.
- Live: Chrome is signed in as Renata Alvarez (Owner, Admin and a Doula) at `<origin>`. Start at `<origin>/` and reach Invoices by the nav. Read only, with one exception the rubric's T3 needs: you may open an Invoice from the list and come back. Never create, send, save, record a payment or delete.

Browser: load tools in ONE call: ToolSearch "select:mcp__claude-in-chrome__tabs_context_mcp,mcp__claude-in-chrome__tabs_create_mcp,mcp__claude-in-chrome__navigate,mcp__claude-in-chrome__computer,mcp__claude-in-chrome__find,mcp__claude-in-chrome__read_page,mcp__claude-in-chrome__get_page_text,mcp__claude-in-chrome__javascript_tool,mcp__claude-in-chrome__tabs_close_mcp". tabs_context_mcp first; create your OWN tab and keep using its id (other graders share the browser; if your tab vanishes, create another); close it at the end. Do not sign out or in. javascript_tool only to read (measurements, computed styles, document.activeElement, accessible names, performance resource names), with one exception: for 375px / 768px / 640px checks, on any app page run `document.documentElement.innerHTML='<body style="margin:0;background:#888"><iframe src="<invoices URL>" style="width:375px;height:740px;border:0;display:block;background:#fff"></iframe></body>'` (change the width) and judge the frame; 640px stands in for 200% zoom at 1280px (say so). Window resizing does not work here. The app follows the OS color scheme; to judge the other theme, read the dark/light token values in tokens.css and, where you can, compute contrast from them. T8: judge from source and the route's specs (invoices.svelte.spec.ts, invoices.overdue.svelte.spec.ts, invoices-load.spec.ts). H5's contractor check: judge from source (the nav, +page.ts) and api/internal/payments/practice_invoices_test.go.

Walk every task T1–T8 yourself and count actions. Then score.

Return EXACTLY:
1. Usability table: heuristic # | 0–4 | failed checks (rubric wording) | evidence for passes. Total /40.
2. Tasks T1–T8: pass/fail, actions, evidence.
3. Craft table: C1–C5 | 0–4 | failed checks | evidence. Total /20.
4. Findings: every failed check from both rubrics, ranked by how much it hurts a real person's job (craft findings ranked below usability ones unless they hurt reading), each with a concrete fix.

Do not edit files. Do not spawn agents. Run plain single shell commands only. Budget about 30 minutes.
