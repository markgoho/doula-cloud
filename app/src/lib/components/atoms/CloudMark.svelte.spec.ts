import '#lib/styles/app.css';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import CloudMark from './CloudMark.svelte';

interface SetupOptions {
	size?: 'sm' | 'md' | 'lg' | 'xl';
	label?: string;
}

async function setup({ size, label }: SetupOptions = {}) {
	const { container } = await render(CloudMark, { size, label });
	/*
	 * querySelector, case 2 of svelte-tests.md: the strokes are decorative
	 * paths with no role and no name, so no accessible query can find them.
	 * They are found in document order, which is paint order: the inner
	 * arch, then the outer line over it.
	 */
	const [arch, outline] = container.querySelectorAll('path');
	return { svg: container.querySelector('svg')!, paths: container.querySelectorAll('path'), arch, outline };
}

// The real, browser-resolved sRGB of a CSS color, whatever syntax
// getComputedStyle hands back (the same canvas read Badge's spec uses).
function pixelOf(cssColor: string): string {
	const canvas = document.createElement('canvas');
	canvas.width = 1;
	canvas.height = 1;
	const context = canvas.getContext('2d')!;
	context.fillStyle = cssColor;
	context.fillRect(0, 0, 1, 1);
	return context.getImageData(0, 0, 1, 1).data.slice(0, 3).join(' ');
}

function strokeOf(path: Element): string {
	return pixelOf(getComputedStyle(path).stroke);
}

function tokenColor(name: string): string {
	return pixelOf(getComputedStyle(document.documentElement).getPropertyValue(name));
}

async function withTheme(theme: 'light' | 'dark', run: () => Promise<void>): Promise<void> {
	document.documentElement.dataset.theme = theme;
	try {
		await run();
	} finally {
		delete document.documentElement.dataset.theme;
	}
}

const themes = ['light', 'dark'] as const;

describe('CloudMark', () => {
	it.each([
		['sm', '40', '19'],
		['md', '60', '28'],
		['lg', '120', '56'],
		['xl', '200', '93']
	] as const)('draws %s at %sx%s', async (size, width, height) => {
		const { svg } = await setup({ size });

		expect(svg.getAttribute('width')).toBe(width);
		expect(svg.getAttribute('height')).toBe(height);
	});

	// The landing's panel caps the xl mark at about 55% of its width
	// (#1645), so in a narrow panel the mark is drawn smaller than 200px:
	// its height must follow, or the strokes sit letterboxed in a 93px box.
	it('keeps its proportion at xl when the space it is given is narrower than it', async () => {
		const { svg } = await setup({ size: 'xl' });
		svg.parentElement!.style.inlineSize = '100px';

		const { width, height } = svg.getBoundingClientRect();
		expect(width).toBe(100);
		expect(height).toBeCloseTo((100 * 94) / 202, 0);
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

	/*
	 * The rules of the mark, from docs/marketing/brand.md (#1486): two
	 * strokes and only two, with the geometry of the reference drawings in
	 * docs/marketing/brand/.
	 */
	it('draws two strokes, the inner arch and the outer line', async () => {
		const { paths } = await setup();

		expect([...paths].map((path) => path.getAttribute('d'))).toEqual([
			'M100 160a32 32 0 0 1 64 0',
			'M56 160a76 76 0 0 1 137.97-44 44 44 0 0 1 44.03 44'
		]);
	});

	describe.each(themes)('in the %s theme', (theme) => {
		// 28px tall or more: the outer line in --color-primary and the inner
		// arch in --color-primary-hover.
		it.each(['md', 'lg', 'xl'] as const)('draws %s in two tones', async (size) => {
			await withTheme(theme, async () => {
				const { arch, outline } = await setup({ size });

				expect(strokeOf(outline)).toBe(tokenColor('--color-primary'));
				expect(strokeOf(arch)).toBe(tokenColor('--color-primary-hover'));
				expect(strokeOf(arch)).not.toBe(strokeOf(outline));
			});
		});

		// Below 28px the two tones are too near to each other to read as two.
		it('draws sm in one color', async () => {
			await withTheme(theme, async () => {
				const { arch, outline } = await setup({ size: 'sm' });

				expect(strokeOf(outline)).toBe(tokenColor('--color-primary'));
				expect(strokeOf(arch)).toBe(tokenColor('--color-primary'));
			});
		});
	});

	it('takes the tokens of the theme it is in', async () => {
		const strokes: string[] = [];
		for (const theme of themes) {
			await withTheme(theme, async () => {
				const { outline } = await setup();
				strokes.push(strokeOf(outline));
			});
		}

		expect(strokes[0]).not.toBe(strokes[1]);
	});

	it('is decorative unless it is given a name', async () => {
		const { svg } = await setup();

		expect(svg.getAttribute('aria-hidden')).toBe('true');
		expect(svg.getAttribute('role')).toBeNull();
	});

	it('is an image when it stands alone', async () => {
		await setup({ label: 'DoulaCloud' });

		await expect.element(page.getByRole('img', { name: 'DoulaCloud' })).toBeVisible();
	});
});
