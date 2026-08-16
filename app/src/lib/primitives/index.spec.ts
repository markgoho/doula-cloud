import { describe, expect, it } from 'vitest';
import { registerLayoutPrimitives } from './index.js';

describe('registerLayoutPrimitives', () => {
	it('registers both the data-driven primitives and Icon', () => {
		registerLayoutPrimitives();

		expect(customElements.get('stack-l')).toBeDefined();
		expect(customElements.get('container-l')).toBeDefined();
		expect(customElements.get('icon-l')).toBeDefined();
	});
});
