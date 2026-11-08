import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import ChoiceGroup from './ChoiceGroup.svelte';

interface SetupOptions {
	legend?: string;
	hint?: string;
	error?: string;
}

async function setup(options: SetupOptions = {}) {
	const children = createRawSnippet(() => ({ render: () => '<p>The choices</p>' }));
	await render(ChoiceGroup, { name: 'topic', children, ...options });
}

describe('ChoiceGroup', () => {
	it('names the group by its legend and draws its choices inside', async () => {
		await setup({ legend: 'Topic' });

		const group = page.getByRole('group', { name: 'Topic' });
		await expect.element(group).toBeVisible();
		await expect.element(group.getByText('The choices')).toBeVisible();
	});

	it('describes the group by its hint', async () => {
		await setup({ legend: 'Topic', hint: 'Pick the one that fits.' });

		await expect.element(page.getByText('Pick the one that fits.')).toBeVisible();
		await expect
			.element(page.getByRole('group', { name: 'Topic' }))
			.toHaveAccessibleDescription('Pick the one that fits.');
	});

	it('announces the refusal and describes the group by it', async () => {
		await setup({ legend: 'Topic', hint: 'Pick the one that fits.', error: 'Choose a topic.' });

		await expect.element(page.getByRole('alert')).toHaveTextContent('Choose a topic.');
		await expect
			.element(page.getByRole('group', { name: 'Topic' }))
			.toHaveAccessibleDescription('Pick the one that fits. Choose a topic.');
	});

	it('draws no group of its own without a legend, and still announces a refusal', async () => {
		await setup({ error: 'Choose a topic.' });

		await expect.element(page.getByRole('group')).not.toBeInTheDocument();
		await expect.element(page.getByRole('alert')).toHaveTextContent('Choose a topic.');
		await expect.element(page.getByText('The choices')).toBeVisible();
	});

	it('announces no refusal when none was given', async () => {
		await setup({ legend: 'Topic' });

		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});
});
