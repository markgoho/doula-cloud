import { describe, expect, it, beforeAll } from 'vitest';
import { primitiveSpecs, registerDataPrimitives } from './primitives.js';

beforeAll(() => {
	registerDataPrimitives();
});

describe.each(primitiveSpecs)('$tagName', (spec) => {
	it('has no data-i and injects no override style at its default config', () => {
		const el = document.createElement(spec.tagName);
		document.body.appendChild(el);

		expect(el.hasAttribute('data-i')).toBe(false);
		expect(document.querySelector(`[data-layout-primitive-style^="${spec.tagName}__"]`)).toBeNull();
	});

	it('stamps data-i and injects the spec-computed override style for a non-default config', () => {
		const el = document.createElement(spec.tagName);
		const overrides: Record<string, string> = {};
		for (const key of Object.keys(spec.defaults)) {
			overrides[key] = `${spec.defaults[key]}-test`;
			el.setAttribute(key, overrides[key]);
		}
		document.body.appendChild(el);

		const id = Object.entries(overrides)
			.map(([key, value]) => `${key}:${value}`)
			.join(';');
		expect(el.getAttribute('data-i')).toBe(id);

		const style = document.querySelector(`[data-layout-primitive-style="${spec.tagName}__${id}"]`);
		expect(style).not.toBeNull();
		expect(style?.textContent).toContain('@layer utilities');
		expect(style?.textContent).toContain(spec.css(overrides, `${spec.tagName}[data-i="${id}"]`));
	});
});
