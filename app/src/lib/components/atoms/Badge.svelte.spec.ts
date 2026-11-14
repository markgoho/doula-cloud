import '#lib/styles/app.css';
import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { contrast, expectDistinctTintHues, pixelOf, themes, withTheme } from '#testing/variantTints.js';
import Badge from './Badge.svelte';
import Icon from './Icon.svelte';

type SetupOptions = Partial<ComponentProps<typeof Badge>>;

async function setup({ label = 'Active', variant = 'success', ...rest }: SetupOptions = {}) {
	const { container } = await render(Badge, { label, variant, ...rest });
	return { container };
}

const variants = ['info', 'success', 'warning', 'error', 'neutral'] as const;

describe('Badge.svelte', () => {
	it('renders the label as visible text', async () => {
		await setup({ label: 'Active' });

		await expect.element(page.getByText('Active')).toBeVisible();
	});

	it.each(variants)('applies a variant-matching class for the %s variant', async (variant) => {
		const { container } = await setup({ variant });

		expect(container.querySelector(`span.${variant}`)).toBeInTheDocument();
	});

	it.each(variants)('renders a decorative icon for the %s variant', async (variant) => {
		const { container } = await setup({ variant });

		const svg = container.querySelector('svg');
		expect(svg).toBeInTheDocument();
		expect(svg).toHaveAttribute('aria-hidden', 'true');
	});

	it('draws the icon it is given in place of its variant icon', async () => {
		const plain = await setup({ variant: 'neutral' });
		const owed = await setup({ variant: 'neutral', icon: 'receipt' });

		expect(owed.container.querySelector('svg')?.getHTML()).not.toBe(plain.container.querySelector('svg')?.getHTML());
	});
});

/*
 * #434 -- the background tint has to carry variant signal on its own, not
 * just the border/icon/text. See tokens.spec.ts for why color-mix(in
 * oklch, ...) failed this: a 12% mix's hue lands on the shorter polar arc
 * toward --color-surface, dragging every variant to nearly the same hue.
 * The regression only shows in the rendered pixel, so a canvas samples it.
 */
describe('the five variants read as visibly different background tints (#434)', () => {
	it.each(themes)('every pair of variants clears a 15-degree hue gap in the %s theme', async (theme) => {
		await expectDistinctTintHues(theme, variants, async (variant) => {
			const { container } = await setup({ variant });
			return getComputedStyle(container.querySelector(`span.${variant}`)!).backgroundColor;
		});
	});
});

/*
 * The contrast floors tokens.spec.ts enforces on tokens.css in isolation --
 * WCAG SC 1.4.3 at 4.5:1 for text, SC 1.4.11 at 3:1 for a UI-component
 * boundary -- checked here against the actual mixed background each
 * variant renders, not the underlying token values.
 */
describe('text keeps its contrast floor on the tinted background, with no outline (#434)', () => {
	it.each(themes)('holds for every variant in the %s theme', async (theme) => {
		await withTheme(theme, async () => {
			for (const variant of variants) {
				const { container } = await setup({ variant });
				const style = getComputedStyle(container.querySelector(`span.${variant}`)!);
				const background = pixelOf(style.backgroundColor);
				const text = pixelOf(style.color);

				expect(contrast(text, background), `${variant} text on its tint`).toBeGreaterThanOrEqual(4.5);
				// A quiet badge draws no outline: the text and the icon carry the meaning.
				expect(style.borderTopColor, `${variant} outline`).toBe('rgba(0, 0, 0, 0)');
			}
		});
	});
});

/*
 * #2843 -- the red badge once led with the "x" a removable filter tag leads
 * with, so one glyph meant "late" and "remove this" a few lines apart.
 */
async function drawnIcon(variant: (typeof variants)[number]) {
	const { container } = await setup({ variant });
	return container.querySelector('svg')!.getHTML();
}

describe('the icon each variant draws (#2843)', () => {
	it('is a different one for every variant, so no two meanings share a glyph', async () => {
		const drawn = await Promise.all(variants.map((variant) => drawnIcon(variant)));

		expect(new Set(drawn).size).toBe(variants.length);
	});

	it("is not the remove-filter tag's x on the error variant", async () => {
		const { container: tag } = await render(Icon, { name: 'x', size: 16 });

		expect(await drawnIcon('error')).not.toBe(tag.querySelector('svg')!.getHTML());
	});
});
