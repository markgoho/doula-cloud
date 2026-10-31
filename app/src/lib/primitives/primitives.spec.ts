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

describe('sidebar-l', () => {
	it('gives the fixed basis to the last child, and the content minimum to the first, at side end', () => {
		const sidebar = primitiveSpecs.find((spec) => spec.tagName === 'sidebar-l')!;

		const css = sidebar.css({ ...sidebar.defaults, side: 'end' }, 'sidebar-l');

		expect(css).toContain(`sidebar-l > :last-child { flex-basis: ${sidebar.defaults.basis};`);
		expect(css).toContain(`sidebar-l > :first-child { flex-basis: 0; flex-grow: 999; min-inline-size: ${sidebar.defaults['content-min']}; }`);
	});
});
