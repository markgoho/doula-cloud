# Distinctiveness and craft: what a 4 means (out of 20)

A second score beside the usability rubric (`clients-list.md`), never folded into it. 40/40 there says every job works; 20/20 here says the screen looks authored for DoulaCloud and finished to the last surface. Neither can be bought with the other: **a craft change that makes any usability check fail scores 0 on the dimension it was made for**, however good it looks.

Same scoring rule as the usability rubric: a dimension scores 4 only when every check under it passes with evidence (a screenshot, a measured value, a computed style). 3 = one check fails, 2 = two, 1 = three or more, 0 = the dimension's core is absent. Three blind graders, median per dimension. Graders apply impeccable's `reference/craft-floor.md` (its Verify list and its Refuse list) as the floor.

The direction itself is set by `impeccable shape` for the Staff app and recorded in DESIGN.md; until it is, C2–C5 grade against `docs/design/brief.md`.

## C1. Specificity
- A grader shown the screen with the name and logo removed can say what kind of product it is for and who uses it, from the composition and language alone.
- At least one element exists because of this product's domain and would not appear in a generic admin list unchanged (judged against the Refuse list's scaffolds).
- No element is a category default the brief did not choose (Refuse list: card grids, hero metrics, eyebrows, gradient text, decorative glass, colored side borders, emoji-as-icons).

## C2. Typography
- The faces are a committed choice with character, self-hosted, and the display voice is not a platform sans.
- Scale and weight steps are obvious and few; the same role has the same style on every Staff screen (measured on 3 screens).
- Numbers in data (money, dates, counts) use tabular figures and line up in a column.
- Prose stays inside 65–75ch; headings balance; nothing overflows at 320px with real copy.

## C3. Color and surface
- The palette is committed and chosen from the use scene (a doula on a phone in a hospital corridor, an owner at a desk), in both themes.
- Each color means one thing app-wide (warning, error, money, link), and secondary text on a colored surface is tinted from it, never flat gray.
- The surfaces the page did not draw carry the design: text selection, caret, scrollbars, focus rings, link underline offset.
- Contrast passes in both themes for text, controls and focus.

## C4. Rhythm and composition
- Spacing groups what belongs together and separates what does not, measured from computed values (related < group < section), and the same relationships hold on every Staff screen.
- Alignment holds to one edge and one baseline grid per screen; nothing is off by a few pixels (measured).
- Density fits Operate mode: the screen's job fills the first screen at 1280px and at 375px.

## C5. Signature and restraint
- At most one signature component (PRODUCT.md): DESIGN.md either names it, recognizable as DoulaCloud and appearing only where it earns its place, or records that no candidate has earned one, with the evidence. A candidate that fails a usability check never counts.
- One authored motion moment at most, smooth and reduced under `prefers-reduced-motion`; no scattered effects.
- Everything outside the signature is quiet: no ornament competes with the screen's job (the usability rubric's checks still pass).

## Changes to this rubric

- 2026-11-08, after round 7: C5 asked for exactly one signature component. PRODUCT.md (Mark, 2026-11-08) allows at most one, kept only if it earns its place. The due strip, the one candidate built, was judged by all three graders to hurt the list and was reverted (60e08e41). The check now passes when DESIGN.md records that no candidate has earned one, with the evidence; it contradicted a recorded commitment, which is the only ground for changing a check.
