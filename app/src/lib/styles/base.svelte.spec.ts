import { describe, expect, it } from 'vitest';
import './app.css';

describe('base.css', () => {
	// Filtering a long list down to a short one removes the scrollbar, and
	// without a reserved track the page shifts sideways by its width.
	it('keeps the scrollbar track on the root whether or not a page scrolls', () => {
		expect(getComputedStyle(document.documentElement).scrollbarGutter).toBe('stable');
	});
});
