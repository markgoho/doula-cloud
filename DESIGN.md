---
name: DoulaCloud Staff app
description: The category standard for a doula Practice's working tool, held to the craft of Linear, Stripe Dashboard and GOV.UK services.
colors:
  surface-bright: "oklch(99.7% 0.002 255)"
  surface: "oklch(99% 0.003 255)"
  surface-container: "oklch(96.6% 0.007 255)"
  surface-container-high: "oklch(94.6% 0.009 255)"
  surface-container-highest: "oklch(92.2% 0.011 255)"
  on-surface: "oklch(23% 0.025 260)"
  on-surface-variant: "oklch(40% 0.03 258)"
  on-surface-muted: "oklch(49.5% 0.03 258)"
  outline: "oklch(57% 0.03 258)"
  outline-variant: "oklch(90.5% 0.012 258)"
  primary: "oklch(40% 0.11 258)"
  primary-hover: "oklch(48% 0.12 258)"
  on-primary: "oklch(98.5% 0.005 255)"
  error: "oklch(50.6% 0.193 28)"
  on-error: "oklch(99% 0.005 28)"
  status: "oklch(45% 0.15 145)"
  info: "oklch(45% 0.15 215)"
  warning: "oklch(45% 0.15 85)"
  neutral: "oklch(45% 0.02 285)"
typography:
  heading-lg:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1.953125rem, 1.875rem + 0.390625cqi, 2.34375rem)"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.018em"
  heading:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1.5625rem, 1.5rem + 0.3125cqi, 1.875rem)"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.012em"
  subheading:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1.25rem, 1.2rem + 0.25cqi, 1.5rem)"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "-0.006em"
  body:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1rem, 0.96rem + 0.2cqi, 1.2rem)"
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: "0"
  body-sm:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1rem, 0.96rem + 0.2cqi, 1.2rem)"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0"
  label:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1rem, 0.96rem + 0.2cqi, 1.2rem)"
    fontWeight: 500
    lineHeight: 1.35
    letterSpacing: "0.005em"
  meta:
    fontFamily: "Hanken Grotesk, Hanken Grotesk Fallback, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1rem, 0.96rem + 0.2cqi, 1.2rem)"
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: "0.01em"
rounded:
  radius: "8px"
  pill: "999px"
spacing:
  "1": "clamp(0.25rem, 0.225rem + 0.125cqi, 0.375rem)"
  "2": "clamp(0.5rem, 0.45rem + 0.25cqi, 0.75rem)"
  "3": "clamp(0.75rem, 0.675rem + 0.375cqi, 1.125rem)"
  "4": "clamp(1rem, 0.9rem + 0.5cqi, 1.5rem)"
  "5": "clamp(1.25rem, 1.125rem + 0.625cqi, 1.875rem)"
  "6": "clamp(1.5rem, 1.35rem + 0.75cqi, 2.25rem)"
  "7": "clamp(1.75rem, 1.575rem + 0.875cqi, 2.625rem)"
  "8": "clamp(2rem, 1.8rem + 1cqi, 3rem)"
  "10": "clamp(2.5rem, 2.25rem + 1.25cqi, 3.75rem)"
  "12": "clamp(3rem, 2.7rem + 1.5cqi, 4.5rem)"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.body}"
    rounded: "{rounded.radius}"
    padding: "{spacing.2} {spacing.4}"
    height: "2.75rem"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.on-surface}"
    rounded: "{rounded.radius}"
    padding: "{spacing.2} {spacing.4}"
    height: "2.75rem"
  button-secondary-hover:
    backgroundColor: "{colors.outline-variant}"
  button-destructive:
    backgroundColor: "{colors.error}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.radius}"
    padding: "{spacing.2} {spacing.4}"
    height: "2.75rem"
  link-primary:
    textColor: "{colors.primary}"
    height: "2.75rem"
  link-primary-hover:
    textColor: "{colors.primary-hover}"
  link-start:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.body}"
    rounded: "{rounded.radius}"
    padding: "{spacing.2} {spacing.4}"
    height: "2.75rem"
  link-start-hover:
    backgroundColor: "{colors.primary-hover}"
  link-nav:
    textColor: "{colors.on-surface-variant}"
    typography: "{typography.body-sm}"
    padding: "0 {spacing.4}"
  link-nav-current:
    textColor: "{colors.primary}"
  link-chip:
    backgroundColor: "{colors.surface-bright}"
    textColor: "{colors.on-surface}"
    typography: "{typography.label}"
    rounded: "{rounded.radius}"
    padding: "{spacing.2} {spacing.3}"
  link-chip-current:
    backgroundColor: "{colors.surface-container}"
    textColor: "{colors.primary}"
  badge:
    rounded: "{rounded.pill}"
    padding: "calc({spacing.1} / 2) {spacing.3}"
  text-input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface}"
    typography: "{typography.body}"
    rounded: "{rounded.radius}"
    padding: "{spacing.2} {spacing.3}"
    height: "2.5rem"
  table-header-cell:
    backgroundColor: "{colors.surface-container}"
    textColor: "{colors.on-surface-variant}"
    typography: "{typography.body-sm}"
    padding: "0 {spacing.3}"
    height: "2.5rem"
  table-cell:
    textColor: "{colors.on-surface}"
    typography: "{typography.body-sm}"
    padding: "0 {spacing.3}"
    height: "2.5rem"
  table-row-hover:
    backgroundColor: "{colors.surface-container}"
---

# Design System: DoulaCloud Staff app

