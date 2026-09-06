import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { CollisionMatch } from '#lib/client.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../../routeFixture.js';
import { detail as baseDetail, fixture } from './page.fixture.js';

/*
 * The Client this form edits, and the `page` it reads, both come from the
 * route's own fixture (#596) -- so what this spec asserts on and what the
 * continuum sweep measures are one description. `vi.mock` is hoisted
 * above every import, so `pageState` is declared empty and filled from
 * the fixture once the imports have run. Same installation, through the
 * same `toPageState`, as `route-continuum.svelte.spec.ts`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId, clientId } = fixture.params;
const detailHref = `/practices/${practiceId}/clients/${clientId}`;
const editDuplicateHref = `${detailHref}/edit/duplicate`;

const anotherClientMatch: CollisionMatch = {
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
	wouldSurvive: false,
	engagements: []
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	editMergeDraft.clear();
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

		await expect.element(testPage.getByLabelText('Given name')).toHaveValue(baseDetail.givenName);
		await expect.element(testPage.getByLabelText('Family name')).toHaveValue(baseDetail.familyName);
		// The fixture's Client carries no preferred name (#537's vocabulary
		// keeps the free-text fields hostile, not every field non-blank), so
		// this asserts the honest blank rather than a value invented for a
		// tidier assertion.
		await expect.element(testPage.getByLabelText('Preferred name')).toHaveValue(baseDetail.preferredName);
		await expect.element(testPage.getByLabelText('Email')).toHaveValue(baseDetail.email);
		await expect.element(testPage.getByLabelText('Phone')).toHaveValue(baseDetail.phone);
		await expect.element(testPage.getByLabelText('Address line 1')).toHaveValue(baseDetail.addressLine1);
		await expect.element(testPage.getByLabelText('Town or city')).toHaveValue(baseDetail.addressLocality);
		await expect.element(testPage.getByLabelText('State')).toHaveValue(baseDetail.addressRegion);
		await expect.element(testPage.getByLabelText('Postal code')).toHaveValue(baseDetail.addressPostalCode);
		await expect.element(testPage.getByLabelText('Date of birth')).toHaveValue(baseDetail.dateOfBirth);
	});

	it('shows an error notice when the Client fails to load', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('client not found', 404));

		await render(Page, {});

		await expect.element(testPage.getByText('client not found')).toBeVisible();
	});

	it('redirects to the survivor rather than rendering a tombstoned record', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...baseDetail, mergedInto: 'client-9' }));

		await render(Page, {});

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients/client-9`);
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

	it('refuses a save that exactly matches a different Client (gate one), naming the match, before writing anything', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true, mergeOffered: false }, 409)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await expect.element(testPage.getByText('Ada Byron', { exact: false })).toBeVisible();
		expect(requestBody(1).override).toBe(false);
		expect(goto).not.toHaveBeenCalled();
	});

	it('saves after the deliberate override, retrying with override: true', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true, mergeOffered: false }, 409)
		);
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(baseDetail));

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();

		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		expect(requestBody(2).override).toBe(true);
		expect(goto).toHaveBeenCalledWith(detailHref);
	});

	it('sends a possible duplicate (gate two) to its own question page rather than a dialog', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{ matches: [{ ...anotherClientMatch, wouldSurvive: true }], substitution: false, mergeOffered: true },
				409
			)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		expect(goto).toHaveBeenCalledWith(editDuplicateHref);
		expect(editMergeDraft.clientId).toBe(clientId);
		expect(editMergeDraft.matches).toEqual([{ ...anotherClientMatch, wouldSurvive: true }]);
		expect(editMergeDraft.mergeOffered).toBe(true);
		expect(editMergeDraft.fields.givenName).toBe(baseDetail.givenName);
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
		expect(goto).toHaveBeenCalledWith(detailHref);
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
			.toHaveAttribute('href', detailHref);
	});
});
