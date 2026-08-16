import { describe, expect, it, beforeAll } from 'vitest';
import { defineIcon } from './icon.js';

beforeAll(() => {
	defineIcon();
});

describe('icon-l', () => {
	it('has no data-i and no role/aria-label by default', () => {
		const el = document.createElement('icon-l');
		document.body.appendChild(el);

		expect(el.hasAttribute('data-i')).toBe(false);
		expect(el.hasAttribute('role')).toBe(false);
		expect(el.hasAttribute('aria-label')).toBe(false);
	});

	it('injects a margin-inline-end override style when space is set', () => {
		const el = document.createElement('icon-l');
		el.setAttribute('space', '1rem');
		document.body.appendChild(el);

		expect(el.getAttribute('data-i')).toBe('space:1rem');
		const style = document.querySelector('[data-layout-primitive-style="icon-l__space:1rem"]');
		expect(style?.textContent).toContain('margin-inline-end: 1rem;');
	});

	it('reflects a non-empty label attribute to role=img and aria-label', () => {
		const el = document.createElement('icon-l');
		el.setAttribute('label', 'Close');
		document.body.appendChild(el);

		expect(el.getAttribute('role')).toBe('img');
		expect(el.getAttribute('aria-label')).toBe('Close');
	});

	it('removes role/aria-label when label is removed', () => {
		const el = document.createElement('icon-l');
		el.setAttribute('label', 'Close');
		document.body.appendChild(el);

		el.removeAttribute('label');

		expect(el.hasAttribute('role')).toBe(false);
		expect(el.hasAttribute('aria-label')).toBe(false);
	});
});