This file records the built system for design work ([ADR-0063](docs/adr/0063-impeccable-is-adopted-a-screen-is-improved-by-blind-rubric-rounds-and-the-staff-app-looks-like-its-category.md)). `app/src/lib/styles/tokens.css` and `app/src/lib/components/**` are the truth; where this file and they, or a canonical record (`GLOSSARY.md`, `docs/adr/`, `docs/design/brief.md`'s Voice section), disagree, they win and this file is corrected.

## Overview

**Creative North Star: "The Familiar Room, Finished"**

The Staff app is the category standard for a working tool, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services (PRODUCT.md, Brand Commitments, chosen 2026-11-08). A doula or an owner opening Clients should know where everything is before noticing the look: a top bar with the nav, a title with one start action beside it, a filter bar, a result line, a table. Nothing about the arrangement is novel. The character is that nothing is out of place: every row is the same height, a group's heading row too, every number lines up, every badge means one thing, and both themes are proved, not hoped for.

The working ground is near-white in light and a deep blue-gray in dark (the 3am scene), with a near-black ink, neutral hairlines, and one blue spent on links, the primary action and the marker of where you are. Hanken Grotesk runs at a small, strict scale whose floor is 16px everywhere. Density is a desk tool's: a 40px table row at a fine pointer, 44px wherever an input can tap. The two worlds this one refuses are the costume world (a themed skin that performs a mood) and the half-finished default (a gray screen, one accent, loose spacing).

The Staff app's signature component is the activity ledger (PRODUCT.md, Positioning item 4): who did what and when, drawn the same way wherever a record is shown. It is the shared columns in `activityLedger.ts`, appearing on the Engagement record, the Practice overview and the Activity screen, and on the Client's own portal Engagement. A list is not a record, so the Clients list carries none by design. A list-level candidate was tried there and rejected: the due strip (each Client's care as a block on a time axis beside the date). All three round-7 graders judged it worse than the plain date text, so it was reverted. It took width, so an overdue row wrapped at 1440px; its amber clashed with Owes; its crowding was computed over the filtered page, not the Practice; its axis labels sat under the 16px type floor; and nothing in the Clients list's jobs used it. Distinctiveness comes from craft (PRODUCT.md).

**Key Characteristics:**
- Near-white working ground, near-black ink, neutral hairlines; nothing pure white or pure black in either theme.
- One blue, for links, the primary action, the current place and the focus ring.
- Quiet filled badges, one meaning per hue app-wide.
- Hanken Grotesk, one 1.25 ratio from a 16px floor, fluid on the container, never under 16px.
- Tabular figures on every date, amount and count in a table.
- Flat surfaces; depth is a tonal ladder and a hairline, never a shadow.
- One corner radius (8px), plus the pill for badges.
- Layout answers the space it is given (container queries), from 320px up.

## Colors

A cool, low-chroma blue-gray family carries the whole working ground; color with any real chroma is reserved for the one blue and the five status meanings. OKLCH is authoritative: the frontmatter values are the light theme, and every value is a token in `app/src/lib/styles/tokens.css`.

### Primary
- **Ink Blue** (`primary`): links, the primary Button, the start link ("Find or add a Client"), the current nav item's 2px rule and color, the current step, the applied filter chip, the focus ring, and the tint of a text selection. Its hover is lighter than rest in both themes (`primary-hover`).
- **On Ink** (`on-primary`): words on a primary or destructive fill.

### Neutral
- **Card White** (`surface-bright`): the raised top of the surface ladder; a card, a chip at rest, a badge's mixing base.
- **Working Ground** (`surface`): the page ground and an input's fill; the root canvas paints it too, so a short page never shows the browser's own white.
- **Header Tint** (`surface-container`): the table header row, a hovered row, the current sheet or menu row, the contents strip.
- **Deeper Tints** (`surface-container-high`, `surface-container-highest`): loading placeholders and the deepest inset panels.
- **Ink** (`on-surface`): body text and headings.
- **Secondary Ink** (`on-surface-variant`): table header labels, legends, nav at rest, secondary icons (EmptyState's icon, the step rail track).
- **Muted Ink** (`on-surface-muted`): the quietest text that still clears 4.5:1, such as a muted column (Ended) and the contents heading.
- **Control Edge** (`outline`): the boundary of a control (input, secondary Button, chip); clears 3:1.
- **Hairline** (`outline-variant`): decorative dividers, the table's row rules, a card's edge; exempt from 3:1.

### Status
Each keeps its own hue, deliberately not harmonized toward the palette.
- **Signal Red** (`error`): late or failed. An overdue Invoice and an expired Offer are late; a failed email delivery, a written-off Invoice and an error are failed. A destructive Button is red too.
- **Amber** (`warning`): needs someone to act. Owing money is one such thing; so are onboarding left unfinished, payouts on hold and a gap in cover.
- **Teal Blue** (`info`): information, or anything pending or waiting on someone else (a pending Request, "N waiting", "Deletion pending", "Awaiting Stripe review"). A different hue from Ink Blue; it never marks an action.
- **Green** (`status`): success.
- **Slate** (`neutral`): a neutral state.

### Named Rules
**The One Blue Rule.** The primary is spent on links, the primary action or start link, the marker of the current place (nav item, step, applied chip), the focus ring and the selection tint. A decorative icon or a progress track is Secondary Ink, not blue. A start link and a primary Button never share a screen region: the accent is spent once.

**The One Meaning Per Hue Rule.** Amber means one thing, needs someone to act (owing money is one such thing); red means one thing, late or failed (an overdue Invoice, a failed email delivery); blue is information or anything pending or waiting, green is success, gray is neutral, app-wide. A new state picks the meaning it shares, never a new hue. `Badge.svelte.spec.ts` holds the five tints visibly apart and their text at 4.5:1 in both themes; `clients-list.svelte.spec.ts` holds a pending Request to the blue badge so amber stays for what needs someone to act. `Badge.svelte` states the same meanings in its own comment.

**The Derived Dark Rule.** The dark theme is derived, not invented: hues are held, only lightness moves, and the surface ladder inverts so the page ground is the darkest step. The system-preference block and the `[data-theme]` override are byte-identical, and `tokens.spec.ts` proves both, and the shell palette, against the WCAG floors. The dark primary carries real chroma (oklch(79% 0.095 250), hover oklch(86% 0.06 250)): at the earlier 0.045 it read as gray beside the secondary ink, so the one blue is held at a chroma that still reads as blue while its lightness stays high enough for 4.5:1 on every dark ground.

**The Night Walls Rule.** The Staff top bar and its narrow menu sheet carry `data-shell`, inside which every color token resolves to one night palette in both themes: night-blue walls over a pale work area by day, a step deeper than the page at night. Components inside the bar read the same tokens they read anywhere.

**The Themed Browser Rule.** The browser's own surfaces follow the theme: `color-scheme` is declared in every theme block, the root canvas is the page ground, the selection is a 28% primary tint with ink text, the caret is ink, the scrollbar is thin in the Control Edge color, and the favicon and manifest take their colors from the tokens (`tokens.usage.spec.ts` holds the favicon). Selection, caret and scrollbar are build-held in `base.css`, not yet spec-held.

The `lamp` and `lamp-surface` tokens exist for the signed-out landing's doors and are not part of the Staff working ground.

## Typography

**Display Font:** none; the `display` step is set only by the brand lockup in the top bar, and no page content uses it
**Body Font:** Hanken Grotesk (with a metric-matched Arial fallback, `size-adjust: 95.07%`, so the swap does not move a line)

**Character:** One humanist grotesk at every step, carried by size, weight and tracking alone. Quiet, legible at the end of a long shift, and specific without being decorative.

### Hierarchy
Every step is a fluid `clamp()` on `cqi`: the floor is its size at a 320px column, the ceiling 1.2 times that at 1920px, and `tokens.spec.ts` re-derives every literal from the ramp. The floors are one 1.25 ratio from 16px.
- **Page title** (`heading-lg`, 600, 31.25px floor, 1.2): every screen's one `<h1>`, the same size on every Template.
- **Section** (`heading`, 600, 25px floor, 1.3): a record's or a hub's sections.
- **Card** (`subheading`, 600, 20px floor, 1.4): a card's title, the narrow sheet's nav rows, the large Button.
- **Body** (`body`, 400, 16px floor, 1.55): running text, inputs, Buttons. Prose caps at `--measure` (65ch).
- **Body small** (`body-sm`, 400, 16px floor, 1.5): table cells, badges, nav items.
- **Label** (`label`, 500, 16px floor, 1.35, +0.005em): legends, field labels, chips.
- **Meta** (`meta`, 400, 16px floor, 1.4, +0.01em): dates in a ledger (in tabular figures there), the filter bar's result line.

`body`, `body-sm`, `label` and `meta` are the same size; they differ only by weight, leading, tracking and the color a component gives them.

### Named Rules
**The One Title Rule.** Every page title is the page Heading at `heading-lg`. `templates/pageTitle.usage.spec.ts` fails a raw `<h1>` outside QuestionPage, a title at another step, or a level-1 Heading in another variant; `expectOneFramedHeading` holds one `<h1>` per screen inside the Template's frame.

**The Sixteen Floor Rule.** No text is under 16px at any width (ADR-0060). Hierarchy below the headings is weight, leading, tracking and ink, never a smaller size.

**The Lined-Up Figures Rule.** Every date, amount and count is set in tabular figures. DataTable sets it on every cell and right-aligns a numeric column, header and body alike (`DataTable.svelte.spec.ts`); `base.css` also sets it on `time`, `data`, `output`, `dd` and `.tabular` (build-held).

**The No Shouting Rule.** No text is set in all caps; size, weight and tracking already make the hierarchy (`tokens.usage.spec.ts`). A component sets no font size, weight or tracking outside the scale.

## Layout

Layout is intrinsic (ADR-0024): every component answers the room it is given through container queries, never the window, and every screen is complete from 320px up. `body` is the root containment context, so a `cqi` token anywhere reads the space it sits in; `layout.usage.spec.ts` fails a width media query or a static `vw`. A media query is kept only for a stated input capability (`any-pointer: coarse`) or a preference.

Every Template owns the page frame: one `center-l` with one `--page-gutter` (16px until the column reaches 640px, growing to 48px), so the title, the filter bar, the result line and the table all start at one left edge. There is no page-wide cap; each region caps itself: prose at `--measure` (65ch), a form at `--form-max` (44rem), a fixed-share table at twice the measure. A table stops where its columns are satisfied rather than stretching across the screen.

Spacing is ten fluid steps on a 4px base (growth factor 1.5). Templates and field molecules read the rhythm by name, never a raw step.

### Named Rules
**The Named Rhythm Rule.** Space is chosen by what it separates: `--space-label` (step 1) a label from its own hint and control; `--space-related` (step 3) items that read as one group; `--space-field` (step 5) one field from the next; `--space-group` (step 7) a group, or the page title, from the next; `--space-section` (step 8) one section from the next.

**The One Left Edge Rule.** A screen's regions share the Template frame's inline-start edge; nothing is indented off it or centered away from it. Build-held through the Templates; the framed-title check is the only part a spec measures today.

**The Desk Density Rule.** A table row is 40px with a fine pointer and 44px of target plus its 1px rule when any input can tap (`--row-height`); a link or Button in a row takes the row's height as its floor, so the row never grows. Every standalone target is at least 44px in both directions (`--hit-target-min`). Inline links inside a sentence are excepted.

## Elevation & Depth

The system is flat. Depth is a tonal ladder (Card White over Working Ground over the container tints) and a hairline, never a cast shadow. The three `box-shadow`s in the build are rings, not elevation: the applied chip's inset doubling of its border (so the chip does not grow), and the radio and checkbox focus halo (a zero-offset spread in the Hairline color).

### Named Rules
**The Tone Not Shadow Rule.** A surface separates from the one under it by a step on the surface ladder or a hairline. Nothing floats.

## Shapes

One corner radius, 8px, on every Button, input, chip, card, start link and placeholder bar; there is deliberately no larger second step. The pill (999px) is a shape of its own, for a badge and a toggle track only. Strokes are fixed and never fluid: a 1px hairline everywhere, a 2px rule only as the current nav or step marker, and a 2px focus ring drawn 2px outside the box.

### Named Rules
**The One Radius Rule.** 8px or a full pill. A card is never rounder than a Button.

## Components

The signature component is the activity ledger, on records (see Overview). No list carries one: the due strip was tried on the Clients list and rejected. A component for a list earns a section here only by beating the plain text it would replace on the Clients rubric's jobs.

### Buttons
- **Shape:** gently curved (8px), 44px minimum in both directions at every width.
- **Primary:** Ink Blue fill, On Ink words, medium weight, body size; hover lightens to `primary-hover`.
- **Secondary:** transparent with a Control Edge outline and Ink words; hover fills with the Hairline tint.
- **Destructive:** Signal Red fill, On Ink words; hover dims to 85%.
- **Unavailable:** a dashed Control Edge outline with Secondary Ink words and no fill, at full opacity, so it never reads as pressed. A loading Button keeps its fill and shows a small ring that stands still under reduced motion.
- **Width:** a screen's or section's own action fills a column narrower than 37rem and keeps its own width above it; in a table cell it always keeps its own width.
- **Focus:** a 2px Ink Blue ring, 2px outside.

### Links
- **Prose and standalone:** Ink Blue, underlined (1px, offset 0.2em), hover to `primary-hover`; a standalone link holds a 44px target. An external link says "(opens in new tab)" in its visible words.
- **Start link:** GOV.UK's start button, drawn as the primary Button with a trailing arrow, for the one navigation that starts a screen's task. Once per screen at most.
- **Nav:** Secondary Ink at rest; the current section is Ink Blue with a 2px rule on the bar's own bottom edge, and `aria-current` as the signal that is not color.

### Chips (filter and contents)
- **Style:** Card White fill, Control Edge outline, Ink label text, 8px radius.
- **State:** the applied chip doubles its border inward, takes the Header Tint fill, Ink Blue text and semibold weight, without changing size.

### Badges
- **Style:** a quiet filled pill: 20% of the meaning's hue mixed over Card White in oklab, text 78% of the hue toward the ink, no outline, medium weight, a 16px icon before the word. One short status word, or a short phrase that is the state's own words ("owed", "12 days overdue", "No longer on this work"); never stretched across a row. Each variant has its own icon, hidden from assistive technology (the words carry the meaning): info, check, the warning triangle (amber), the circled exclamation `warning-circle` (red, chosen on the rendered page in both themes over an octagon and a seal), and the minus circle. The "×" belongs to a removable filter tag alone, the one control among them, so an error badge never leads with it (#2843).

### Inputs / Fields
- **Style:** Working Ground fill, Control Edge 1px outline, 8px radius, 40px minimum height, capped at the measure.
- **Focus:** the 2px Ink Blue ring, 2px outside.
- **Error / Disabled:** a Signal Red outline in the GOV.UK error pattern; disabled dims to 0.6.

### Data table
- **Header:** the Header Tint row with Secondary Ink semibold labels, flush with the first column's text on every list. It is sticky (#2839): it stays at the top of the window while its table's rows scroll and leaves with the table, so a short table inside a record never covers the next section's title. The tint is opaque in both themes and the hairline is drawn by the cell, so no row shows through. Whatever focus lands on in the body (a row's link, the row Load more moves to, the row Back returns to) keeps one header row and its hairline clear above it (`scroll-margin-block-start`, from `--row-height`), so focus is never hidden under it (WCAG 2.2 Focus Not Obscured). The record cards have no header and are unchanged.
- **Rows:** 40px (44px at a coarse pointer), and every row is this height, a day's heading row included, body-small type, a Hairline rule under each, a Header Tint on hover that is never the only cue for anything; the first column is the row header, drawn as a plain cell.
- **A row with a link is a click target as a whole (#2888).** `DataTable` does this by default whenever `rowHref` is given (the Clients, Invoices and Schedule lists, and every other table with a row link): a plain primary click anywhere on the row, or on a record card, calls `click()` on the row's one link (the `data-row-link` hook, never "the first `a`"), so `listReturn` sees the navigation it always saw and Back still lands focus on the row. Nothing is nested, the row has no role and no tab stop, and a screen reader and the keyboard are unchanged. The pointer is `pointer` over the row and the card, and the card takes the row's hover tint. It does nothing when: the click was on, or inside, an `a, button, input, select, textarea, summary, label, option, details, dialog`, a `[role=button|link|menuitem|checkbox]` or a `[contenteditable]` (`closest()`, stopping at the row), or the row-actions cell or the card's actions, which also keep the default cursor; another handler has called `preventDefault`; `event.detail` is not 1, so the second click of a double-click opens nothing; the pointer moved more than 5px since `pointerdown`; or the reader was selecting text in this row at `pointerdown` (a selection in another row does not count). Ctrl, Cmd or Shift click, and middle click, open the link in a new tab (`window.open(href, "_blank", "noopener")`); a modified or middle click on the link itself stays the browser's. Not a stretched `::after` link, which would cover the text a reader selects and copies. One known gap: the first click of a double-click has `detail` 1 and follows the row before the second arrives, so a double-click to select a word still opens the row; no delay was added to wait for a second click.
- **A row link's name carries the fact that tells two rows apart, and the cell is not read again (#2864).** The Schedule's link is named "Quinn Dunmore, Sat Nov 14, 2:00am" and the Invoices list's "Avery Estrada, INV-0031", so two rows of one Client are two links a list of links tells apart (a link's description is not in that list, which is why the name carries the fact and a client-only name with `aria-describedby` was refused). The same fact is then heard once: a column with `namedByRowLink` (the Schedule's When) is `aria-hidden` in the table and, on a compact card, with its hidden label; the cell stays drawn and keeps its place under its header, and a row without a link reads it as any cell. The Invoices list hides only the reference in its Invoice cell, which goes on to say the care and how it was billed. The accessibility tree on the rendered page shows the row as the link, the Type and the Doula.
- **Columns:** a cell keeps `--space-2` either side, so two columns are two steps apart. The first column holds a track (a long two-word name) so what follows it starts at one x in every filter state; the one flexible column (Status on the Clients list, the Client on the Invoices list) is at least its track, takes a longer value's one-line width while the frame has room, and wraps only where the frame runs out, so a table stops where its columns are satisfied and a wide window leaves its room after the table, never between a name and its amount: the Clients list, the Invoices list and the Schedule pass `DataTable`'s `columnWidths="one-line"` (#2739). A column's held width is its `track` option, a CSS length for the whole cell, insets included, so a table's column x positions are the sum of its tracks: a column holds the room its busiest real value takes (the Clients list's Date, "11 weeks postpartum", about 17.4ch), so the columns after it start at one x in every order and filter state. One-line sets the cell inset itself. The records take `recordColumns` (`auto`, cards at the reading measure; `fill`, the Clients list, cards in as many 38ch columns as the frame holds; `one`, the Schedule, one card to a line at every width), so no route reaches into `DataTable` for either (#2862). Numeric columns right-aligned in tabular figures; a meta date column holds one line.
- **Money is said once, in the Money column.** The amount and its state: owed in the amber (warning) badge, the word "owed" on every Staff screen that shows an open Invoice (the Clients list, the Invoices screen and the Engagement’s Invoices section; the Client portal keeps its own words), overdue in the red (error) badge with its day count ("12 days overdue"), written off in the red badge too (failed, as the Engagement's Invoices section draws it), every other state ("payment on its way" while a payment made has not yet arrived, paid, void, and "paid, $300.00 refunded" where some of a paid Invoice went back, so the row says it was paid and the rows that say refunded add up to "Of that, refunded") a plain word, through the one `invoiceMoneyState`. Both lists draw the cell with one component, `MoneyAmount`: the amount, then the state, in two tracks in the table (the state's `--money-state-track` wide, so amounts end on one edge and the column's header ends with them) and run on after the amount on a card. The lists take their badge from the Engagement's one color rule (`invoiceStateBadge` reads `invoiceRowStatusVariant`) and keep only amber and red, the colors that ask something of the reader; the Engagement's Status column, the one place its row says the state, also badges paid (green) and void (gray). A paid row says "$1,500.00 paid" in the table on both lists; on the Invoices list its Payment cell then shows the day alone ("Nov 10, 2026", under the "Payment" heading, the lead's track left empty so every day starts at one x), so "paid" is seen once in the row, and a screen reader reading the row whole hears "Paid" before the day, as it hears "Sent" before the Sent day. On an Invoices card, which has no heading, the date line shows "Paid Nov 10, 2026" and the headline the amount alone, or the amount and the refund ("$1,500.00 $300.00 refunded"), so the card says paid once; every other state keeps its headline word. An unpaid row keeps "Payment due Dec 12, 2026". The Status cell holds the kind, the care phase and the Contract, and a pending Request's blue badge; it never repeats a money fact, and a screen reader hears one ("$950.00, 12 days overdue"). The badge sits after the amount in a track sized to the widest badge, so amounts end on one edge. A contractor never receives an Invoice state (ADR-0008), so she never saw these badges. **The Clients list's Money cell says what the Client owes in all (#2874, decided 2026-11-14):** the total of her open Invoices with no payment on its way, across every open Engagement, with the count of Invoices ("$500.00 owed, 2 Invoices"), where it showed the one oldest open Invoice per Engagement (#741) and so could not say what a Client with a deposit and a balance owed. "Most owed first" orders by that total, so the order follows exactly what the cell says, and a Client owing two smaller Invoices sits above one owing a single larger one when the sum is bigger. The BFF returns the total and the count on each row (`owedCents`, `owedInvoiceCount`; absent for a contractor), and the owed order sorts by the same sum. The cell draws it (#2874, built 2026-11-14): the total, the badge "owed" (amber), then the count, "1 Invoice" or "2 Invoices", in the badge's own track, and a screen reader hears "$500.00, owed, 2 Invoices". The badge reads "overdue" (red) with no day count when the oldest open Invoice with no payment on its way of any of her open Engagements (the one each row carries) is late, because a day count belongs to one Invoice and the total is of several, and because a badge with its days and a count together ("62 days overdue" and "2 Invoices") is wider than the 20ch track, which would push the table out of the 75.2rem band at a 1280px window; the days are on the Invoices list. A Client who owes nothing keeps the one-line-per-Engagement words (paid, payment on its way, void, written off, a contractor's fee). A record card says the total once, in its first Engagement's line. The total draws its own two tracks with `MoneyAmount`'s CSS (the amount, then the badge and the count in the state's track), since `MoneyAmount` has no place for a count; its state and amount are not `invoiceMoneyState`'s.
- **Date column: a live birth is weeks, not a day.** "3 weeks postpartum" (under fourteen days, "5 days postpartum"), counted from the birth day to today; the birth day stays for a screen reader in a `<time datetime>` with a visually hidden ", born Oct 18, 2026". A pregnancy that ended in a loss or an unknown outcome keeps "Ended Oct 30, 2026" and is never called postpartum.
- **A Visit's When** is drawn the one way wherever a Visit is listed, by `VisitWhen`: "Fri Nov 13, 2:00pm", the comma always drawn, in a `<time datetime>` that keeps the exact instant, so the Schedule and the Engagement's Visits read character for character the same, and a card runs it on as one phrase. In a table the day holds a track of its own (the longest day of this year, "Wed Nov 25,", a constant and not the longest among the rows loaded) so every weekday starts at one x, and the clock holds a track ending on one edge in tabular figures, a word space between the two; a day with a year runs past its track as the one exception. The Schedule holds its Client, When and Doula tracks the same way the Clients list holds its Name, so those columns start at one x in every filter state.
- **Applied filters are tags after Apply.** One removable tag per applied filter ("Who owe money", "Doula: Priya Raman", "Name: Camille"), each a link to the list minus that one filter, named "Remove filter: Doula Priya Raman", then "Clear filters" where two or more are on (beside one tag its × is the clear; a filter on that draws no tag keeps the link); they sit on Apply's line in the room it leaves and wrap under it only where that runs out, and nothing is reserved for them, so the default list loses no row and no empty band sits between Apply and the count. The narrowing boxes sit in one group under the legend "Show only", so "Clients" is said once, by the page.
- **The Clients list's Doula cell folds, it does not widen the table (#2862).** In the table a Client's Engagement with one Doula says her name; with two or more it says the first name and a count, "Renata Alvarez +1 more", "Renata Alvarez +4 more". The full list is in the cell for a screen reader, which hears every name, and a record card, which wraps, says them all. The rule is one name, not two, for a measured reason: with the Date held at its longest value a Postpartum row's Status line (290px) is about 17px short at a 1207px frame, rows were 71px, and a pair of names ("Renata Alvarez, Delia Marchetti", 248px) is the widest cell in the demo Practice that is not held. The Doula column gives up that width, so the ticket's rule that the 75.2rem band does not rise holds: every row is 40px in every order, seven whole rows show on the first 1280×800 screen in the default and the name order, and a Client with five Doulas draws no sideways scroll from the 55rem floor to the widest window (the Doula folded under the Date in the middle band folds the same way). Two names were tried again on 2026-11-14 (the #2862 call asked for them if Money held a typical row on one line and seven rows stayed whole), with the Money cell redrawn as the total and the count (#2874; it did not widen the column, 302.6px before and after) on the demo Practice at a 1280×800 window: two names take the Doula column from 209.7px to 268.7px and the Status column falls from 318.3px to 292.4px, 26px short of what its Postpartum line needs, so those rows are 71px tall and the first screen holds six whole rows, not seven (measured under "Most owed first"). One name stays. A wrap was considered and refused: a wrapping Doula column shares the table's shortfall with Status and makes every Postpartum row two lines again.
- **A refused "Load more" reads "Try again" (#2864).** While `loadMoreError` is set, `DataTable`'s button reads "Try again" and the notice beside it says "Try again" too, so the words name a control that is on the screen; the one button keeps its place and its focus, and reads "Load more" again once a page lands. Decided once, in `DataTable`, for every list.
- **Narrow:** below a 55rem frame the table becomes a record view, a different tree: the name, the next field, then the rest on one line, labels kept for a screen reader. Cards fill the frame, two to a row where it holds two.
- **Clients list bands (frame width, measured):** records below 55rem; from 55rem a table with the Doula folded under the Date, the Status free to wrap to two lines and every row two lines tall, so the rows are equal; from 75.2rem (the busiest typical page on one line: the floor check measures about 1200px) the Doula has her own column and every row is one line. The result line sits on its own row at the frame's left edge, directly above the table, wrapping at the bar's width (a status line, not prose, so no reading measure caps it) while its own box, and so its focus ring, hugs its sentence and as tall as the longest count a filter state can say (drawn unseen under it): one line where that fits on one (about 1115px at a 1207px bar, so from a frame of about 1170px), two below, so no filter state moves the table; a typed name long enough to outrun it is the one state that can. At rest it is one line from 1170px up, the Invoices list's too, so the gap from the line to the table is the same on both lists (18px at a 1280px window, 12px at 375px, where both say two lines). A 1280px window gives a 1207px frame, so it is in the one-line band and its first screen holds seven whole rows (a Badge-bearing row takes its pill's padding back, so every row is 40 to 43px). The narrowing boxes ("Who owe money", "Active, with no Contract", "Mine") sit in one "Show only" group on the row above the fields, with "Also show anyone not yet in care" after them. Every choice control in a filter bar's switches row, these boxes and the Invoices list's radios alike, is GOV.UK's small size (a 24px box or circle, the 44px target kept), set by `FilterBar` so both lists match.
- **Invoices list (frame width, measured):** records below 71rem (five columns, measured at 1136px on 2026-11-13 with the care kind in the Invoice cell, the Money column held at a four-figure amount and the Client at a 16ch track; the table hugs its columns, so the held Money, Invoice, Sent and Payment columns are what this width is), a table from it. Four compact totals come first (a note such as "on 17 Invoices" wraps whole) (`SummaryList` `isCompact`): the owed pair and the paid pair side by side where the row holds both and stacked where it does not, with no query, the figures on one edge. Then the Clients list's bar: a row of three inline radios after a visible "Show": "All", "Unpaid", "Overdue" (`RadioGroup` `legend` and `isInline` in `FilterBar` `switches`, small, as below), on one line from a 375px window and wrapping within the group below it, then the "Reference or Client" search with Apply after it. The applied tags follow Apply on its line, in the room already empty beside it (`FilterBar` `applied`), and nothing is reserved for them: at rest the result line sits the related step under Apply, and applying one view moves neither Apply, the line nor the first row where the bar is 30rem or wider (1280px among them). Where the bar is narrower than 30rem the search narrows to share its line with Apply, and an applied tag takes the line under them, so applying a view there moves the list by that line; at 375×740 the first record card is whole on the first screen. A view and a search together may wrap the tags to a second line, which moves the list by that line. The Money cell is the Clients list's (`MoneyAmount`, above): amber owed, red overdue with its days, red written off, and paid, void, payment on its way and "paid, $300.00 refunded" as plain words. A search or view with no match says so once: the result line is the live carrier a screen reader hears, and the empty state draws the same sentence for sight only (`DataTable` `isEmptyMessageSpoken`), as on the Clients list. The state's track is as wide as the longest state the list can say, the Money column at least a four-figure amount and that track, and Invoice, Sent and Payment as wide as the longest each can say, so every column starts and the amounts end at one place in every view, searched or not. The Invoice column (its heading "Invoice": the cell is three facts that identify one, not a reference alone) says the reference, then the care the Invoice bills for in the Clients list's words (Birth, Postpartum), then how it was billed ("INV-0031 · Birth · By hand"), so the list reads as a doula Practice's book and two Invoices of one Client for different Engagements read apart; the cell says the reference in the table too, so the cell under "Invoice" holds it (WCAG 1.3.1), though the row link says it as well. A search with no match in the Unpaid or Overdue view offers to search all Invoices for the same text. The day an Invoice was created is labeled Sent on every Staff screen (the founder's decision of 2026-11-12), and a Refund is Refunded there (GLOSSARY). Each row's one link is named by the Client and the reference (`DataTable` `rowLinkLabel`), so two Invoices of one Client are two links told apart. Back from an Invoice reads as many pages as were open and returns to the same scroll position, with focus on the row she left from.
- **Schedule (frame width, measured):** records below 55rem (880px; measured on 2026-11-13: a 879.7px frame draws the records, a 883.5px frame the table), a table from it. Re-measured on 2026-11-13 with the Type column added (#2870): the floor did not move, since a 879.7px frame still draws the records and a 881.6px frame the table, which is 853.5px wide there with no overflow, and 888px wide at a 1207px frame. The records are one column at every width below the table, never two to a row, because the list is read in time order, down and not across (the route's own rule in `ScheduleScreen.svelte`). Four columns: Client (a `grows` track of 15.2ch, the longest name the seed holds, so no narrowing widens it), When (a day track of 10.5ch and a clock track of 7ch, from `VisitWhen`), Type (the Engagement's words, "Prenatal", "Birth" or "Postpartum", as plain text with a track of 10ch, so the Doula starts at one place whichever types the days hold; in a card it joins the Doula's line, "Prenatal · Priya Raman", and adds no line) and Doula (at least 42ch, a long name with its "No longer on this work" badge, so the table is as wide with one Doula's Visits as with all of them). The bar is the Clients list's: "Days" with three inline radios, "Next 7 days", "Next 30 days" and "Choose days" (`RadioGroup` `isInline` in `FilterBar` `switches`), "Show only" "Mine" for a Doula where another Doula holds the role (as on the Clients list), the Doula select opening on "Any Doula" for an Owner or Admin, then Apply. From and To are drawn only under "Choose days", with one hint for the pair ("Both days are included.", `DateRangeFields` `hint`) and a range error drawn once over the pair (`fromError` and `toError`). A preset of N days is today and the N - 1 days after it. The address is the state: `?days=7`, `?from=&to=`, `?mine=true`, `?doula=`, and the bare path is the next 30 days. The applied tags are "Next 7 days", "Days: Tue Nov 10 to Thu Nov 19", "Mine", "Doula: Priya Raman" and "Type: Postpartum"; the default days draw none. The result line says the count, the days, whose and the order, and a day never breaks across a line: "Showing 12 Visits from Fri Nov 13 to Sat Dec 12, soonest first." and, narrowed, "Showing Priya Raman's 4 Visits from Fri Nov 13 to Thu Nov 19, soonest first." While more exist it says how many there are in all, which the first page carries: "Showing the first 30 of 42 Visits from Fri Nov 13 to Sat Dec 12, soonest first." An empty result offers the opening days first, keeping Mine, the Doula or the type ("Show the next 30 days", `scheduleEmptyNextStep`); on the opening days, "Show every type of Visit" when a type is chosen, then "Show every Doula's Visits". A Doula no longer on the Engagement is drawn by `VisitDoula` as the name, then the amber badge "No longer on this work", on the Schedule and on the Engagement's Visits. A row opens its Engagement at that Visit (`#visit-<id>`), and Back restores the loaded rows and the position, with focus on the row (`listReturn`).
- **What was billed is said once, first, in the totals (#2871, built 2026-11-14).** "Billed $12,790.00 on 14 Invoices" is the first total, in a `SummaryList` of its own beside the owed pair and the paid pair (three lists on one line where the row holds them, so the first row at 1280px does not move), with no period too ("the whole book"). Billed is the Invoices sent that still stand as a bill: open, paid (a refunded one too, in full) and written off count; a draft never reached the Client and a void one returned its amount to its Contract (GLOSSARY.md, Invoice), so neither does. Owed plus paid plus written off is billed, so the figures beside it add up to it. The BFF returns `billedCents` and `billedCount`, which follow the period and ignore the view and the search like every total. On a 375px phone the fifth row is one line more above the first card (29px). **A finished download says so in place (#2871):** a status `Notice` in the error Notice's spot under the title row, "Downloaded invoices-2026-11-14.csv, 17 Invoices." (the server's file name and its `Invoice-Count`), polite, cleared when the list changes or another download starts; GOV.UK's confirmation pattern in place, as the Notification banner row says.
- **The Invoices list's period and its download (#2871, built 2026-11-14).** A `Sent` select in the bar's `folded` fields (Any time, This month, Last month, This year, Last year, Choose days) narrows to the day an Invoice was sent, in the Practice's zone. It applies when changed, and "Choose days" opens the typed From and To (`DateRangeFields`, "Both days are included.") and sends nothing until Apply. The address is `?sent=last-month`, a preset by name so a shared link means the same month later, or `?sentFrom=&sentTo=`; the app turns a preset into two days for the BFF. The tag is in fixed words ("Sent in October 2026", "Sent in 2026", "Sent Oct 3 to Oct 17, 2026"), a whole month or year read as one, and a preset and chosen days that name the same days read alike. While a period is applied the four totals follow it, and one line of meta text over them, "Totals for Invoices sent in October 2026.", says so and describes both lists; with none there is no line and the totals are the whole book's. The result line names the period ("Showing 4 unpaid Invoices sent in 2026, earliest payment due first.") and an empty one offers "Show Invoices sent any time". Below a 42rem bar the select folds with Order behind the summary, which reads "More filters": "More filters and order" did not fit beside the search and Apply at 375px, and the summary shares Apply's line only in the short form. **Download CSV** is a secondary button on the title row for an Owner or Admin, after the heading, with a visually hidden description ("A spreadsheet of the 35 Invoices this list shows."); it is drawn only once script runs, from the first frame so the title row never changes height, and is unavailable until the list has landed. Its margin takes the difference between the title row's height and a 44px target out of its own block margin so the row, and the list under it, hold still. It posts the applied view, search, order and period, so every matching Invoice leaves, not the 30 loaded, and the file keeps the server's name. A refusal, a rate limit or no connection is said in a Notice under the title row ("Your Invoices did not download. Nothing you saved is lost.") with the button still there, and each download is one Activity entry that reads "Invoices downloaded. 14 Invoices: Unpaid, sent in October 2026, with a search", saying whether a search was applied and never its text.
- **The Schedule's days** sit each under its own heading, "Today, Fri Nov 13", "Tomorrow, Sat Nov 14", then the day as a row writes it, with the year only when it is not this one. In the table it is a heading row over its day's `<tbody>`, one `<th scope="rowgroup">` across every column, at the row height, semibold in the ink, with no tint, so it is never mistaken for the header or a hover. In the cards it is an `<h2>` across the grid. It is never stuck to the window. Each row keeps its whole When, so the Schedule and the Engagement still read the same, and a heading left open past midnight is still true about its date. Its days and times are the Practice's own (ADR-0036), and a reader whose zone differs is told once, in a line of meta text under the result line.
- **The ordering column** carries `aria-sort` on its header and no other column does (`orderedBy`, #2869); the Schedule's is When, ascending, and descending under "Latest first".
- **Loading:** rows stay up and are marked busy while a newer filter loads; a Skeleton reserves exactly the row height.
- **A read that fails** is said in place of the list in the list's own words ("Your Invoices did not load. Nothing you saved is lost."; on the Schedule, "Your schedule did not load. Nothing you saved is lost.") with "Try again", and the bar keeps the view and the typed search; a failed "Load more" keeps the rows and says so beside the button, never in the server's words.

### Filter bar
- **Style:** a plain form above the result line, no box of its own. Checkboxes and Apply stay in view at every width; the text and select fields fold into a disclosure below a 42rem bar. The result line is Secondary Ink at meta size.
- **Arriving narrowed:** a Doula who is neither Owner nor Admin arrives on the Clients list and the Schedule already narrowed to "Mine", by the link that brought her. The bare address is the whole Practice for every reader, so one shared link is one list, and the "Mine" tag's × or "Clear filters" is one action to it. A box, a radio and a select take effect when changed (#2888, decided 2026-11-14): `FilterBar`'s `onChangeApply` waits 250ms for the next change, holds a select the keyboard stepped until Enter or until focus leaves it, keeps focus on the control, replaces its own history entry while focus stays on that control, and sends the whole bar as drawn. A typed field (a name, a search, a date) waits for Apply or Enter, and Apply stays drawn at every width in the same place. While the list is empty the bar draws no result-line ruler, so the empty state sits one line under the tags, not two.
- **Doula no longer on this work (#2863, #2888):** a "Show only" box beside "Mine" on the Schedule, offered to an Owner or Admin only, as the Doula select is. It takes the words of the badge on a row (`NO_LONGER_ON_THIS_ENGAGEMENT`), so one condition has one name; its tag is the same words. The result line says it applied ("Showing 2 Visits whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, soonest first."), and, for rows not narrowed to them, how many there are after the line ("2 have a Doula no longer on this work."), never on the box. Empty: "No Visit has a Doula no longer on this work from Fri Nov 13 to Sat Dec 12." with "Show the next 30 days". The address stays `?needsDoula=true`.
- **The Schedule's bar (#2870):** row one is the switches, "Days" (three radios, one stop) then the links "Earlier" and "Later"; row two is the fields, "Client" first, then (folded below a 42rem bar) "Show only", the Doula select and Type, then "Order", Apply and the tags. Below 42rem, "Show only", the Doula select, Type and Order fold under "More filters and order", for every reader (a contractor, or a Doula at a one-Doula Practice, has Type and Order to fold); Days, Earlier, Later, the Client search, Apply, the tags and the result line never fold. At 375px the radios take two lines with "Earlier" and "Later" together in the second line's empty end, "Client" shares a line with the summary, and Apply follows.
- **The Clients list's Doula select is held from the first paint (#2864).** For an Owner or an Admin it is drawn before the roster lands, disabled at its own width and holding the address's Doula with an empty label (or "Any Doula" where the address names none), and enabled in place when the roster lands; a roster that lands empty leaves it drawn and enabled with "Any Doula" alone. The "More filters and order" summary follows the same test, so it never becomes "Order". No Doula tag and no result line are drawn until the roster lands (the count in "Clear filters" includes the pending Doula, and an unseen chip holds the tag's box, so where a narrow bar gives the tag its own line under Apply, as at 1024px, that line is there from the first paint and the list does not move when the tag appears), and an empty list waits for it too, since its sentence names her. Her name is read from the whole roster, a Doula who holds no Client included; "that Doula" is left only for a roster that could not be read or an id on no roster.
- **An empty Clients result offers one next step, chosen by what emptied it (#2864).** A Name with other filters, or a Name among the Clients in care: "Search every Client for "Cam"" (a button; the Name and the order kept, every box and the Doula off, "Also show anyone not yet in care" on). A Name already over every Client: "Find or add a Client" ("Add a Client" while the Practice holds none). Filters and no Name: "Show every Client in care" (the order kept). A Doula who opens on Mine, with other filters on: "Show all your Clients" (Mine kept). Mine alone, or the default list with nobody in care, offers none: no other list holds a Client of hers, and "Also show anyone not yet in care" changes nothing beside Mine.
- **The Client search field (#2870):** an unnamed `type="search"` field (`maxlength` 100), first in the fields row. A Client's name is in no address (#1829), so it rides in history state and the read is a POST; its tag is "Client: Camille". It is applied with Apply or Enter, and sent as drawn with any control that applies when changed.
- **The Visit's type (#2888):** a `Type` select in the fold after the Doula, offered to every reader: "Any type" (the empty value), "Prenatal", "Birth", "Postpartum", the words the Type column draws. The address is `?type=prenatal|birth|postpartum`, the empty value left out; the BFF takes `type` on the read and on the search's POST body, and the count and the rows read the same WHERE. It applies when changed (#2888). Its tag is "Type: Postpartum". The result line puts the type before the noun in lowercase ("Showing 3 postpartum Visits from Fri Nov 13 to Sat Dec 12, soonest first.", "Showing Priya Raman's 2 birth Visits from …"), and an empty result says "No postpartum Visits from Fri Nov 13 to Sat Dec 12." On the opening days the way on is "Show every type of Visit"; on other days "Show the next 30 days" keeps the type. At 1280px the select holds 13ch and the Doula select 17ch so Client, "Show only", Doula, Type, Order and Apply stay on one line; it adds one Tab stop at 1280px and none at 375px with the fold closed. The ruler holds the longest typed line (postpartum, a Doula and a search).
- **Earlier and Later (#2870):** each moves the applied days by their own length, 7 for the next 7 days, 30 for the next 30, and to − from + 1 for chosen days, as calendar days in the Practice's zone (a daylight-saving day or a month's end moves nothing). The address is `?from=&to=`; a result that is the next 7 or 30 days counted from today is written as that preset (`?days=7`, or the bare path). The names are "Earlier, Fri Nov 6 to Thu Nov 12", the days it lands on. In a stepped state "Choose days" is the checked radio with From and To filled, and the Days tag's × is one action back to today's default. A step keeps Mine, the Doula, the narrowing to Visits waiting, the order and the search; "Load more" stays inside the window, and under latest first "Earlier" still steps the window earlier.
- **A reader-chosen order (#2869):** an `Order` select is the last field before Apply where the bar is 42rem or wider, and folds below it, reversing the first call on #2869 (decided 2026-11-13). `FilterBar` draws it inside the fold after the folded fields, so wide nothing moves; narrow it is behind the summary, which is the route's wording: "More filters and order" where other filters fold with it (the Clients list's Doula and "Also show anyone not yet in care"), "Order" where only Order folds (the Invoices list and the Schedule), and then the summary sits beside Apply on Apply's own line and costs no line. An applied order is not counted in the summary's "Filters: N on", and the fold stays closed while one is applied, since the result line says it. A select inside a closed `<details>` is still sent, so Apply keeps the order with no script. **Width:** the select's cell is held at the drawn width of its longest option (that label drawn unseen in the same cell, as the result line's ruler is), so a pick or a view never moves Apply or the fields; no route passes a width for it. **Tags, all three lists:** a tag sits beside Apply where it fits, and otherwise takes one line under Apply and the list moves by exactly that line (its height and the row gap); Apply, Order and the fields never move. A chosen order draws no tag, because the result line says it ("…, latest first.", at every count from one Visit up) and the select holds it, so it is said once on the page; "Clear filters" is the way back to the list's own order, drawn beside a chosen order even when exactly one other tag is on (`FilterBar`'s `hasUntaggedFilter`), and it is the one control that shows and undoes the order where the fold is closed. It takes effect when changed (#2888), the list's own order is the empty choice (labeled with the words the list is in, "Soonest first"), and a chosen order is said by the result line and undone by "Clear filters". The table marks the ordering column with `aria-sort` (`DataTable`'s `orderedBy`); its headers are not buttons, so the bar is the one place the order is chosen.

### Navigation
- **Style:** the Staff top bar is a night-blue wall in both themes (The Night Walls Rule), 60px tall, with Secondary Ink items and the current item's 2px Ink Blue rule on its bottom edge. Below a 61.75rem bar the items move into a narrow sheet, where the current item's rule turns onto the leading edge.

## Do's and Don'ts

### Do:
- **Do** spend the primary only on a link, the primary action or start link, the current place, the focus ring and the selection tint.
- **Do** pick a status hue by meaning: amber needs someone to act (owing money is one such thing), red late or failed, blue information or anything pending or waiting, green success, gray neutral.
- **Do** set every date, amount and count in tabular figures, and right-align a numeric column.
- **Do** draw every page title as the page Heading at `heading-lg`, inside the Template's frame, on the frame's left edge.
- **Do** separate surfaces by a step on the surface ladder or a hairline.
- **Do** answer the container with a container query, and check every screen at 320px and in both themes.
- **Do** read spacing by its named rhythm (`--space-label`, `--space-related`, `--space-field`, `--space-group`, `--space-section`).

### Don't:
- **Don't** use pure white or pure black, in either theme.
- **Don't** name a color, a px length or a font size of a component's own; spend a token (`tokens.usage.spec.ts`).
- **Don't** set text in all caps or under 16px.
- **Don't** add a second corner radius or a cast shadow.
- **Don't** give a new state a new hue, or use amber for anything but needs someone to act, or red for anything but late or failed.
- **Don't** use a width media query or a static `vw` for layout.
- **Don't** put a start link beside a primary Button.
