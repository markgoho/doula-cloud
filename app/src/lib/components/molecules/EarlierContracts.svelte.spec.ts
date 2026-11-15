import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import EarlierContracts from './EarlierContracts.svelte';

const TIME_ZONE = 'America/New_York';

describe('EarlierContracts', () => {
	it('says the day, who voided it, and the request that asked', async () => {
		await render(EarlierContracts, {
			timeZone: TIME_ZONE,
			earlier: [
				{
					id: 'c1',
					voidedAt: '2026-11-09T15:00:00Z',
					voidedBy: 'Dee Whitlock',
					requestedBy: 'Kimiko Ito',
					reason: 'Signed the wrong template'
				}
			]
		});

		await expect.element(page.getByRole('heading', { name: 'Earlier Contract' })).toBeVisible();
		await expect.element(page.getByText('Nov 9, 2026 by Dee Whitlock')).toBeVisible();
		await expect.element(page.getByText('Kimiko Ito asked: Signed the wrong template')).toBeVisible();
	});

	it('anchors each one by its id, and draws no reason where none was asked', async () => {
		const { container } = await render(EarlierContracts, {
			timeZone: TIME_ZONE,
			earlier: [
				{ id: 'c2', voidedAt: '2026-11-10T15:00:00Z' },
				{ id: 'c1', voidedAt: '2026-11-09T15:00:00Z' }
			]
		});

		await expect.element(page.getByRole('heading', { name: 'Earlier Contracts' })).toBeVisible();
		// Case 3 of the rule: an id is not in the accessible tree.
		expect(container.querySelector('#contract-c1')).not.toBeNull();
		expect(container.querySelector('#contract-c2')).not.toBeNull();
		expect(page.getByText('Reason').elements()).toHaveLength(0);
		await expect.element(page.getByText('Nov 10, 2026')).toBeVisible();
	});

	it('draws nothing for no earlier Contract', async () => {
		await render(EarlierContracts, { timeZone: TIME_ZONE, earlier: [] });

		expect(page.getByRole('heading').elements()).toHaveLength(0);
	});
});
