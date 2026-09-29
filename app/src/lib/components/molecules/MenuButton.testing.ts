/**
Where `MenuButton`'s panel lands, for a spec that asserts it opens below a
top bar however tall the bar grew (#1573).

A sibling file for the reason `LabeledField.testing.ts` gives: nothing
outside a spec imports it, so the production bundle never meets it.
*/

/**
The one open panel. Read by `:popover-open` rather than by an accessible
query because what a caller asserts is its geometry, which the accessible
tree does not carry, and a panel has no role of its own to be asked by.
*/
export function openPanel(): HTMLElement {
	return document.querySelector<HTMLElement>(':popover-open')!;
}

/**
Makes the panel take `MenuButton`'s fallback path in an engine that has
anchor positioning, so the path an engine without it takes can be measured
here at all. `@supports` cannot be switched off, so this undoes what that
block sets -- the anchored position and the margin -- and restores the
fallback's own pin, which that block's `inset: auto` overrode. Remove it
when done.
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
