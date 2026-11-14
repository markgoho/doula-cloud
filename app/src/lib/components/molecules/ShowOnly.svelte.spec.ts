import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import { createRawSnippet } from 'svelte';
import ShowOnly from './ShowOnly.svelte';

async function setup() {
	const children = createRawSnippet(() => ({
		render: () => '<label><input type="checkbox" name="mine" value="true" /> Mine</label>'
	}));
	return render(ShowOnly, { children });
}

describe('ShowOnly', () => {
	it('groups its boxes under "Show only", so each reads as the end of it', async () => {
		await setup();

		await expect.element(page.getByRole('group', { name: 'Show only' })).toBeVisible();
		await expect.element(page.getByRole('checkbox', { name: 'Mine' })).toBeVisible();
	});

	it('holds its boxes inside the group', async () => {
		await setup();

		const group = page.getByRole('group', { name: 'Show only' }).element();
		expect(group.contains(page.getByRole('checkbox', { name: 'Mine' }).element())).toBe(true);
	});
});
