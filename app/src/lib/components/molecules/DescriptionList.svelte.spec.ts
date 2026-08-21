import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DescriptionList from './DescriptionList.svelte';

type SetupOptions = Partial<ComponentProps<typeof DescriptionList>>;

const defaultItems = [
	{ label: 'Status', value: 'Active' },
	{ label: 'Created', value: '1/1/2026' }
];

async function setup({ items = defaultItems, ...rest }: SetupOptions = {}) {
	const { container } = await render(DescriptionList, { items, ...rest });
	return { container };
}

describe('DescriptionList.svelte', () => {
	it('renders each item as a label/value pair', async () => {
		await setup();

		await expect.element(page.getByText('Status')).toBeVisible();
		await expect.element(page.getByText('Active')).toBeVisible();
		await expect.element(page.getByText('Created')).toBeVisible();
		await expect.element(page.getByText('1/1/2026')).toBeVisible();
	});

	it('renders as a semantic description list', async () => {
		const { container } = await setup();

		expect(container.querySelectorAll('dt')).toHaveLength(2);
		expect(container.querySelectorAll('dd')).toHaveLength(2);
	});
});
