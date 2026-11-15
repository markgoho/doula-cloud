---
target: Staff Contracts list (rubric round 1; craft 17/20)
total_score: 37
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 5
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/feat-2749-contracts-hill-climb/app/src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte"
target_fingerprint: "sha256:c549906b32059224d2fb42dd3e11696f950707d97ea9e9c80e849af3129e919e"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/feat-2749-contracts-hill-climb/app/src/routes/practices/[practiceId]/contracts/ContractsScreen.svelte
timestamp: 2026-11-15T02-47-53Z
slug: acticeid-contracts-contractsscreen-svelte-0a09fe7d
---
Method: round 1 at cd55a4b4, after the first build (the BFF's list of every Contract by state, the filter bar, the queue above it, the nav for every Staff member, the Engagement landing) and two fix builds. Four blind walks by `persona-walker` at 44105890: Dee (1280×800, pointer then keyboard only; T1, T2, T3, T9), Renata (1280×800; T4, T5), Priya (375×740, touch; T6), Lena (1280×800; T7). The walks showed two defects, fixed before the graders with no second walk: the Signed PDF button gave no sign it had downloaded, and a typed Engagement address logged a TypeError. Three blind graders, median per item, all in chrome-devtools as Renata. Seed: 39 Contracts (3 not sent, 6 sent from today to 41 days, 28 signed, 2 voided), three void requests nobody has decided.

## Usability 37/40 at the median (graders 37, 35, 38) · Craft 17/20 (18, 18, 17)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 3, H7 4, H8 3, H9 4, H10 4. Craft: C1 4, C2 3, C3 4, C4 2, C5 4.

Jobs: T1 to T10 pass (3/3 each).

## Findings (two or three graders)

- H6a: in the default view the Contracts not sent stand among the sent ones by day (Talia Brandt, made Nov 8, between Imogen Okafor, sent Nov 5, and Tamsin Reyes, sent Nov 9), the order is named "Oldest first", and the brief does not say where they stand (3/3).
- H8e: the "Signed PDF" column is drawn empty in the Not signed, Draft and Sent views, which can hold no signed Contract; in the Draft view every "Sent by" cell is a dash (3/3).
- H4d: reminding or sending again is deferred to #2951, not declined with a reason, and Dee's walk reached for it on a Contract 41 days out (2/3; the third held it as sequencing).
- C4: the title-to-intro gap (40px) is larger than the gaps between sections (36 to 38px), and at 1280×800 one Contract row of nine is on the first screen (row 1 at 744 to 785) (3/3).
- C2: each void request's line runs about 110ch, past the 65 to 75ch measure (2/3).

## Notes (one grader, or the walks)

- H6d: "Joss Adeyemi +1" hides the second Doula's name on screen; Lena could not find her own (B; Dee, Lena).
- H8f: a Draft's row says "Draft" and then "Sent by: No one yet" (B).
- The badge reads "Void asked" where the Engagement says "Void requested" (B; Priya).
- A voided row opens the Engagement at `#contract-<its id>`, which is not on the page: the Contract section shows the replacement Draft and the voided Contract is nowhere there (B).
- The voided list names who sent a Contract, not who voided it or why; the Engagement's Contract section does not say either (Renata).
- Changing Show changes the Order select's width and moves Apply (813 to 851px) and the tag; column positions shift between views (A).
- The Order's words change with each view, and "All" changes the order to newest first (B, C; Renata, Priya, Lena).
- "Clear filters" returns to Not signed, not to All (A, C; Renata).
- A Draft's row does not say who wrote it; the Engagement does not say the day a void was asked; the void warning does not say what becomes of an Invoice still owed (Dee).
- At 375px "Apply" stands beside a collapsed "Filters and order" (C).

## Expected and missing, before the brief

Named by all three: a reminder or send-again from the row (Later, #2951), bulk selection (Later, #2951), sort by a column heading (Later, #2955), a count beside each view (Declined), an export (Later, #2954), a date range (Later, #2954), whether the Client opened it (Later, #2952), expiry (Declined). Named by two: starting a Contract from the list (Declined), search as it is typed (not in the baseline; the lists post the search and wait for Apply, #1829), saved views (Declined). Named by one: a row menu (Declined), a Contract's name or template (not in the baseline; the Care cell says the kind), a row peek or side panel (not in the baseline).

Declines and deferrals judged: #2951 does not hold for a reader DocuSign or Dropbox Sign trained (2/3); #2955 holds (2/3) and does not hold at 1280px (1/3); the count beside each view holds weakly (3/3); #2954 holds (2/3), weakly against the Invoices list's own "Download CSV" (1/3); #2952, expiry, a new Contract from the list, the row menu and saved views hold (each grader who met it). "Search as it is typed" and "a row peek" are added to the baseline before round 2.
