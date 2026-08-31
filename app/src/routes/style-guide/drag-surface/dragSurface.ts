import type { Component } from 'svelte';

/*
 * The demo half of the drag surface (CONTEXT.md): the list of components a
 * reader can put inside the frame.
 *
 * Every component already has a style-guide page, and that page is an
 * ordinary Svelte component -- so the surface renders the page itself
 * rather than carrying a second copy of every demo's props. A fixture
 * improved for one is improved for both, which is what keeps the drag
 * surface and the continuum check one artifact rather than two.
 */

export interface Demo {
	name: string;
	slug: string;
	component: Component;
}

export interface PageModule {
	default: Component;
}

/**
`../description-list/+page.svelte` -> `description-list`.
*/
export function toSlug(modulePath: string): string {
	return modulePath.split('/').at(-2) ?? '';
}

export function toDemos(
	modules: Record<string, PageModule>,
	pages: readonly { name: string; slug: string }[]
): Demo[] {
	const componentBySlug = new Map(
		Object.entries(modules).map(([modulePath, module]) => [toSlug(modulePath), module.default])
	);
	return pages.flatMap((page) => {
		const component = componentBySlug.get(page.slug);
		return component ? [{ name: page.name, slug: page.slug, component }] : [];
	});
}
