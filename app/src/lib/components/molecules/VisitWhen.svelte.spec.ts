import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import VisitWhen from './VisitWhen.svelte';

const AT = '2026-11-13T14:00:00-05:00';
const ZONE = 'America/New_York';

describe('VisitWhen', () => {
	it('says the day and the time with the comma, in a time that carries the instant', async () => {
		await render(VisitWhen, { at: AT, timeZone: ZONE });

		const time = page.getByText('Fri Nov 13,').element().parentElement!;
		expect(time.tagName).toBe('TIME');
		expect(time.getAttribute('datetime')).toBe(AT);
		expect(time.textContent).toMatch(/^\w{3} \w{3} \d{1,2}, \d{1,2}:\d{2}[ap]m$/);
	});

	it('says the same words in the table form and on a card', async () => {
		await render(VisitWhen, { at: AT, timeZone: ZONE, isTracked: true });
		const tracked = page.getByText('Fri Nov 13,').element().parentElement!.textContent;
		await render(VisitWhen, { at: AT, timeZone: ZONE });
		const plain = page.getByText('Fri Nov 13,').last().element().parentElement!.textContent;

		expect(tracked).toBe(plain);
	});

	it('says "Not yet scheduled" in plain text when the Visit has no instant', async () => {
		await render(VisitWhen, { timeZone: ZONE });

		await expect.element(page.getByText('Not yet scheduled')).toBeVisible();
		expect(page.getByText('Not yet scheduled').element().closest('time')).toBeNull();
	});
});
