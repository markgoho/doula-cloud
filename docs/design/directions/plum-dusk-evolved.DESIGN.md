---
name: Plum Dusk, evolved
colors:
  surface: '#fff8f9'
  surface-dim: '#e1d8da'
  surface-bright: '#fff8f9'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#fbf1f4'
  surface-container: '#f5ebee'
  surface-container-high: '#efe6e8'
  surface-container-highest: '#e9e0e3'
  on-surface: '#1f1a1c'
  on-surface-variant: '#51434b'
  inverse-surface: '#342f31'
  inverse-on-surface: '#f8eef1'
  outline: '#83737b'
  outline-variant: '#d5c1cb'
  surface-tint: '#8e4479'
  primary: '#722c60'
  on-primary: '#ffffff'
  primary-container: '#8e4479'
  on-primary-container: '#ffc9ea'
  inverse-primary: '#ffade2'
  secondary: '#605e60'
  on-secondary: '#ffffff'
  secondary-container: '#e6e1e4'
  on-secondary-container: '#666366'
  tertiary: '#2c4f00'
  on-tertiary: '#ffffff'
  tertiary-container: '#43681a'
  on-tertiary-container: '#b9e689'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#ffd8ee'
  primary-fixed-dim: '#ffade2'
  on-primary-fixed: '#3b0030'
  on-primary-fixed-variant: '#722c60'
  secondary-fixed: '#e6e1e4'
  secondary-fixed-dim: '#c9c5c8'
  on-secondary-fixed: '#1c1b1d'
  on-secondary-fixed-variant: '#484649'
  tertiary-fixed: '#c3f092'
  tertiary-fixed-dim: '#a8d479'
  on-tertiary-fixed: '#0f2000'
  on-tertiary-fixed-variant: '#2d5001'
  background: '#fff8f9'
  on-background: '#1f1a1c'
  surface-variant: '#e9e0e3'
typography:
  headline-lg:
    fontFamily: Hanken Grotesk
    fontSize: 32px
    fontWeight: '600'
    lineHeight: 40px
    letterSpacing: -0.02em
  headline-lg-mobile:
    fontFamily: Hanken Grotesk
    fontSize: 24px
    fontWeight: '600'
    lineHeight: 32px
    letterSpacing: -0.01em
  headline-md:
    fontFamily: Hanken Grotesk
    fontSize: 20px
    fontWeight: '600'
    lineHeight: 28px
  body-lg:
    fontFamily: Hanken Grotesk
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
  body-md:
    fontFamily: Hanken Grotesk
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  label-sm:
    fontFamily: Hanken Grotesk
    fontSize: 12px
    fontWeight: '600'
    lineHeight: 16px
    letterSpacing: 0.05em
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  unit: 8px
  container-max-width: 1280px
  gutter: 24px
  margin-mobile: 16px
  margin-desktop: 40px
---

## Brand & Style

The design system is built for a focused practice-management environment, prioritizing clarity, calm, and professional reliability. The aesthetic is a refined **Minimalism** with a **Corporate/Modern** backbone, utilizing high-end editorial constraints to avoid visual clutter.

The target audience consists of doulas and birth workers who require a digital workspace that feels supportive rather than demanding. The UI evokes a "quietly confident" emotional response—restrained in its use of color and motion, ensuring that the practitioner’s data remains the focal point. By stripping away shadows and relying on structured, hairline-based containment, the interface achieves a modern, architectural feel that is both accessible and authoritative.

## Colors

This design system utilizes a strictly light-mode palette. The foundation is a warm, neutral off-white background (`#FAF9F8`) that prevents eye strain during long periods of administrative work.

- **Primary Plum (#8E4479):** Reserved strictly for primary call-to-actions, active navigation states, and critical progress indicators. It is never used for large background areas.
- **Plum Tint (#F7F2F5):** Used as a surface tint for active list items, hover states, or subtle background differentiation in sidebars.
- **Neutral Grey/Black (#1A1618):** Used for primary text and high-contrast iconography to ensure WCAG AA accessibility.
- **Hairline Border (#E5E1E0):** A neutral, low-contrast grey used to define the boundaries of the UI without adding visual weight.

## Typography

The design system employs a single, neutral grotesque family to maintain a systematic and utilitarian feel. Hierarchy is established through rigorous application of weight and size rather than typeface variance.

- **Headlines:** Use semi-bold weights with slight negative letter-spacing to create a "grounded" look for section titles.
- **Body:** Standardized at 16px for primary readability, dropping to 14px for dense data tables and secondary information.
- **Labels:** Small caps or uppercase are used sparingly for metadata and table headers to provide a clear distinction from interactive body text.

## Layout & Spacing

The layout follows a **Fixed Grid** philosophy for desktop to maintain a structured "dashboard" feel, while transitioning to a fluid model for mobile.

- **Rhythm:** All spacing is derived from a base-8 increment. Generous internal padding (24px+) is encouraged within cards to reflect the "quietly confident" brand personality.
- **Grid:** A 12-column grid is used for the main workspace. Sidebars are fixed at 280px, while content areas expand to fill the remaining space up to the 1280px max-width.
- **Mobile Adaptation:** On mobile, margins reduce to 16px and multi-column card layouts reflow into a single-stack vertical feed.

## Elevation & Depth

This design system is strictly **Flat**. Depth is achieved exclusively through **Tonal Layering** and **Hairline Borders**.

- **Shadows:** No drop shadows or inner shadows are permitted. 
- **Borders:** All interactive surfaces and containers are defined by 1px solid borders in a neutral grey. 
- **Z-Index:** Layering is communicated by stacking order. Overlays (modals) use a solid, high-contrast border and a dim, neutral-tinted backdrop rather than a shadow cast.
- **Active State:** Elements indicate focus or selection by changing border color to the primary Plum or adding the faint Plum surface tint.

## Shapes

The shape language is consistent and "Soft-Modern." An 8px radius is applied to all primary UI containers including buttons, cards, and input fields.

- **Standard Radius:** 8px (`0.5rem`) for cards, buttons, and inputs.
- **Inner Elements:** Nested elements (like images within cards) should use a slightly smaller radius (4px) to maintain visual harmony.
- **Circular:** Reserved only for user avatars and status pips.

## Components

- **Buttons:** Primary buttons are solid Plum with white text. Secondary buttons are transparent with a 1px Plum border. Tertiary buttons are text-only with no border. All use 8px corners and medium density padding (12px vertical, 24px horizontal).
- **Input Fields:** Clean, white backgrounds with 1px neutral borders. On focus, the border transitions to Plum. Labels are positioned above the field using the `label-sm` style.
- **Cards:** White or off-white background with a 1px border. No shadow. Cards should include a 24px internal gutter for content.
- **Activity Feed:** Minimalist list items separated by 1px horizontal hairlines. Use the Plum tint as a background for "New" or "Unread" items.
- **Navigation:** A flat, vertical sidebar on the left. Active links use a bold weight and a small vertical Plum bar indicator on the left edge.
- **Chips:** Small, 4px rounded capsules used for tagging status (e.g., "Active", "Complete"). Use low-saturation background tints to avoid competing with primary Plum actions.
