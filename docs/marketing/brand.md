# The DoulaCloud brand sheet

**Brand** is how the product refers to itself: name, mark, color, type, and voice. It is durable. Positioning and message change when the market answers; the brand does not.

This sheet was settled on [#1000](https://github.com/markgoho/doula-cloud/issues/1000) on 2026-09-27, from three prototypes that the founder reacted to. It is a child of the [Go to market map (#991)](https://github.com/markgoho/doula-cloud/issues/991). The map ruled that marketing has no second identity: the brand is the app's own tokens and the app's own mark, refined. So this sheet names tokens and files. It does not copy values.

## Name

The name is set by [ADR-0050](../adr/0050-the-product-is-named-doulacloud-one-word.md). This sheet does not state it again.

**Beside the mark, the name is set in one weight: semibold (600).** The capital C is the only thing that marks the join between the two words. Two other settings were shown and not chosen: two weights (regular, then semibold), and two tones (the second word in `--color-primary`). The two-tone setting also spends the accent on decoration, which [the design brief](../design/brief.md) does not permit.

The sizes are the app's. `BrandLockup` pairs the `sm`, `md`, and `lg` sizes of `CloudMark` with the `subheading`, `heading`, and `display` steps of the type scale, with a gap of `--space-3`. This sheet changes none of them.

## Mark

The mark is **two strokes**.

| Stroke                                      | As a cloud               | As a body                                                                                |
| ------------------------------------------- | ------------------------ | ---------------------------------------------------------------------------------------- |
| The outer line, a big lobe and a small lobe | The cloud                | A pregnant body that lies down: the big lobe is the belly, the small lobe is the breasts |
| The inner arch, under the big lobe          | A layer inside the cloud | The baby                                                                                 |

The second reading is design intent. It is the reason for each rule below, and it is recorded here so that a later change to the mark does not remove it by accident. Public copy does not explain the mark.

### Files

| File                                                   | Use                                                                                           |
| ------------------------------------------------------ | --------------------------------------------------------------------------------------------- |
| [`brand/mark.svg`](brand/mark.svg)                     | The mark in two tones. For a mark that is 28px tall or more.                                  |
| [`brand/mark-one-color.svg`](brand/mark-one-color.svg) | The mark in one color. For a mark that is less than 28px tall, and for one-color print.       |
| [`brand/icon.svg`](brand/icon.svg)                     | The mark on a plum tile. For each square slot: a browser tab, a home screen, a social avatar. |

These files are the reference drawings. The app draws the mark in `CloudMark.svelte` and the lockup in `BrandLockup.svelte`. The change that brings the app, its icons, and the design canvas in step with this sheet is [#1486](https://github.com/markgoho/doula-cloud/issues/1486).

### Rules

- **Two strokes, and only two.** No third layer between them.
- **The geometry does not change.** The two lobes are two circles on one baseline. They cross at the top of the small lobe. The baby is concentric with the belly.
- **Two tones when the mark is 28px tall or more.** The outer line is `--color-primary`. The inner arch is `--color-primary-hover`. 28px is the height of the mark in the app's `md` lockup.
- **One color below 28px.** The two strokes are `--color-primary`. At that size the two tones are too near to each other to read as two.
- **In a square slot, use the tile.** The two strokes are `--color-on-primary`, with a stroke of 18 units, on a `--color-primary` square with a corner radius of 14 in a 64-unit box. The tile keeps the light theme's values in the two themes, because it carries its own ground.
- **Do not crop the mark.** A crop that cuts the small lobe leaves concentric arches, which read as a rainbow.
- **Do not fill the mark.** A solid cloud loses the inner arch.

### What was judged, and what changed because of it

The mark was judged at 16px, 32px, and 48px in a square slot, in a lockup beside the name, on a 500px card between lines of ordinary text, and on a light ground and a dark ground.

| What was seen                                                                                                                                                                  | What changed                                                          |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------- |
| At 32px the three-arc mark is 15px tall, and at 16px it is 7px tall. The three arcs merge, and the innermost arc (`--color-outline-variant`) is not visible on a light ground. | The middle layer is removed. The mark is two strokes.                 |
| With three arcs, the eye sees three nested clouds. With two, it sees one body with a baby inside.                                                                              | The inner arch takes a plum tone, not the hairline tone.              |
| The open mark is about two times wider than its height, so it fills less than half of a square slot.                                                                           | A square slot uses the plum tile, with a stroke of 18 and not 14.     |
| At small sizes, the two plum tones read as one.                                                                                                                                | Below 28px the mark has one color.                                    |
| On a dark ground, the dark theme's tokens keep the outer line the strongest stroke.                                                                                            | Nothing. The mark takes the dark theme's tokens with no other change. |

One change was tried and rejected. At small sizes the outer line can read as one continuous curve, because the lobes cross at the top of the small lobe and the line only goes down from there. Three drawings moved the small lobe to the right to make a drop between the lobes (8, 16, and 24 units). Each drop made the mark wider (216 to 240 units, from 182), so the mark was drawn smaller in each slot. The deeper drops read as two arches side by side, which weakens the reading of one body. The outline stays as it is.

## Color

The palette is Plum Dusk, as [the design brief](../design/brief.md) sets it and as `app/src/lib/styles/tokens.css` holds it. The token name is the brand's name for the color. Marketing spends the token. It does not copy the value, because the value is different in the dark theme.

| Token                        | Role in the brand                                                         |
| ---------------------------- | ------------------------------------------------------------------------- |
| `--color-primary`            | The plum. The outer line of the mark, the tile, a primary action, a link. |
| `--color-primary-hover`      | The lighter plum. The inner arch of the mark at large sizes.              |
| `--color-on-primary`         | Type and strokes on the plum. The strokes on the tile.                    |
| `--color-surface`            | The ground of a page.                                                     |
| `--color-surface-bright`     | The ground of a card.                                                     |
| `--color-on-surface`         | The name, headlines, and prose.                                           |
| `--color-on-surface-variant` | Secondary text.                                                           |
| `--color-outline-variant`    | Hairlines and the edge of a card.                                         |

The brief's rules hold on each marketing surface: one accent hue, no pure white, no pure black, no drop shadow, and a card is declared by a one-pixel edge.

## Type

**Hanken Grotesk**, weights 400 to 600, self-hosted from `app/src/lib/styles/fonts/`. It is the only family, for the name, headlines, prose, and labels.

The scale is the brief's scale, through the `--text-*` tokens. A marketing page can use the `display` step more freely than the app does, and more vertical space. It adds no second family.

## Voice

Founder voice, first person, on each channel through launch. The map settled this, and it is looked at again after January 2027.

Three sentences show it. Each is from the teaser copy that [#362](https://github.com/markgoho/doula-cloud/issues/362) fixed.

> I'm building the thing you keep rebuilding in a spreadsheet.

A headline. It names the reader's work, not the product's features.

> I need an email address I can actually write to.

An error message. One person speaks, including when something fails.

> Then nothing from me until January.

A promise. It says what will not happen.

The rules:

- **Say "I", not "we".** One person signs.
- **Use contractions.** Write the way the founder talks.
- **Say what is not ready.** Make no claim that the product cannot show.
- **Name work that a solo doula and an agency both do.**
- **Use no pronoun for a Client or a doula.** Use the person's name, or the domain noun. This is the brief's voice rule, and it holds in marketing copy also.

## Departures from the app

The sheet adds no color and no typeface that the app does not carry.

Two things are different from the app as it is today, and the two are changes to the app, not departures from it. The mark has two strokes and not three, and a square slot uses the tile. [#1486](https://github.com/markgoho/doula-cloud/issues/1486) makes those changes.

One thing is superseded. The teaser prototype on [#362](https://github.com/markgoho/doula-cloud/issues/362) used indigo `#474bb5`, Schibsted Grotesk, and Newsreader, before the app had a design brief. The map ruled one identity, so the teaser build ([#358](https://github.com/markgoho/doula-cloud/issues/358)) takes the tokens, the typeface, and the mark on this sheet.

## The prototypes

The three prototypes are on the branch [`prototype/brand-sheet`](https://github.com/markgoho/doula-cloud/tree/prototype/brand-sheet). They are throwaway, and they are not for merge. Each reaction is recorded, in the founder's words, in the comments of [#1000](https://github.com/markgoho/doula-cloud/issues/1000).
