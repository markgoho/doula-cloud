---
name: Quiet Operator
colors:
  surface: '#f8faf6'
  surface-dim: '#d8dbd7'
  surface-bright: '#f8faf6'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#f2f4f1'
  surface-container: '#eceeeb'
  surface-container-high: '#e7e9e5'
  surface-container-highest: '#e1e3e0'
  on-surface: '#191c1b'
  on-surface-variant: '#404944'
  inverse-surface: '#2e312f'
  inverse-on-surface: '#eff1ee'
  outline: '#707974'
  outline-variant: '#bfc9c3'
  surface-tint: '#2b6954'
  primary: '#004735'
  on-primary: '#ffffff'
  primary-container: '#1f5f4b'
  on-primary-container: '#97d7bd'
  inverse-primary: '#94d3ba'
  secondary: '#4c6359'
  on-secondary: '#ffffff'
  secondary-container: '#cce6d9'
  on-secondary-container: '#50685e'
  tertiary: '#632e29'
  on-tertiary: '#ffffff'
  tertiary-container: '#7f443e'
  on-tertiary-container: '#ffb8b0'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#b0f0d6'
  primary-fixed-dim: '#94d3ba'
  on-primary-fixed: '#002117'
  on-primary-fixed-variant: '#0a513d'
  secondary-fixed: '#cfe8dc'
  secondary-fixed-dim: '#b3ccc0'
  on-secondary-fixed: '#091f18'
  on-secondary-fixed-variant: '#354b42'
  tertiary-fixed: '#ffdad6'
  tertiary-fixed-dim: '#ffb4ab'
  on-tertiary-fixed: '#380c0a'
  on-tertiary-fixed-variant: '#6e3731'
  background: '#f8faf6'
  on-background: '#191c1b'
  surface-variant: '#e1e3e0'
typography:
  display-sm:
    fontFamily: Inter
    fontSize: 24px
    fontWeight: '600'
    lineHeight: 32px
    letterSpacing: -0.02em
  headline-sm:
    fontFamily: Inter
    fontSize: 18px
    fontWeight: '600'
    lineHeight: 24px
    letterSpacing: -0.01em
  body-md:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
    letterSpacing: 0em
  body-sm:
    fontFamily: Inter
    fontSize: 13px
    fontWeight: '400'
    lineHeight: 18px
    letterSpacing: 0em
  label-md:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: '500'
    lineHeight: 16px
    letterSpacing: 0.02em
  data-tabular:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  unit: 4px
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
  gutter: 16px
  margin: 24px
---

## Brand & Style
The design system is built for "Quiet Operator," a practice-management aesthetic that prioritizes utility, calm, and administrative efficiency. The target audience—doulas and birth workers—requires a tool that recedes into the background, providing a stable and professional environment for sensitive data management.

The style is **Professional Minimalism** with a focus on structural clarity. It avoids emotional artifice, relying instead on high information density and a "tool-like" precision. There are no shadows, no gradients, and no unnecessary decorations. Visual hierarchy is established through meticulous alignment and subtle tonal shifts in the neutral palette.

## Colors
The palette is intentionally restrained to reduce cognitive load. 

- **Ground:** A cool, near-white slate (#F8FAFC) serves as the primary background color for the application workspace.
- **Primary:** Deep Pine Green (#1F5F4B) is reserved strictly for primary call-to-action buttons, active navigation states, and critical toggle indicators.
- **Neutrals:** A range of slates (from #F1F5F9 for secondary surfaces to #0F172A for text) handles all structural elements, iconography, and secondary metadata.
- **Status:** Functional colors (red for errors, amber for warnings) should be desaturated to match the professional tone, used only in small icons or subtle text labels.

## Typography
The system utilizes **Inter** for its neutral, grotesque character and exceptional legibility at small sizes. 

- **Base Size:** 14px is the standard for all body and input text.
- **Tabular Figures:** For all numerical data, timestamps, and pricing, the `tnum` (tabular figures) OpenType feature must be enabled to ensure vertical alignment in tables.
- **Density:** Line heights are kept tight (approx 1.4x) to facilitate high information density in list views and data grids.
- **Case:** Labels and metadata should occasionally use all-caps with slight letter spacing to differentiate from editable content.

## Layout & Spacing
This design system employs a **Compact Fluid Grid**. The layout is designed to maximize "above the fold" information without feeling cluttered.

- **Grid:** A 12-column grid is used for major page sections, but the internal density of components is governed by a 4px baseline shift.
- **Density:** Padding in data rows and table cells is kept at a strict 8px (vertical) and 12px (horizontal).
- **Reflow:** On desktop, sidebars are fixed at 240px. On tablets, sidebars collapse to icons only. On mobile, the layout stacks vertically with horizontal margins reduced to 16px.
- **Grouping:** Use logical grouping through whitespace (16px or 24px) rather than containment boxes where possible.

## Elevation & Depth
In keeping with the "Quiet Operator" aesthetic, this system **uses zero shadows**. 

- **Flat Hierarchy:** Depth is communicated through color-blocking and hairline dividers (1px solid #E2E8F0).
- **Surface Tiers:** Backgrounds are #F8FAFC. Active work areas or "cards" are white (#FFFFFF) with a hairline border.
- **Selection:** Active items in a list or sidebar are indicated by a 2px vertical border-left in the Primary Pine Green, rather than a drop shadow or heavy lift.
- **Modals:** Overlays use a semi-transparent slate backdrop (60% opacity) to focus attention, but the modal itself remains flat with a crisp 1px border.

## Shapes
The shape language is "nearly square." 

- **Standard Radius:** All buttons, input fields, and containers use a **4px (0.25rem)** corner radius. 
- **Context:** This slight softening prevents the UI from feeling aggressive while maintaining a precise, architectural appearance.
- **Exceptions:** Status "pills" or tags may use a fully rounded radius to differentiate them from interactive buttons.

## Components
- **Buttons:** Primary buttons are Solid Pine Green with white text. Secondary buttons are White with a Slate border and Slate text. No hover "lift"; hover states should simply darken the background color by 5-10%.
- **Inputs:** Text fields are white with a 1px Slate-200 border. Focus states use a 1px Pine Green border. No glow or outer shadows on focus.
- **Data Tables:** These are the core of the app. Rows have a 1px bottom border. Header cells use `label-md` styling (uppercase, bold). Use alternating row stripes (Slate-50) only for very wide data sets.
- **Chips/Tags:** Small, rectangular (4px radius). Use Slate-100 backgrounds with Slate-700 text for neutral status. 
- **Lists:** High-density vertical stacks. Iconography is monochromatic (Slate-500). Use horizontal hairline dividers to separate items.
- **Checkboxes:** Square with a 2px radius. When checked, the fill is Pine Green with a white checkmark.
