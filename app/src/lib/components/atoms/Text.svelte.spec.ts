import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Text from './Text.svelte';

type SetupOptions = Partial<ComponentProps<typeof Text>>;

async function setup({ text = 'Body copy', ...rest }: SetupOptions = {}) {
	const { container } = await render(Text, { text, ...rest });
	return { container };
}

describe('Text.svelte', () => {
	it('renders the text as visible content', async () => {
		await setup({ text: 'Body copy' });

		await expect.element(page.getByText('Body copy')).toBeVisible();
	});

	it('reads at the body step in the default tone when nothing is asked for', async () => {
		await setup();

		await expect.element(page.getByText('Body copy')).toHaveClass(/step-body\b/);
		await expect.element(page.getByText('Body copy')).toHaveClass(/tone-default/);
	});

	it.each(['body', 'body-sm', 'label', 'meta'] as const)(
		'applies the %s type step',
		async (step) => {
			const { container } = await setup({ step });

			expect(container.querySelector(`p.step-${step}`)).toBeVisible();
		}
	);

	it.each(['default', 'variant', 'muted'] as const)('applies the %s color tone', async (tone) => {
		const { container } = await setup({ tone });

		expect(container.querySelector(`p.tone-${tone}`)).toBeVisible();
	});

	it('varies step and tone independently, so a quiet label is expressible', async () => {
		const { container } = await setup({ step: 'label', tone: 'muted' });

		expect(container.querySelector('p.step-label.tone-muted')).toBeVisible();
	});

	it('does not cap the line length unless measure is asked for', async () => {
		const { container } = await setup();

		expect(container.querySelector('p.measure')).toBeNull();
	});

	it('caps at --measure when measure is set (#609)', async () => {
		const { container } = await setup({ measure: true });

		expect(container.querySelector('p.measure')).toBeVisible();
	});

	// The id exists so a control can point at this sentence through
	// aria-describedby. The example used to be the Billing screen's own
	// 'buy-credits-help', removed with the unreachable branch that owned it
	// (#1162) -- this atom's behavior is unchanged, so the test keeps a
	// neutral id rather than following that one screen around.
	it('carries an id when one is given, for a control to reference via aria-describedby', async () => {
		const { container } = await setup({ id: 'field-help' });

		expect(container.querySelector('p#field-help')).toBeVisible();
	});

	it('carries no id when none is given', async () => {
		const { container } = await setup();

		expect(container.querySelector('p')?.id).toBe('');
	});
});
