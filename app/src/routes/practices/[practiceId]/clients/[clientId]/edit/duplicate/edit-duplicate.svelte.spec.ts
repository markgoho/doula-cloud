import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { CollisionMatch } from '#lib/client.js';
import { displayName } from '#lib/clientDetail.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import { toPageState } from '../../../../../../routeFixture.js';
import Page from './+page.svelte';
import { clientId, fields, fixture, matches, practiceId, seedEditMergeDraft } from './page.fixture.js';

/*
 * Gate two's question, on the edit path (#814). Modelled on
 * `intake-sequence.svelte.spec.ts`'s own "the duplicate check" block --
 * this route has no multi-step sequence around it, so it gets a spec of
 * its own rather than a shared one.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const editHref = `/practices/${practiceId}/clients/${clientId}/edit`;
const base = `${editHref}/duplicate`;

function detailHref(id: string): string {
	return `/practices/${practiceId}/clients/${id}`;
}

function requestBody(callIndex: number): { override?: boolean; otherClientId?: string } {
	const init = apiFetchWithSession.mock.calls[callIndex][1] as RequestInit;
	return JSON.parse(init.body as string) as { override?: boolean; otherClientId?: string };
}

interface SetupOptions {
	/**
	 * A Change-link-style round trip, or a chosen match's own review state.
	 */
	search?: string;
	respond?: Response;
}

async function setup({ search = '', respond }: SetupOptions = {}) {
	const state = toPageState(fixture);
	Object.assign(pageState, { ...state, url: new URL(`${state.url.href}${search}`) });
	if (respond) apiFetchWithSession.mockResolvedValue(respond);
	await render(Page);
}

beforeEach(() => {
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	apiFetchWithSession.mockReset();
	seedEditMergeDraft();
});

describe('a direct load with nothing in the draft', () => {
	it('sends the reader back to the edit page', async () => {
		editMergeDraft.clear();

		await setup();

		expect(goto).toHaveBeenCalledWith(editHref);
	});
});

describe('when a merge is possible (mergeOffered: true)', () => {
	it('offers every match plus a different person, and says nothing has been saved yet', async () => {
		await setup();

		await expect
			.element(testPage.getByLabelText(displayName(matches[0])).first())
			.toBeVisible();
		await expect.element(testPage.getByLabelText('No, a different person')).toBeVisible();
		await expect
			.element(testPage.getByText('Nothing has been saved yet', { exact: false }))
			.toBeVisible();
	});

	it('refuses to go on until one of them is chosen', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Choose whether this is the same person' }))
			.toBeVisible();
	});

	it('re-sends the edit with override when a different person is chosen', async () => {
		await setup({ respond: jsonResponse({ id: clientId, ...fields }) });

		await testPage.getByLabelText('No, a different person').click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(requestBody(0).override).toBe(true);
		expect(goto).toHaveBeenCalledWith(detailHref(clientId));
	});

	it('reviews the changes before writing them', async () => {
		await setup();

		await testPage.getByLabelText(displayName(matches[0])).first().click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}?match=${matches[0]!.id}`);
	});

	it('names the match as the survivor and saves the merge on confirm (wouldSurvive: true)', async () => {
		await setup({
			search: `?match=${matches[0]!.id}`,
			respond: jsonResponse({ id: matches[0]!.id, ...fields })
		});

		await expect
			.element(testPage.getByText(`Save these changes to ${displayName(matches[0])}?`))
			.toBeVisible();

		await testPage.getByRole('button', { name: 'Save changes' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/clients/${clientId}/merge`,
			expect.objectContaining({ method: 'POST' })
		);
		expect(requestBody(0).otherClientId).toBe(matches[0]!.id);
		expect(goto).toHaveBeenCalledWith(detailHref(matches[0]!.id));
	});

	it('goes straight to the merge when nothing typed differs from the survivor', async () => {
		const noChangeMatch: CollisionMatch = { ...matches[0]!, ...fields, id: 'client-5', wouldSurvive: true };
		editMergeDraft.open(clientId, fields, [noChangeMatch], true);
		await setup({ respond: jsonResponse({ id: noChangeMatch.id, ...fields }) });

		await testPage.getByLabelText(displayName(noChangeMatch)).first().click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(detailHref(noChangeMatch.id));
	});

	it("names the record being edited as the survivor when it is the older, unattached row (wouldSurvive: false)", async () => {
		const survivorFields = { ...fields, familyName: 'Okafor', phone: '' };
		const absorbed: CollisionMatch = {
			...matches[1]!,
			id: 'client-6',
			familyName: 'Okafor-Reid',
			phone: '555-0199',
			wouldSurvive: false
		};
		editMergeDraft.open(clientId, survivorFields, [absorbed], true);
		const survivorName = displayName({ ...survivorFields, id: clientId });
		await setup({ search: `?match=${absorbed.id}`, respond: jsonResponse({ id: clientId, ...survivorFields }) });

		await expect
			.element(testPage.getByText(`Save these changes to ${survivorName}?`))
			.toBeVisible();
		// The absorbed match's own non-blank values win over what was typed --
		// the reverse of the ordinary direction, per `fold`.
		await expect.element(testPage.getByText('Okafor → Okafor-Reid')).toBeVisible();

		await testPage.getByRole('button', { name: 'Save changes' }).click();

		expect(requestBody(0).otherClientId).toBe(absorbed.id);
		expect(goto).toHaveBeenCalledWith(detailHref(clientId));
	});

	it('surfaces a failed merge through the error summary', async () => {
		await setup({
			search: `?match=${matches[0]!.id}`,
			respond: jsonResponse('this client record has already been merged into another', 409)
		});

		await testPage.getByRole('button', { name: 'Save changes' }).click();

		await expect
			.element(testPage.getByText('this client record has already been merged into another'))
			.toBeVisible();
	});
});

describe('when no merge is possible (mergeOffered: false)', () => {
	it('offers no match, explains why, and saves as a different person on the only choice', async () => {
		editMergeDraft.open(clientId, fields, matches, false);
		await setup({ respond: jsonResponse({ id: clientId, ...fields }) });

		await expect.element(testPage.getByRole('radio')).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText("can't be combined with another record", { exact: false }))
			.toBeVisible();

		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		expect(requestBody(0).override).toBe(true);
		expect(goto).toHaveBeenCalledWith(detailHref(clientId));
	});
});
