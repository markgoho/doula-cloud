import { existsSync, readdirSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
	atomPages,
	moleculePages,
	organismPages,
	templatePages,
	toDisplayName,
	toSlug
} from './components.js';

const componentsRoot = new URL('../../lib/components/', import.meta.url);

const tiers = [
	{ directory: 'atoms', pages: atomPages },
	{ directory: 'molecules', pages: moleculePages },
	{ directory: 'organisms', pages: organismPages },
	{ directory: 'templates', pages: templatePages }
] as const;

// Independent of toSlug, so a bug shared by both the registry and this
// oracle can't hide a drift from itself.
function toKebabCase(name: string): string {
	return name.replaceAll(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
}

function fileSlugsOnDisk(directory: string): string[] {
	const tierDirectory = new URL(`${directory}/`, componentsRoot);
	return readdirSync(tierDirectory)
		.filter((file) => file.endsWith('.svelte'))
		.map((file) => toKebabCase(file.replace(/\.svelte$/, '')))
		.toSorted((a, b) => a.localeCompare(b));
}

describe('style guide component registry', () => {
	for (const tier of tiers) {
		it(`${tier.directory} registry has exactly the components on disk`, () => {
			const registeredSlugs = tier.pages.map((page) => page.slug).toSorted((a, b) => a.localeCompare(b));
			expect(registeredSlugs).toEqual(fileSlugsOnDisk(tier.directory));
		});
	}

	it('never registers the same slug twice within a tier', () => {
		for (const tier of tiers) {
			const slugs = tier.pages.map((page) => page.slug);
			expect(new Set(slugs).size).toBe(slugs.length);
		}
	});
});

function componentsOnDisk(): string[] {
	return tiers.flatMap((tier) =>
		readdirSync(new URL(`${tier.directory}/`, componentsRoot))
			.filter((file) => file.endsWith('.svelte'))
			.map((file) => `${tier.directory}/${file.replace(/\.svelte$/, '')}`)
	);
}

function hasSpec(component: string): boolean {
	return existsSync(new URL(`${component}.svelte.spec.ts`, componentsRoot));
}

describe('a component has a spec beside it', () => {
	it('finds no component without one', () => {
		const missing = componentsOnDisk()
			.filter((component) => !hasSpec(component))
			.map(
				(component) =>
					`app/src/lib/components/${component}.svelte has no spec: add ${component.split('/').pop()}.svelte.spec.ts beside it (docs/design/workflow.md, "What a new component needs")`
			);
		expect(missing, missing.join('\n')).toEqual([]);
	});
});

describe('toDisplayName', () => {
	it.each([
		['CloudMark', 'Cloud mark'],
		['DataTable', 'Data table'],
		['MembershipFields', 'Membership fields'],
		['SignedOutTopBar', 'Signed-out top bar']
	])('renders %s as "%s"', (componentName, expected) => {
		expect(toDisplayName(componentName)).toBe(expected);
	});
});

describe('toSlug', () => {
	it.each([
		['CloudMark', 'cloud-mark'],
		['SignedOutTopBar', 'signed-out-top-bar']
	])('renders %s as "%s"', (componentName, expected) => {
		expect(toSlug(componentName)).toBe(expected);
	});
});
