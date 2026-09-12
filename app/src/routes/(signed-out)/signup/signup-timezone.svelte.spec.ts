import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

/*
 * #1166: the zone the founder states for her Practice.
 *
 * A file of its own because these tests need the browser to report no
 * zone at all -- the one case the pre-selection cannot cover, and the
 * only way the form's own "choose a timezone" refusal is reachable. The
 * rest of the signup screen's tests want the ordinary pre-selected form,
 * so mocking the detection there would change what they are testing.
 */

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const createUserWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signInWithEmailAndPassword = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn());
vi.mock('firebase/auth', () => ({
	createUserWithEmailAndPassword,
	signInWithEmailAndPassword,
	signOut
}));
vi.mock('#lib/firebase.js', () => ({ getFirebaseAuth: () => ({}) }));
vi.mock('#lib/api.js', () => ({ apiBaseURL: () => '' }));

/* A browser that will not say where it is -- a locked-down profile, or a
   runtime with no zone data. The seven options still render; nothing is
   chosen for her. */
vi.mock('#lib/timezones.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/timezones.js')>()),
	detectTimezone: () => ''
}));

const globalFetch = vi.hoisted(() => vi.fn());

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	for (const mock of [goto, createUserWithEmailAndPassword, globalFetch]) mock.mockReset();
});

async function fillEverythingButTheZone() {
	await testPage.getByLabelText('Practice name').fill('Riverside Doulas');
	await testPage.getByLabelText('Your name').fill('Priya Sharma');
	await testPage
		.getByRole('combobox', { name: 'Which state do you work from?' })
		.selectOptions('New Jersey');
	await testPage.getByLabelText('Email').fill('priya@example.com');
	await testPage.getByLabelText('Password').fill('correct horse');
}

const submit = () => testPage.getByRole('button', { name: 'Create Practice' }).click();

async function setup() {
	await render(Page, {});
}

describe("the zone a Practice is created in (#1166)", () => {
	it('refuses a signup with no zone chosen, and sends nothing', async () => {
		await setup();
		await fillEverythingButTheZone();
		await submit();

		// The summary entry at the top of the page, which is the one a
		// person meets first and the one that moves focus to the control.
		// The same sentence also appears beside the control itself, which
		// is why this names the link rather than the text.
		await expect
			.element(
				testPage.getByRole('link', { name: 'Choose the timezone this Practice works in' })
			)
			.toBeVisible();
		expect(globalFetch).not.toHaveBeenCalled();
		expect(createUserWithEmailAndPassword).not.toHaveBeenCalled();
	});

	it('offers the seven US zones by the name a person would say', async () => {
		await setup();

		const control = testPage.getByRole('combobox', {
			name: 'What timezone does this Practice work in?'
		});
		await expect.element(control).toBeVisible();
		// Read off the control rather than asserted one option at a time:
		// an <option> inside a closed <select> is not a visible element,
		// and the order matters as much as the membership -- the list runs
		// east to west, the way the country is usually read.
		const labels = [...control.element().querySelectorAll('option')].map(
			(option) => option.textContent
		);
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
