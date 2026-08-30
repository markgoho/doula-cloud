import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { ClientMatch } from '#lib/client.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1', clientId: 'client-1' } }
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const baseDetail: ClientDetail = {
	id: 'client-1',
	givenName: 'Ada',
	familyName: 'Lovelace',
	preferredName: 'Ada',
	email: 'ada@example.com',
	phone: '555-0100',
	addressLine1: '1 Analytical Engine Way',
	addressLine2: '',
	addressLocality: 'London',
	addressRegion: 'LDN',
	addressPostalCode: 'SW1A 1AA',
	dateOfBirth: '1815-12-10',
	fieldValues: { pronouns: 'she/her' },
	resolvedFields: [],
	engagements: [],
	history: []
};

const anotherClientMatch: ClientMatch = {
	id: 'client-2',
	givenName: 'Ada',
	familyName: 'Byron',
	preferredName: '',
	email: 'ada.byron@example.com',
	phone: '',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '1815-12-10',
	engagements: []
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
});

async function setup(overrides: Partial<ClientDetail> = {}) {
	const detail = { ...baseDetail, ...overrides };
	apiFetchWithSession.mockResolvedValueOnce(jsonResponse(detail));
	await render(Page, {});
	await expect.element(testPage.getByLabelText('Given name')).toHaveValue(detail.givenName);
	return { detail };
}

function requestBody(callIndex: number): { override: boolean } {
	const init = apiFetchWithSession.mock.calls[callIndex][1] as RequestInit;
	return JSON.parse(init.body as string) as { override: boolean };
}

describe('client edit', () => {
	it('pre-fills the twelve structural columns from her current record', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Given name')).toHaveValue('Ada');
		await expect.element(testPage.getByLabelText('Family name')).toHaveValue('Lovelace');
		await expect.element(testPage.getByLabelText('Preferred name')).toHaveValue('Ada');
		await expect.element(testPage.getByLabelText('Email')).toHaveValue('ada@example.com');
		await expect.element(testPage.getByLabelText('Phone')).toHaveValue('555-0100');
		await expect.element(testPage.getByLabelText('Address line 1')).toHaveValue('1 Analytical Engine Way');
		await expect.element(testPage.getByLabelText('Town or city')).toHaveValue('London');
		await expect.element(testPage.getByLabelText('State')).toHaveValue('LDN');
		await expect.element(testPage.getByLabelText('Postal code')).toHaveValue('SW1A 1AA');
		await expect.element(testPage.getByLabelText('Date of birth')).toHaveValue('1815-12-10');
	});

	it('shows an error notice when the Client fails to load', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('client not found', 404));

		await render(Page, {});

		await expect.element(testPage.getByText('client not found')).toBeVisible();
	});

	it("refuses to save with a blank given name, client-side, before any request", async () => {
		await setup();

		await testPage.getByLabelText('Given name').fill('');
		await testPage.getByRole('button', { name: 'Save' }).click();

		// ErrorSummary renders this refusal as a link to the field it names.
		await expect
			.element(testPage.getByRole('link', { name: "Enter the Client's given name" }))
			.toBeVisible();
		// The load is the only request made -- the refusal never reached the network.
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	it('refuses a save that matches a different Client, naming the match, before writing anything', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [anotherClientMatch] }, 409));

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await expect.element(testPage.getByText('Ada Byron', { exact: false })).toBeVisible();
		expect(requestBody(1).override).toBe(false);
		expect(goto).not.toHaveBeenCalled();
	});

	it('saves after the deliberate override, retrying with override: true', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [anotherClientMatch] }, 409));
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(baseDetail));

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();

		await testPage.getByRole('button', { name: 'Save as a different person' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		expect(requestBody(2).override).toBe(true);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-1');
	});

	it('surfaces the endpoint refusal as an error rather than a silent failure', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('client not found', 404));

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('client not found')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('returns to the Client detail hub after a successful save', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(baseDetail));

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-1');
	});

	it('shows that a changed email revokes any pending portal invite', async () => {
		await setup();

		await expect
			.element(testPage.getByText('revokes any pending portal invite', { exact: false }))
			.not.toBeInTheDocument();

		await testPage.getByLabelText('Email').fill('new@example.com');

		await expect
			.element(testPage.getByText('revokes any pending portal invite', { exact: false }))
			.toBeVisible();
	});

	it('exposes Save and Cancel as reachable, labelled controls', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Cancel' }))
			.toHaveAttribute('href', '/practices/practice-1/clients/client-1');
	});
});
