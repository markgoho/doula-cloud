import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import VisitDoula from './VisitDoula.svelte';

describe('VisitDoula', () => {
	it('says only her name while she is on the Engagement', async () => {
		await render(VisitDoula, { name: 'Priya Raman', isOffEngagement: false });

		await expect.element(page.getByText('Priya Raman')).toBeVisible();
		expect(page.getByText('No longer on this work').elements()).toHaveLength(0);
	});

	it('says in words when she is no longer on the Engagement, after her name', async () => {
		await render(VisitDoula, { name: 'Priya Raman', isOffEngagement: true });

		await expect.element(page.getByText('Priya Raman')).toBeVisible();
		await expect.element(page.getByText('No longer on this work')).toBeVisible();
		// A screen reader hears the two as one phrase, joined by the comma
		// the page does not draw.
		const doula = page.getByText('Priya Raman').element().parentElement!;
		expect(doula.textContent!.replaceAll(/\s+/g, ' ')).toBe('Priya Raman, No longer on this work');
	});
});
