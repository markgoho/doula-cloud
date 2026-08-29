interface ComponentPage {
	name: string;
	slug: string;
}

/*
 * PascalCase-to-words can't tell a hyphenated compound ("signed-out") from
 * two plain words, so that one case stays a human call; every other name is
 * safe to derive.
 */
const displayNameOverrides: Readonly<Record<string, string>> = {
	SignedOutTopBar: 'Signed-out top bar'
};

export function toDisplayName(componentName: string): string {
	const override = displayNameOverrides[componentName];
	if (override) return override;
	const spaced = componentName.replaceAll(/([a-z0-9])([A-Z])/g, '$1 $2');
	return spaced.charAt(0) + spaced.slice(1).toLowerCase();
}

export function toSlug(componentName: string): string {
	return componentName.replaceAll(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
}

function tierPages(modules: Record<string, unknown>): ComponentPage[] {
	return Object.keys(modules)
		.map((path) => path.split('/').pop()?.replace(/\.svelte$/, '') ?? '')
		.toSorted((a, b) => a.localeCompare(b))
		.map((componentName) => ({ name: toDisplayName(componentName), slug: toSlug(componentName) }));
}

/*
 * Each array is read straight off the component files under src/lib/components
 * -- a new component appears here with no registry entry to remember to add,
 * and a deleted one drops out the same way.
 */
export const atomPages = tierPages(import.meta.glob('../../lib/components/atoms/*.svelte'));
export const moleculePages = tierPages(import.meta.glob('../../lib/components/molecules/*.svelte'));
export const organismPages = tierPages(import.meta.glob('../../lib/components/organisms/*.svelte'));

/*
 * Templates own their own gutters and max-width (ADR-0018), so these pages
 * render outside the style-guide's own padded wrapper -- see +layout.svelte.
 */
export const templatePages = tierPages(import.meta.glob('../../lib/components/templates/*.svelte'));

export const templateSlugs: readonly string[] = templatePages.map((templatePage) => templatePage.slug);
