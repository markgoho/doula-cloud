---
name: Nurture & Node
colors:
  surface: '#fff8f6'
  surface-dim: '#e6d7d3'
  surface-bright: '#fff8f6'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#fff1ed'
  surface-container: '#fbeae6'
  surface-container-high: '#f5e5e1'
  surface-container-highest: '#efdfdb'
  on-surface: '#221a18'
  on-surface-variant: '#54433e'
  inverse-surface: '#372e2c'
  inverse-on-surface: '#feede9'
  outline: '#87736d'
  outline-variant: '#dac1ba'
  surface-tint: '#944931'
  primary: '#76321c'
  on-primary: '#ffffff'
  primary-container: '#944931'
  on-primary-container: '#ffcebf'
  inverse-primary: '#ffb59e'
  secondary: '#944931'
  on-secondary: '#ffffff'
  secondary-container: '#fd9d7f'
  on-secondary-container: '#77331c'
  tertiary: '#00504d'
  on-tertiary: '#ffffff'
  tertiary-container: '#006a66'
  on-tertiary-container: '#96e7e1'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#ffdbd0'
  primary-fixed-dim: '#ffb59e'
  on-primary-fixed: '#3a0b00'
  on-primary-fixed-variant: '#76321c'
  secondary-fixed: '#ffdbd0'
  secondary-fixed-dim: '#ffb59e'
  on-secondary-fixed: '#3a0b00'
  on-secondary-fixed-variant: '#76321c'
  tertiary-fixed: '#a0f1eb'
  tertiary-fixed-dim: '#84d4cf'
  on-tertiary-fixed: '#00201e'
  on-tertiary-fixed-variant: '#00504d'
  background: '#fff8f6'
  on-background: '#221a18'
  surface-variant: '#efdfdb'
typography:
  headline-xl:
    fontFamily: Playfair Display
    fontSize: 48px
    fontWeight: '700'
    lineHeight: 56px
    letterSpacing: -0.02em
  headline-lg:
    fontFamily: Playfair Display
    fontSize: 32px
    fontWeight: '600'
    lineHeight: 40px
  headline-md:
    fontFamily: Playfair Display
    fontSize: 24px
    fontWeight: '600'
    lineHeight: 32px
  body-lg:
    fontFamily: Inter
    fontSize: 18px
    fontWeight: '400'
    lineHeight: 28px
  body-md:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
  body-sm:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  label-md:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: '600'
    lineHeight: 16px
    letterSpacing: 0.05em
  headline-lg-mobile:
    fontFamily: Playfair Display
    fontSize: 28px
    fontWeight: '600'
    lineHeight: 36px
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  unit: 8px
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 40px
  container-margin: 32px
  gutter: 24px
---

## Brand & Style
The design system is built to evoke the warmth of a private journal and the reliability of a steady hand. It rejects the cold, sterile aesthetics of traditional healthcare software in favor of a "hearth" feel—designed specifically for doula agencies who balance professional management with deeply personal care.

The style is a blend of **Editorial Minimalism** and **Tonal Layering**. It prioritizes high-contrast serif typography and generous whitespace to create a sense of calm and focus. Rather than using shadows to create depth, the system relies on subtle shifts in background warmth to define different functional areas, ensuring the interface feels organic and grounded.

## Colors
The palette is rooted in earth tones, moving away from clinical blues and greys. 

- **Ground**: The base layer of the application uses a warm cream (#fff8f6), providing a soft, non-reflective canvas that reduces eye strain.
- **Tonal Stepping**: Depth is achieved through a hierarchy of warmth. Use `#fdf0ec` for secondary containers (like sidebars or list views) and `#f9e8e2` for interactive surface elements (like input backgrounds or active cards).
- **Accents**: Terracotta serves as the primary driver for action. Use the deep Primary Terracotta (#944931) for high-emphasis buttons and navigation states. The Secondary Accent (#d67d61) is reserved for softer backgrounds, indicators, and decorative accents.
- **Typography**: All text should be rendered in a deep, warm charcoal (#2d1a15) rather than pure black to maintain the organic feel.

## Typography
The typographic hierarchy creates an "Editorial" feel. 

- **Headlines**: Playfair Display provides an authoritative yet maternal voice. Its high-contrast strokes should be used for page titles and section headers to give the app the presence of a well-bound book.
- **Body & Labels**: Inter is used for all functional data and long-form reading. It provides the necessary clarity for practice management tasks, schedules, and clinical notes.
- **Scale**: Use generous line heights for body text (1.5x minimum) to ensure a "breathing" layout that prevents information density from becoming overwhelming.

## Layout & Spacing
This design system employs a **Fluid Grid** model with high internal margins to maintain a relaxed, premium feel.

- **Rhythm**: All spacing is derived from an 8px base unit. 
- **Density**: Use "Medium-High" density for data tables but "Low" density for dashboard views and intake forms. Components should never feel cramped; when in doubt, increase the `xl` padding (40px) between major sections.
- **Structure**: Desktop layouts should favor a wide left-hand navigation bar (on the `surface_sunken` tone) with a centered or wide-margin content area. This mimics the layout of a ledger or journal.

## Elevation & Depth
In this design system, **shadows are strictly prohibited.** Depth is communicated through color and linework:

1.  **Level 0 (The Ground):** The base `#fff8f6` layer.
2.  **Level 1 (The Surface):** Use `#fdf0ec` for containers that sit on the ground, such as sidebars or card groups.
3.  **Level 2 (The Focus):** Use `#f9e8e2` for elements that need to stand out within a surface, such as active input fields or selected list items.
4.  **The Stroke:** Use a very thin (1px) border in a slightly darker version of the surface color to define boundaries without adding visual weight. 

This creates a "flat-tactile" look where elements feel like paper layers stacked on a desk.

## Shapes
A consistent **8px (0.5rem) radius** is applied to all UI elements, including buttons, cards, and input fields. This specific "Rounded" setting strikes a balance between professional structure and approachable softness.

Avoid pill-shaped buttons; the 8px corner maintains the "journal" aesthetic better than fully circular ends, which can feel too "tech-startup." Small elements like checkboxes should follow a smaller 4px radius to maintain visual consistency.

## Components
- **Buttons**: Primary buttons use the Terracotta (#944931) with white Inter text. Secondary buttons use a subtle stroke and no fill. Avoid heavy hover effects; instead, use a slight color shift to the Secondary Accent (#d67d61).
- **Cards**: Cards should not have shadows. Use the `surface_sunken` color for the background and a 1px border. Padding inside cards should be generous (24px - 32px).
- **Input Fields**: Fields should feel like lines in a notebook. Use a solid background fill (#fdf0ec) with a bottom-only border or a very soft 1px all-around border.
- **Chips/Badges**: Use the Secondary Accent (#d67d61) at low opacity (10-15%) with the Primary Terracotta text for a soft, integrated look.
- **Lists**: Use ample vertical spacing between list items. Use thin horizontal dividers in the `surface_accent` color to separate items, ensuring the interface remains "airy."
- **Specialty Components**: Include a "Daily Log" component—a specialized card designed to look like a diary entry, utilizing the Playfair Display font for dates and Inter for the content.
