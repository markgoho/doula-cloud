import { describe, expect, it } from 'vitest';
import { iconManifest } from './manifest.ts';

const generatedModules = import.meta.glob<{ duotone: string; light: string }>('./generated/*.ts', {
	eager: true
});

describe('iconManifest', () => {
	it('is a non-empty list of unique icon names', () => {
		expect(iconManifest.length).toBeGreaterThan(0);
		expect(new Set(iconManifest).size).toBe(iconManifest.length);
	});

	it.each(iconManifest)('has a generated module with duotone and light markup for "%s"', (name) => {
		const icon = generatedModules[`./generated/${name}.ts`];
		expect(icon?.duotone).toContain('<path');
		expect(icon?.light).toContain('<path');
	});
});
