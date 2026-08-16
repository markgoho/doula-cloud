import { describe, expect, it } from 'vitest';
import { defineLayoutPrimitive } from './defineLayoutPrimitive.js';

function mountFixture(tag: string, attrs: Record<string, string> = {}) {
	const el = document.createElement(tag);
	for (const [key, value] of Object.entries(attrs)) {
		el.setAttribute(key, value);
	}
	document.body.appendChild(el);
	return el;
}

describe('defineLayoutPrimitive', () => {
	it('leaves data-i unset and injects no style for the default config', () => {
		defineLayoutPrimitive(
			'fixture-default-l',
			{ space: '1rem' },
			(v, s) => `${s} { gap: ${v.space}; }`
		);

		const el = mountFixture('fixture-default-l');

		expect(el.hasAttribute('data-i')).toBe(false);
		expect(document.querySelector('[data-layout-primitive-style^="fixture-default-l__"]')).toBeNull();
	});

	it('stamps data-i and injects a scoped, @layer utilities style for a non-default config', () => {
		defineLayoutPrimitive(
			'fixture-override-l',
			{ space: '1rem' },
			(v, s) => `${s} > * + * { gap: ${v.space}; }`
		);

		const el = mountFixture('fixture-override-l', { space: '2rem' });

		expect(el.getAttribute('data-i')).toBe('space:2rem');

		const style = document.querySelector('[data-layout-primitive-style="fixture-override-l__space:2rem"]');
		expect(style).not.toBeNull();
		expect(style?.textContent).toContain('@layer utilities');
		expect(style?.textContent).toContain('fixture-override-l[data-i="space:2rem"] > * + *');
		expect(style?.textContent).toContain('gap: 2rem;');
	});

	it('dedups identical resolved configs across instances into one injected style', () => {
		defineLayoutPrimitive(
			'fixture-dedup-l',
			{ space: '1rem' },
			(v, s) => `${s} { gap: ${v.space}; }`
		);

		mountFixture('fixture-dedup-l', { space: '3rem' });
		mountFixture('fixture-dedup-l', { space: '3rem' });

		const styles = document.querySelectorAll('[data-layout-primitive-style="fixture-dedup-l__space:3rem"]');
		expect(styles).toHaveLength(1);
	});

	it('resolves multi-prop configs by combining set and default values', () => {
		defineLayoutPrimitive(
			'fixture-multi-l',
			{ space: '1rem', limit: '4' },
			(v, s) => `${s} { gap: ${v.space}; --limit: ${v.limit}; }`
		);

		const el = mountFixture('fixture-multi-l', { space: '2rem' });

		expect(el.getAttribute('data-i')).toBe('space:2rem;limit:4');
	});

	it('reverts to the default (no data-i, no new style) when a non-default attribute is removed', () => {
		defineLayoutPrimitive(
			'fixture-revert-l',
			{ space: '1rem' },
			(v, s) => `${s} { gap: ${v.space}; }`
		);

		const el = mountFixture('fixture-revert-l', { space: '4rem' });
		expect(el.hasAttribute('data-i')).toBe(true);

		el.removeAttribute('space');
		expect(el.hasAttribute('data-i')).toBe(false);
	});
});
