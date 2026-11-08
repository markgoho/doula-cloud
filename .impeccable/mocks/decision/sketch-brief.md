# Sketch brief: the Staff Clients list's first screen, in one visual world

Write ONE self-contained HTML file: the first viewport of DoulaCloud's Staff Clients list at 1440 x 900. `html, body { width:1440px; height:900px; overflow:hidden; margin:0 }`. Inline CSS only, no JavaScript, nothing fetched except Google Fonts. Under 200 lines. Real content below, verbatim.

## Mode rules (Operate: a working screen)
- Someone who uses software like this must know where everything is before they notice the world. The world lends four things only: type, palette, density, and ONE signature move. It never supplies the layout, the navigation model or the controls.
- Color does real jobs: a committed palette for the shell (a colored top bar or a tinted ground), state colors (money owed, overdue), selection, and the signature move. A gray screen with one accent is the category default, not restraint.
- Every control is a standard web control: real-looking checkboxes, a text input, a select, a button, a link, a table with headers. Never a costume of the world's own objects (no ledger paper standing in for a table, no fake stamps standing in for buttons).
- Finish bar: an aligned grid, consistent spacing, a real type scale, clear hierarchy. The signature move adds to a screen that is already that clean.
- Refuse: eyebrow/kicker labels above headings, gradient text, decorative glass, colored left/right borders thicker than 1px on rows or cards, emoji or unicode glyphs as icons, cards as the page structure, hard offset shadows.
- Avoid these looks entirely: clinical/medical, generic SaaS (Inter + purple-blue), baby-brand cute (pastels, bubbly type), spa/wellness (sage, beige, airy serif). Do not use these over-used faces: Inter, Fraunces, Playfair Display, Cormorant, Lora, Crimson, Newsreader, Syne, Space Grotesk, Space Mono, IBM Plex, DM Sans, DM Serif, Outfit, Plus Jakarta Sans, Instrument Sans. Pick faces from Google Fonts whose character fits the world. Body text at least 15px, AA contrast.

## Content (verbatim)
- Top bar: product "DoulaCloud"; nav: Overview, Clients (current), Schedule, On call, Invoices, Contracts, Credits, Staff, Offers, Settings; right side: "Rooted Birth Collective" and an avatar "RA".
- Page title "Clients", with the primary action "Find or add a Client" beside it (left-aligned with the title, not at the far right).
- Filter bar, legend "Choose which Clients to show": checkboxes "Only my Clients", "Only Clients who owe money", "Only active Clients with no Contract", "Also show Clients not yet in care"; then a "Name" text field, a "Doula" select showing "Any Doula", an "Apply" button.
- Result line: "Showing the first 30 Clients, soonest due first."
- Table columns: Name | Status | Due | Doula. Names are links. In Status, the care kind (Birth / Postpartum) in bold, then any call to act as a badge (only "Owes …" and "… overdue …" are badges), then quiet facts separated by " · ".
  1. Casey Estrada | Birth · Intake · No Contract yet | Nov 9, 2026 | Rowan Petrosyan
  2. Quinn Estrada | Birth · Intake · No Contract yet | Nov 10, 2026 | Rowan Petrosyan
  3. Morgan Grier | Birth · Active · No Contract yet | Nov 26, 2026 | Marisol Terrazas
  4. Camille Boyd | Birth · [badge: Owes $1,500.00] · Active · Contract signed | Nov 28, 2026 | Priya Raman
  5. Imani Brooks | Postpartum · [badge, error: $950.00 overdue by 12 days] · Active · Contract signed | Born Oct 27, 2026 | Priya Raman
  6. Tessa Moreau | Birth · Active · Contract signed · Paid $1,500.00 | Dec 23, 2026 | Priya Raman
  7. Reese Calloway | Birth · Active · No Contract yet | Dec 31, 2026 | Joss Adeyemi
  8. Jamie Dunmore | Birth · Active · No Contract yet | Jan 3, 2027 | Joss Adeyemi
  9. Mira Castellanos | Birth · Intake · Contract signed | Jan 11, 2027 | Priya Raman
  10. Grace Whitfield | Birth · Intake · Contract sent | Jan 27, 2027 | Priya Raman
  (Show as many rows as fit; the viewport may cut the table.)

## Your world
See the prompt that sent you here. Commit to it fully in palette, type and density, and make its ONE signature move visible and functional (it must carry real data, not decorate).

Write only your one file at the path the prompt names. Do not edit anything else, run no servers, spawn no agents. Report the path and the faces and hex colors you used, in three lines.
