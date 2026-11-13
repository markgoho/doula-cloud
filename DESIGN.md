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

The Staff app is the category standard for a working tool, executed at the craft level of Linear, Stripe Dashboard and GOV.UK services (PRODUCT.md, Brand Commitments, chosen 2026-11-08). A doula or an owner opening Clients should know where everything is before noticing the look: a top bar with the nav, a title with one start action beside it, a filter bar, a result line, a table. Nothing about the arrangement is novel. The character is that nothing is out of place: every row is the same height, every number lines up, every badge means one thing, and both themes are proved, not hoped for.

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
- **Style:** a quiet filled pill: 20% of the meaning's hue mixed over Card White in oklab, text 78% of the hue toward the ink, no outline, medium weight, a 16px icon before the word. One short status word; never stretched across a row.

### Inputs / Fields
- **Style:** Working Ground fill, Control Edge 1px outline, 8px radius, 40px minimum height, capped at the measure.
- **Focus:** the 2px Ink Blue ring, 2px outside.
- **Error / Disabled:** a Signal Red outline in the GOV.UK error pattern; disabled dims to 0.6.

### Data table
- **Header:** the Header Tint row with Secondary Ink semibold labels, flush with the first column's text on every list. It is sticky (#2839): it stays at the top of the window while its table's rows scroll and leaves with the table, so a short table inside a record never covers the next section's title. The tint is opaque in both themes and the hairline is drawn by the cell, so no row shows through. Whatever focus lands on in the body (a row's link, the row Load more moves to, the row Back returns to) keeps one header row and its hairline clear above it (`scroll-margin-block-start`, from `--row-height`), so focus is never hidden under it (WCAG 2.2 Focus Not Obscured). The record cards have no header and are unchanged.
- **Rows:** 40px (44px at a coarse pointer), body-small type, a Hairline rule under each, a Header Tint on hover that is never the only cue for anything; the first column is the row header, drawn as a plain cell.
- **Columns:** a cell keeps `--space-2` either side, so two columns are two steps apart. The first column holds a track (a long two-word name) so what follows it starts at one x in every filter state; the one flexible column (Status on the Clients list, the Client on the Invoices list) holds a track and wraps only where the frame runs out, so a table stops where its columns are satisfied and a wide window leaves its room after the table, never between a name and its amount: both lists pass `DataTable`'s `columnWidths="one-line"` and set the track as `--data-table-grows-track` (#2739). Numeric columns right-aligned in tabular figures; a meta date column holds one line.
- **Money is said once, in the Money column.** The amount and its state: owed in the amber (warning) badge, the word "owed" on every Staff screen that shows an open Invoice (the Clients list, the Invoices screen and the Engagement’s Invoices section; the Client portal keeps its own words), overdue in the red (error) badge with its day count ("12 days overdue"), written off in the red badge too (failed, as the Engagement's Invoices section draws it), every other state ("payment on its way" while a payment made has not yet arrived, paid, void, and "$300.00 refunded" where some of a paid Invoice went back, so the rows that say refunded add up to "Of that, refunded") a plain word, through the one `invoiceMoneyState`. Both lists draw the cell with one component, `MoneyAmount`: the amount, then the state, in two tracks in the table (the state's `--money-state-track` wide, so amounts end on one edge and the column's header ends with them) and run on after the amount on a card. The lists take their badge from the Engagement's one color rule (`invoiceStateBadge` reads `invoiceRowStatusVariant`) and keep only amber and red, the colors that ask something of the reader; the Engagement's Status column, the one place its row says the state, also badges paid (green) and void (gray). A paid row says "$1,500.00 paid" on both lists; on the Invoices list its Payment cell then says the day alone ("Nov 10, 2026", under the "Payment" heading, the lead's track left empty so every day starts at one x), so "paid" is said and heard once in the row, while an unpaid row keeps "Payment due Dec 12, 2026". The Status cell holds the kind, the care phase and the Contract, and a pending Request's blue badge; it never repeats a money fact, and a screen reader hears one ("$950.00, 12 days overdue"). The badge sits after the amount in a track sized to the widest badge, so amounts end on one edge. A contractor never receives an Invoice state (ADR-0008), so she never saw these badges.
- **Date column: a live birth is weeks, not a day.** "3 weeks postpartum" (under fourteen days, "5 days postpartum"), counted from the birth day to today; the birth day stays for a screen reader in a `<time datetime>` with a visually hidden ", born Oct 18, 2026". A pregnancy that ended in a loss or an unknown outcome keeps "Ended Oct 30, 2026" and is never called postpartum.
- **Applied filters are tags after Apply.** One removable tag per applied filter ("Who owe money", "Doula: Priya Raman", "Name: Camille"), each a link to the list minus that one filter, named "Remove filter: Doula Priya Raman", then "Clear filters" where two or more are on (beside one tag its × is the clear; a filter on that draws no tag keeps the link); they sit on Apply's line in the room it leaves and wrap under it only where that runs out, and nothing is reserved for them, so the default list loses no row and no empty band sits between Apply and the count. The narrowing boxes sit in one group under the legend "Show only", so "Clients" is said once, by the page.
- **Narrow:** below a 55rem frame the table becomes a record view, a different tree: the name, the next field, then the rest on one line, labels kept for a screen reader. Cards fill the frame, two to a row where it holds two.
- **Clients list bands (frame width, measured):** records below 55rem; from 55rem a table with the Doula folded under the Date, the Status free to wrap to two lines and every row two lines tall, so the rows are equal; from 75.2rem (the busiest typical page on one line: the floor check measures about 1200px) the Doula has her own column and every row is one line. The result line sits on its own row at the frame's left edge, directly above the table, as wide as the bar (a status line, not prose, so no reading measure caps it) and as tall as the longest count a filter state can say (drawn unseen under it): one line where that fits on one (about 1115px at a 1207px bar, so from a frame of about 1170px), two below, so no filter state moves the table; a typed name long enough to outrun it is the one state that can. At rest it is one line from 1170px up, the Invoices list's too, so the gap from the line to the table is the same on both lists (18px at a 1280px window, 12px at 375px, where both say two lines). A 1280px window gives a 1207px frame, so it is in the one-line band and its first screen holds seven whole rows (a Badge-bearing row takes its pill's padding back, so every row is 40 to 43px). The narrowing boxes ("Who owe money", "Active, with no Contract", "Mine") sit in one "Show only" group on the row above the fields, with "Also show anyone not yet in care" after them. Every choice control in a filter bar's switches row, these boxes and the Invoices list's radios alike, is GOV.UK's small size (a 24px box or circle, the 44px target kept), set by `FilterBar` so both lists match.
- **Invoices list (frame width, measured):** records below 70.2rem (five columns, measured at 1123px on 2026-11-13 with the care kind in the Reference cell and the Money column held still; the table hugs its columns, so the held Reference, Sent, Payment and state tracks are what this width is), a table from it. Four compact totals come first (a note such as "on 17 Invoices" wraps whole) (`SummaryList` `isCompact`): the owed pair and the paid pair side by side where the row holds both and stacked where it does not, with no query, the figures on one edge. Then the Clients list's bar: a row of three inline radios after a visible "Show": "All", "Unpaid", "Overdue" (`RadioGroup` `legend` and `isInline` in `FilterBar` `switches`, small, as below), on one line from a 375px window and wrapping within the group below it, then the "Reference or Client" search with Apply after it. The applied tags follow Apply on its line, in the room already empty beside it (`FilterBar` `applied`), and nothing is reserved for them: at rest the result line sits the related step under Apply, and applying one view moves neither Apply, the line nor the first row where the bar is 30rem or wider (1280px among them). Where the bar is narrower than 30rem the search narrows to share its line with Apply, and an applied tag takes the line under them, so applying a view there moves the list by that line; at 375×740 the first record card is whole on the first screen. A view and a search together may wrap the tags to a second line, which moves the list by that line. The Money cell is the Clients list's (`MoneyAmount`, above): amber owed, red overdue with its days, red written off, and paid, void, payment on its way and "$300.00 refunded" as plain words. A search or view with no match says so once: the result line is the live carrier a screen reader hears, and the empty state draws the same sentence for sight only (`DataTable` `isEmptyMessageSpoken`), as on the Clients list. The state's track is as wide as the longest state the list can say, and Reference, Sent and Payment are as wide as the longest each can say, so the amounts end on one edge in every view, searched or not. The Reference cell says the reference, then the care the Invoice bills for in the Clients list's words (Birth, Postpartum), then how it was billed ("INV-0031 · Birth · By hand"), so the list reads as a doula Practice's book and two Invoices of one Client for different Engagements read apart; the cell says the reference in the table too, so the cell under "Reference" holds it (WCAG 1.3.1), though the row link says it as well. A search with no match in the Unpaid or Overdue view offers to search all Invoices for the same text. The day an Invoice was created is labeled Sent on every Staff screen (the founder's decision of 2026-11-12), and a Refund is Refunded there (GLOSSARY). Each row's one link is named by the Client and the reference (`DataTable` `rowLinkLabel`), so two Invoices of one Client are two links told apart. Back from an Invoice reads as many pages as were open and returns to the same scroll position, with focus on the row she left from.
- **Loading:** rows stay up and are marked busy while a newer filter loads; a Skeleton reserves exactly the row height.
- **A read that fails** is said in place of the list in the list's own words ("Your Invoices did not load. Nothing you saved is lost.") with "Try again", and the bar keeps the view and the typed search; a failed "Load more" keeps the rows and says so beside the button, never in the server's words.

### Filter bar
- **Style:** a plain form above the result line, no box of its own. Checkboxes and Apply stay in view at every width; the text and select fields fold into a disclosure below a 42rem bar. The result line is Secondary Ink at meta size.

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
