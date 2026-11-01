// WCAG 2.2 AA, which is the bar GDS holds its own services to, and this
// repo has already adopted the GOV.UK Design System as its reference for
// service patterns (ADR-0021) -- so the level is pre-argued rather than
// picked here. axe's `best-practice` tag is deliberately off: those are
// opinions, not conformance failures, and a gate that blocks on an
// opinion is a gate people learn to route around.
//
// In a module of its own since #1526, so a spec that scans a route inside
// its own walk reads the same bar accessibility.e2e.ts does.
export const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];
