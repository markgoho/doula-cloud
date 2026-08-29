import { existsSync, readdirSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const componentsRoot = new URL('../../lib/components/', import.meta.url);
const tiers = ['atoms', 'molecules', 'organisms', 'templates'] as const;

function toKebabCase(name: string): string {
	return name.replaceAll(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
}

describe('style guide retrofit coverage', () => {
	for (const tier of tiers) {
		const tierDirectory = new URL(`${tier}/`, componentsRoot);
		if (!existsSync(tierDirectory)) continue;

		const componentFiles = readdirSync(tierDirectory).filter((file) => file.endsWith('.svelte'));

		for (const file of componentFiles) {
			const componentName = file.replace(/\.svelte$/, '');
			const slug = toKebabCase(componentName);

			it(`has a /style-guide page for the ${tier} component "${componentName}"`, () => {
				const pagePath = new URL(`${slug}/+page.svelte`, import.meta.url);
				expect(existsSync(pagePath)).toBe(true);
			});
		}
	}
});
