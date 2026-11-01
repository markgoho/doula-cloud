import { describe, expect, it, beforeAll } from 'vitest';
import { primitiveSpecs, registerDataPrimitives } from './primitives.js';

beforeAll(() => {
	registerDataPrimitives();
});

describe.each(primitiveSpecs)('$tagName', (spec) => {
	it('has no data-i and injects no override style at its default config', () => {
		const element = document.createElement(spec.tagName);
		document.body.append(element);

		expect(Object.hasOwn(element.dataset, 'i')).toBe(false);
		expect(document.querySelector(`[data-layout-primitive-style^="${CSS.escape(spec.tagName)}__"]`)).toBeNull();
	});

	it('stamps data-i and injects the spec-computed override style for a non-default config', () => {
		const element = document.createElement(spec.tagName);
		const overrides: Record<string, string> = {};
		for (const [key, value] of Object.entries(spec.defaults)) {
			overrides[key] = `${value}-test`;
			element.setAttribute(key, overrides[key]);
		}
		document.body.append(element);

		const id = Object.entries(overrides)
			.map(([key, value]) => `${key}:${value}`)
			.join(';');
		expect(element.dataset.i).toBe(id);

		const style = document.querySelector(`[data-layout-primitive-style="${CSS.escape(spec.tagName)}__${CSS.escape(id)}"]`);
		expect(style).not.toBeNull();
		expect(style?.textContent).toContain('@layer utilities');
		expect(style?.textContent).toContain(spec.css(overrides, `${spec.tagName}[data-i="${id}"]`));
	});
});
