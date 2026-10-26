import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toPageState } from '../../../routeFixture.js';
import { signupDraft } from '../signupJourney.svelte.js';
import Page from './+page.svelte';
import { fixture } from './page.fixture.js';

/*
 * #1166: the zone the founder states for her Practice, on the journey's
 * "Where do you work?" page (#2049).
 *
 * A file of its own because these tests need the browser to report no
 * zone at all -- the one case the pre-selection cannot cover, and the
 * only way the page's own "choose a timezone" refusal is reachable.
 */

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

/* A browser that will not say where it is -- a locked-down profile, or a
   runtime with no zone data. The seven options still render; nothing is
   chosen for her. */
vi.mock('#lib/timezones.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/timezones.js')>()),
	detectTimezone: () => ''
}));

beforeEach(() => {
	goto.mockReset();
	signupDraft.clear();
	Object.assign(pageState, toPageState(fixture));
});

async function setup() {
	await render(Page);
}

describe('the zone a Practice is created in (#1166)', () => {
	it('refuses to continue with no zone chosen', async () => {
		await setup();
		await testPage.getByRole('combobox', { name: 'Which state do you work from?' }).selectOptions('New Jersey');
		await testPage.getByRole('button', { name: 'Continue' }).click();

		// The summary entry, which is the one that moves focus to the
		// control; the same sentence also appears beside the control.
		await expect
			.element(testPage.getByRole('link', { name: 'Choose the timezone this Practice works in' }))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('offers the seven US zones by the name a person would say', async () => {
		await setup();

		const control = testPage.getByRole('combobox', { name: 'What timezone does this Practice work in?' });
		await expect.element(control).toBeVisible();
		// Read off the control: an <option> inside a closed <select> is not
		// a visible element, and the order matters as much as the
		// membership -- east to west, the way the country is usually read.
		const labels = [...control.element().querySelectorAll('option')].map((option) => option.textContent);
		expect(labels).toEqual([
			'Choose a timezone',
			'Eastern time (New York)',
			'Central time (Chicago)',
			'Mountain time (Denver)',
			'Mountain time, no daylight saving (Phoenix)',
			'Pacific time (Los Angeles)',
			'Alaska time (Anchorage)',
			'Hawaii time (Honolulu)'
		]);
	});
});
