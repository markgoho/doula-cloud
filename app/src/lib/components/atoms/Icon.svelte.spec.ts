import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Icon, { SIZE_THRESHOLD_FOR_DUOTONE } from './Icon.svelte';

type SetupOptions = Partial<ComponentProps<typeof Icon>>;

async function setup({ name = 'check', ...rest }: SetupOptions = {}) {
	return await render(Icon, { name, ...rest });
}

describe('Icon.svelte', () => {
	it('is decorative by default', async () => {
		const { container } = await setup();

		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('aria-hidden', 'true');
		expect(svg).not.toHaveAttribute('role');
		expect(svg).not.toHaveAttribute('aria-label');
	});

	it('exposes an accessible name and role="img" when given a label', async () => {
		await setup({ label: 'Success' });

		await expect.element(page.getByRole('img', { name: 'Success' })).toBeInTheDocument();
	});

	it('omits aria-hidden when labeled', async () => {
		const { container } = await setup({ label: 'Success' });

		expect(container.querySelector('svg')).not.toHaveAttribute('aria-hidden');
	});

	it('renders the duotone weight (a distinct icon-fg and icon-bg path) at or above the threshold', async () => {
		const { container } = await setup({ size: SIZE_THRESHOLD_FOR_DUOTONE });

		expect(container.querySelectorAll(':scope svg path')).toHaveLength(2);
		expect(container.querySelector(':scope svg path.icon-fg')).toBeInTheDocument();
		expect(container.querySelector(':scope svg path.icon-bg')).toBeInTheDocument();
	});

	it('renders the light weight (a single icon-fg path) below the threshold', async () => {
		const { container } = await setup({ size: SIZE_THRESHOLD_FOR_DUOTONE - 1 });

		expect(container.querySelectorAll(':scope svg path')).toHaveLength(1);
		expect(container.querySelector(':scope svg path.icon-fg')).toBeInTheDocument();
	});

	it('honors an explicit weight override above the threshold', async () => {
		const { container } = await setup({ size: SIZE_THRESHOLD_FOR_DUOTONE, weight: 'light' });

		expect(container.querySelectorAll(':scope svg path')).toHaveLength(1);
	});

	it('honors an explicit weight override below the threshold', async () => {
		const { container } = await setup({ size: SIZE_THRESHOLD_FOR_DUOTONE - 1, weight: 'duotone' });

		expect(container.querySelectorAll(':scope svg path')).toHaveLength(2);
	});

	it('marks the svg non-focusable', async () => {
		const { container } = await setup();

		expect(container.querySelector('svg')).toHaveAttribute('focusable', 'false');
	});

	it('sizes the svg from the size prop', async () => {
		const { container } = await setup({ size: 32 });

		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('width', '32');
		expect(svg).toHaveAttribute('height', '32');
	});

	it('defaults to a 24px, duotone icon', async () => {
		const { container } = await setup();

		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('width', '24');
		expect(container.querySelectorAll(':scope svg path')).toHaveLength(2);
	});
});
