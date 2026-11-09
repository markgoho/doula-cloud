import '#lib/styles/app.css';
import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { contrast, expectDistinctTintHues, pixelOf, themes, withTheme } from '#testing/variantTints.js';
import Notice from './Notice.svelte';

type SetupOptions = Partial<ComponentProps<typeof Notice>>;

async function setup({ message = 'Something happened', variant = 'info', ...rest }: SetupOptions = {}) {
	const { container } = await render(Notice, { message, variant, ...rest });
	return { container };
}

const variants = ['error', 'status', 'info'] as const;

describe('Notice.svelte', () => {
	it('renders the message as visible text', async () => {
		await setup({ message: 'Missing invite token' });

		await expect.element(page.getByText('Missing invite token')).toBeVisible();
	});

	it.each([
		['error', 'alert'],
		['status', 'status'],
		['info', 'status']
	] as const)('gives the %s variant role="%s"', async (variant, role) => {
		const { container } = await setup({ variant });

		expect(container.querySelector(`[role="${CSS.escape(role)}"]`)).toBeInTheDocument();
	});

	it('applies a variant-matching class', async () => {
		const { container } = await setup({ variant: 'status' });

		expect(container.querySelector('.status')).toBeInTheDocument();
	});

	// For a Notice that reports the result of a control which is gone once
	// pressed (#1598's "Put me on this work", the Feedback drawer):
	// focus lands on the result and not on the document.
	it('takes focus when it appears, where it is asked to', async () => {
		await setup({ message: 'You are on this work.', variant: 'status', isFocusedOnAppear: true });

		await expect.element(page.getByRole('status')).toHaveFocus();
	});

	it('takes no focus and adds no stop to the tab order unless it is asked to', async () => {
		const { container } = await setup({ variant: 'status' });

		expect(document.activeElement).toBe(document.body);
		expect(container.querySelector('[tabindex]')).toBeNull();
	});

	it.each(variants)('renders a decorative icon for the %s variant', async (variant) => {
		const { container } = await setup({ variant });

		const svg = container.querySelector('svg');
		expect(svg).toBeInTheDocument();
		expect(svg).toHaveAttribute('aria-hidden', 'true');
	});
});

/*
 * #434 -- same defect as Badge.svelte: color-mix(in oklch, ...) dragged
 * every variant's tint toward --color-surface's own hue on the shorter
 * polar arc. See Badge.svelte.spec.ts for the fuller comment.
 */
describe('the three variants read as visibly different background tints (#434)', () => {
	it.each(themes)('every pair of variants clears a 15-degree hue gap in the %s theme', async (theme) => {
		await expectDistinctTintHues(theme, variants, async (variant) => {
			const { container } = await setup({ variant });
			return getComputedStyle(container.querySelector(`p.${variant}`)!).backgroundColor;
		});
	});
});

/*
 * The contrast floors tokens.spec.ts enforces on tokens.css in isolation --
 * WCAG SC 1.4.3 at 4.5:1 for text, SC 1.4.11 at 3:1 for a UI-component
 * boundary -- checked here against the actual mixed background each
 * variant renders, not the underlying token values.
 */
describe('text and border keep their contrast floor on the tinted background (#434)', () => {
	it.each(themes)('holds for every variant in the %s theme', async (theme) => {
		await withTheme(theme, async () => {
			for (const variant of variants) {
				const { container } = await setup({ variant });
				const style = getComputedStyle(container.querySelector(`p.${variant}`)!);
				const background = pixelOf(style.backgroundColor);
				const text = pixelOf(style.color);
				const border = pixelOf(style.borderColor);

				expect(contrast(text, background), `${variant} text on its tint`).toBeGreaterThanOrEqual(4.5);
				expect(contrast(border, background), `${variant} border on its tint`).toBeGreaterThanOrEqual(3);
			}
		});
	});
});
