// WCAG 2.2 AA, which is the bar GDS holds its own services to, and this
// repo has already adopted the GOV.UK Design System as its reference for
// service patterns (ADR-0021) -- so the level is pre-argued rather than
// picked here. axe's `best-practice` tag is deliberately off: those are
// opinions, not conformance failures, and a gate that blocks on an
// opinion is a gate people learn to route around.
//
// In a module of its own since #1526: accessibility.e2e.ts is the gate,
// and founder-feedback.e2e.ts scans the two founder routes inside its own
// walk, because only one spec file at a time may hold the founder (see
// stack.ts's seedFounder). Both read the bar from here, so it is one bar.
export const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];
