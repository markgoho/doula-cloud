import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import CloudMark from './CloudMark.svelte';

interface SetupOptions {
	size?: 'sm' | 'md' | 'lg';
	label?: string;
}

async function setup({ size, label }: SetupOptions = {}) {
	const { container } = await render(CloudMark, { size, label });
	return { svg: container.querySelector('svg')! };
}

describe('CloudMark', () => {
	it.each([
		['sm', '40', '19'],
		['md', '60', '28'],
		['lg', '120', '56']
	] as const)('draws %s at %sx%s', async (size, width, height) => {
		const { svg } = await setup({ size });

		expect(svg.getAttribute('width')).toBe(width);
		expect(svg.getAttribute('height')).toBe(height);
	});

	/*
	 * One stroke for every size. An SVG stroke is in viewBox units and
	 * scales with the frame, which is what produces the weight ramp the
	 * canvas had to state by hand -- pen.dev's strokeWidth is node pixels
	 * and does not scale (#411).
	 */
	it('scales by the frame and never by the stroke', async () => {
		const small = await setup({ size: 'sm' });
		const large = await setup({ size: 'lg' });

		expect(small.svg.getAttribute('stroke-width')).toBe('14');
		expect(large.svg.getAttribute('stroke-width')).toBe('14');
	});

	it('is decorative unless it is given a name', async () => {
		const { svg } = await setup();

		expect(svg.getAttribute('aria-hidden')).toBe('true');
		expect(svg.getAttribute('role')).toBeNull();
	});

	it('is an image when it stands alone', async () => {
		await setup({ label: 'Doula Cloud' });

		await expect.element(page.getByRole('img', { name: 'Doula Cloud' })).toBeVisible();
	});
});
