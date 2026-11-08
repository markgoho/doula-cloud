---
target: signed-out landing page
total_score: 24
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/(signed-out)/+page.svelte"
target_fingerprint: "sha256:17319f15eed440006a5d2d74e5f49567da34675d27806d8d882aeb43d4988082"
target_path: /home/mark/github/doula-cloud/.claude/worktrees/impeccable-trial/app/src/routes/(signed-out)/+page.svelte
timestamp: 2026-11-07T16-56-37Z
slug: app-src-routes-signed-out-page-svelte
---
Method: dual-agent (A: design review, B: detector)

## Design Health Score: 24/40 (Acceptable)

| # | Heuristic | Score | Key Issue |
|---|---|---|---|
| 1 | Visibility of System Status | 3 | Static page; no memory of last door used |
| 2 | Match System / Real World | 2 | "Staff" / "Client portal" are system words |
| 3 | User Control and Freedom | 3 | Plain links; no exit to doula.cloud |
| 4 | Consistency and Standards | 3 | Three doors in two visual languages |
| 5 | Error Prevention | 2 | Wrong-door risk; Staff line omits Admins |
| 6 | Recognition Rather Than Recall | 3 | Must self-classify |
| 7 | Flexibility and Efficiency | 2 | No remembered door |
| 8 | Aesthetic and Minimalist Design | 2 | Half the desktop viewport is decoration; generic flat look |
| 9 | Error Recovery | 3 | Depends on login screens cross-linking |
| 10 | Help and Documentation | 1 | No "not sure which?", no link to what DoulaCloud is |

## Design Specificity Verdict
LLM: category-interchangeable split-screen auth layout; only authored moments are copy ("Up late? Welcome.", "Your contract, your birth plan, and your invoices"). Flat dark panels, candy-pink primary, one grotesk weight read as starter template.
Deterministic: 0 findings in source (page, layout, two import levels) and rendered at 1280x800 and 320x700. Overlay injected, "[impeccable] No anti-patterns found."

## What's Working
1. Greeting computed before paint: no swap, no layout shift.
2. DoorLink: whole-row links, labelledby/describedby, >=44px targets, sensible tab order.
3. Intrinsic layout (grid-l, cqi mark), sound at 320px and embedded.

## Priority Issues
1. [P1] Phone: doors below decorative welcome hero (inferred from source; not rendered). Fix: doors first when stacked, greeting one line above h1. Command: layout, adapt.
2. [P1] Doors ranked by population; Staff is the only filled door, Client door secondary. Fix: equal sign-in doors; "Set up a Practice" demoted to a line. Command: layout, clarify.
3. [P2] Generic identity; candy-pink accent wrong for 3am. Fix: dusky deeper plum in dark, real material or no welcome panel, second typeface for greeting, distinct late-night moment. Command: colorize, typeset.
4. [P2] Focus ring on filled primary door is primary-on-primary, near 2.4.11 failure. Fix: two-tone ring / on-surface outline. Command: harden.
5. [P2] Door labels in system words; Admin not named. Fix: keep "Staff sign in" (Voice rule 5 example), change line to "For doulas, owners and office staff"; rethink "Client portal". Command: clarify.

## Persona Red Flags
- Client from email (Hannah/Nadia): loudest element says Staff; bright greeting on hot pink mismatched for loss.
- Dee (Admin): not named under Staff door.
- Casey (mobile): scrolls past mark before any door (inferred).
- Sam (a11y): greeting read before h1; weak focus ring on primary.
- Tasha (evaluator): only pitch is 15px "first three clients free".

## Minor Observations
Arrow on one door only; underline inside bordered card; "Welcome to DoulaCloud." repeats lockup; full-height divider reads as seam; dead field below panels; light mode unchecked.

## Questions to Consider
1. If nearly everyone here has an account, why a hero at all?
2. Could the page know who is arriving (last door, ?for=client on Client emails)?
3. What if the page were designed for the 3am reader first?
