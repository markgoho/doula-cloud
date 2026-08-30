import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientMatch } from '#lib/client.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' } }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const adaMatch: ClientMatch = {
	id: 'client-2',
	givenName: 'Ada',
	familyName: 'Byron',
	preferredName: '',
	email: 'ada.byron@example.com',
	phone: '555-0100',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '1815-12-10',
	engagements: [{ engagementId: 'engagement-1', kind: 'birth', status: 'active', createdAt: '2024-01-01' }]
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup() {
	await render(Page, {});
}

function requestUrl(callIndex = 0): string {
	return apiFetchWithSession.mock.calls[callIndex][0] as string;
}

describe('the search that fronts intake (#498)', () => {
	it('refuses a search with nothing typed, naming the reason and moving focus to it', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter a name, date of birth, email or phone to search' }))
			.toBeVisible();
		expect(document.activeElement?.textContent).toContain('There is a problem');
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('runs on name, date of birth, email and phone -- whatever the staff member has', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Ada');
		await testPage.getByLabelText('Date of birth').fill('1815-12-10');
		await testPage.getByLabelText('Email').fill('ada@example.com');
		await testPage.getByLabelText('Phone').fill('555-0100');
		await testPage.getByRole('button', { name: 'Search' }).click();

		expect(requestUrl()).toBe(
			'/api/practices/practice-1/clients/search?name=Ada&dateOfBirth=1815-12-10&email=ada%40example.com&phone=555-0100'
		);
	});

	it('prints each result\'s name and history unrestricted, linking to the Client detail hub (#494)', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [adaMatch] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Ada');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect.element(testPage.getByRole('heading', { level: 2, name: '1 match' })).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 3, name: 'Ada Byron' })).toBeVisible();
		await expect.element(testPage.getByText('1815-12-10')).toBeVisible();
		await expect.element(testPage.getByText('Birth · active')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Open Ada Byron's record" }))
			.toHaveAttribute('href', '/practices/practice-1/clients/client-2');
		// A client-side result set moves no focus on its own -- the results
		// heading takes it, so a screen reader announces the count.
		expect(document.activeElement?.id).toBe('client-search-results');
	});

	it('offers to start intake on an empty result set, carrying the typed name (#497)', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Name').fill('Nadia Haddad');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect.element(testPage.getByRole('heading', { level: 2, name: 'No matches' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', '/practices/practice-1/clients/new?name=Nadia%20Haddad');
	});

	it('offers to start intake with no name carried when none was typed', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [] }));
		await setup();

		await testPage.getByLabelText('Phone').fill('555-0100');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Add a new Client' }))
			.toHaveAttribute('href', '/practices/practice-1/clients/new');
	});

	it('surfaces a refusal as a readable message rather than a raw crash -- e.g. a contractor\'s 403 (#501)', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse('a contractor doula does not search for clients at a practice she contracts for', 403)
		);
		await setup();

		await testPage.getByLabelText('Name').fill('Ada');
		await testPage.getByRole('button', { name: 'Search' }).click();

		await expect
			.element(testPage.getByText('a contractor doula does not search for clients at a practice she contracts for'))
			.toBeVisible();
	});
});
