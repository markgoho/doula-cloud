---
target_identity: "file:app/src/routes/practices/[practiceId]/engagements/[engagementId]/EngagementScreen.svelte"
target_fingerprint: "sha256:bf820efa62548bf98e33e05542b7d26525b21f92e53f9b48b635d11c163fe819"
target_path: app/src/routes/practices/[practiceId]/engagements/[engagementId]/EngagementScreen.svelte
timestamp: 2026-11-16T15-09-40Z
slug: ents-engagementid-engagementscreen-svelte-f17b4fd5
---
Method: round 4, the closing round after the climb's loose ends (#2991, PR #3003; #2983, PR #3002), at d26d0faa. Three blind Opus graders, median per item. Priya (375×740), Dee (1280×800) and Lena (375×740) all walked this build (round4/acts-*.md). The orchestrator's save evidence is from f4cacd20; the graders were told to judge each save-refresh half from source and specs too, or mark it unverified.

## Usability 39/40 at the median (graders 39, 39, 39) · Craft 19/20 (20, 19, 17)

Medians: H1 4, H2 4, H3 4, H4 3, H5 4, H6 4, H7 4, H8 4, H9 4, H10 4. Craft: C1 4, C2 4, C3 4, C4 3, C5 4.

Up from 38/40 in rounds 2 and 3. Jobs: T1–T7, T9 pass 3/3 (T7's "another Engagement's address tells her nothing" half unverified by all three); T10 passes 3/3; T8 fails 2/3 (A, C).

## Findings (two or three graders)

- 4f / T8 (A, C): at 1280px and wider, the Details aside's `<h2>` came right after the facts in the DOM, so heading navigation read Details (drawn at y≈858 in the right column) before On call (drawn at y≈460). Fixed in this PR: the aside now follows the sections in the DOM and is placed beside them by its grid row, so heading navigation and Tab read the main column and then the related-items column; measured at 1440: headings On call … Activity, then Details at x1102 y875, the aside still drawn at 857.
- C4 (B, C): failed by two graders on different items, each a lone item — B: the current Activity filter ("All") has a ~42px gap after it against ~20px for the others; C: INV-0001's Invoices row wraps to three lines and says the payment method twice.

## Notes (one grader)

- 9c (B): a refused "Load more" shows the BFF's raw `message` ("internal error" for a 500) because `paginatedList.svelte.ts` prefers `error_.message` over the list's own `failureMessage`. Filed (app-wide; every paginated list).
- 4e (B): the Client's home address with a map link is not on the record. The Home address row (#2991) is drawn only when the Client has one, and Camille's seeded Client has none.
- C2 (C): one grotesque for every role, no display voice. C5 (C): DESIGN.md names the Activity ledger the signature, but it is drawn as a plain table.
- Walks (not scored): Escape does not close "Record a payment", whose Continue and Cancel sit below the fold at 1280×800 (Dee); Cancel discards typed Visit notes and messages with no guard (Priya); Add a Visit asks only for a date, no Type or Notes (Priya); the Phone fact is 3px below the 375×740 fold (Priya); Lena's own fee has no invoiced/paid state and her fee and the Invoices are not in the "Contract and money" Activity filter (Lena).

## Expected and missing

Named by two or more graders: previous/next record (A, B, C; declined in the baseline, "no job on record; Back returns to the list" — all three say the decline holds, weakly, for a Linear-trained owner); keyboard shortcuts (A, B, C; Later, #2956); copy a link or ID (A, B, C; declined, holds); files on the record (A, B, C; not in the baseline — partly built as the message Attachment); tasks or reminders (A, B; not in the baseline). Lone: a Client-view preview, add a Visit to a calendar, a paid-to-date total (C); comments on Activity, search in Activity or Messages (A); Activity by person (B).
