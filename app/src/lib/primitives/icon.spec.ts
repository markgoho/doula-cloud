import { describe, expect, it, beforeAll } from 'vitest';
import { defineIcon } from './icon.js';

beforeAll(() => {
	defineIcon();
});

describe('icon-l', () => {
	it('has no data-i and no role/aria-label by default', () => {
		const element = document.createElement('icon-l');
		document.body.append(element);

		expect(Object.hasOwn(element.dataset, 'i')).toBe(false);
		expect(element.hasAttribute('role')).toBe(false);
		expect(element.hasAttribute('aria-label')).toBe(false);
	});

	it('injects a margin-inline-end override style when space is set', () => {
		const element = document.createElement('icon-l');
		element.setAttribute('space', '1rem');
		document.body.append(element);

		expect(element.dataset.i).toBe('space:1rem');
		const style = document.querySelector('[data-layout-primitive-style="icon-l__space:1rem"]');
		expect(style?.textContent).toContain('margin-inline-end: 1rem;');
	});

	it('reflects a non-empty label attribute to role=img and aria-label', () => {
		const element = document.createElement('icon-l');
		element.setAttribute('label', 'Close');
		document.body.append(element);

		expect(element.getAttribute('role')).toBe('img');
		expect(element.getAttribute('aria-label')).toBe('Close');
	});

	it('removes role/aria-label when label is removed', () => {
		const element = document.createElement('icon-l');
		element.setAttribute('label', 'Close');
		document.body.append(element);

		element.removeAttribute('label');

		expect(element.hasAttribute('role')).toBe(false);
		expect(element.hasAttribute('aria-label')).toBe(false);
	});
});
