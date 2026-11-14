import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import CoverageGapWhen from './CoverageGapWhen.svelte';

const ZONE = 'America/New_York';
const NOW = new Date('2026-11-13T17:00:00Z');
const STARTS = '2026-11-14T23:00:00Z';

async function setup(endsAt: string) {
	const { container } = await render(CoverageGapWhen, { startsAt: STARTS, endsAt, timeZone: ZONE, now: NOW });
	return { container, times: () => [...container.querySelectorAll('time')] };
}

describe('CoverageGapWhen', () => {
	it('says when the gap runs across two days, each end in a time that keeps its instant', async () => {
		const { container, times } = await setup('2026-11-15T14:00:00Z');

		await expect.element(page.getByText('Sat Nov 14, 6:00pm')).toBeVisible();
		expect(container.textContent?.trim()).toBe('Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am');
		expect(times().map((time) => time.getAttribute('datetime'))).toEqual([STARTS, '2026-11-15T14:00:00Z']);
	});

	it('says the clock alone for an end on the same day', async () => {
		const { container } = await setup('2026-11-15T02:00:00Z');

		expect(container.textContent?.trim()).toBe('Sat Nov 14, 6:00pm to 9:00pm');
	});
});
