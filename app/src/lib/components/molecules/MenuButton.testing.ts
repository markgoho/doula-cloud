import type { Locator } from 'vitest/browser';

/**
Where `MenuButton`'s panel lands, for a spec that asserts it opens below a
top bar however tall the bar grew (#1573).

A sibling file for the reason `LabeledField.testing.ts` gives: nothing
outside a spec imports it, so the production bundle never meets it.
*/

/**
The panel holding `inside`, found from something in it an accessible query
reached. The panel itself is a plain popover container with no role of its
own, so there is nothing to ask for it by name; what a caller asserts is its
geometry, which the accessible tree does not carry.
*/
export function panelHolding(inside: Locator): HTMLElement {
	return inside.element().closest<HTMLElement>('[popover]')!;
}

/**
Makes the panel take `MenuButton`'s fallback path in an engine that has
anchor positioning, so the path an engine without it takes can be measured
here at all. `@supports` cannot be switched off, so this undoes what that
block sets -- the anchored position and the margin -- and restores the
fallback's own pin, which that block's `inset: auto` overrode. It restates
that pin, so a change to the fallback rule in `MenuButton.svelte` has to be
made here too. Remove it when done.
*/
export function withoutAnchorPositioning(): HTMLStyleElement {
	const style = document.createElement('style');
	style.textContent = `[popover] {
		position: fixed !important;
		position-area: none !important;
		margin: 0 !important;
		inset-block-start: var(--menu-fallback-top, var(--top-bar-height)) !important;
		inset-inline-end: var(--page-gutter) !important;
	}`;
	document.head.append(style);
	return style;
}

/**
The bottom of a bar's own content, inside its bottom border: the edge the
current nav item's accent rule sits on, and the one a panel opens below.
*/
export function contentBottom(bar: HTMLElement): number {
	const bottomBorder = bar.offsetHeight - bar.clientHeight - bar.clientTop;
	return bar.getBoundingClientRect().bottom - bottomBorder;
}
