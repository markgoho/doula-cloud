import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractView from './ContractView.svelte';

interface SetupOptions {
	prose?: string;
	values?: Record<string, string>;
}

async function setup({
	prose = 'Agreement for {{client_name}} at {{price}}.',
	values = { client_name: 'Jamie', price: '$1,200' }
}: SetupOptions = {}) {
	await render(ContractView, { prose, values });
}

describe('ContractView.svelte', () => {
	it('renders the prose with every merge field placeholder filled in', async () => {
		await setup();

		await expect.element(page.getByText('Agreement for Jamie at $1,200.')).toBeInTheDocument();
	});

	it('renders an unfilled placeholder as blank', async () => {
		await setup({ prose: 'Agreement for {{client_name}}.', values: {} });

		await expect.element(page.getByText('Agreement for .')).toBeInTheDocument();
	});

	it('renders prose with no merge fields unchanged', async () => {
		await setup({ prose: 'Plain prose, no merge fields.', values: {} });

		await expect.element(page.getByText('Plain prose, no merge fields.')).toBeInTheDocument();
	});
});
