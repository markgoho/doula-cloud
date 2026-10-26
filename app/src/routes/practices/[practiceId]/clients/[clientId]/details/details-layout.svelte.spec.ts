import { createRawSnippet } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { jsonResponse } from '#lib/testResponse.js';
import Layout from './+layout.svelte';
import { clientId, practiceId, record, seedDetails } from './detailsFixture.js';
import { load } from './+page.js';
import { detailsSave } from './detailsSave.svelte.js';

/*
 * The layout every question of a Client's details journey sits in, and
 * the door in front of it (#1610). What the questions themselves do is
 * `client-details.svelte.spec.ts`'s.
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

/**
Opens the journey for her and answers her record's read with `body`.
*/
async function setup(body: unknown = record, status = 200) {
	Object.assign(pageState, {
		params: { practiceId, clientId },
		url: new URL(`https://example.test/practices/${practiceId}/clients/${clientId}/details/email`)
	});
	clientDetails.open(clientId, undefined);
	apiFetchWithSession.mockResolvedValue(jsonResponse(body, status));
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>the email question</p>' }))
	});
}

beforeEach(() => {
	goto.mockReset();
	apiFetchWithSession.mockReset();
	sessionStorage.clear();
	// The Practice's template is already read; only her record is not.
	seedDetails();
});

describe('the details layout', () => {
	it('reads her record and shows the question', async () => {
		await setup();

		await expect.element(testPage.getByText('the email question')).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith(`/api/practices/${practiceId}/clients/${clientId}`);
		expect(clientDetails.draft.answers.email).toBe(record.email);
	});

	it('says why nothing can be added to an erased record', async () => {
		await setup({ ...record, erasedAt: '2026-09-01T00:00:00Z' });

		await expect.element(testPage.getByText('no details can be added')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Back to the Client's record" }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/${clientId}`);
		await expect.element(testPage.getByText('the email question')).not.toBeInTheDocument();
	});

	it('sends a merged record to the record it became', async () => {
		await setup({ id: clientId, mergedInto: 'client-2' });

		await vi.waitFor(() =>
			expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients/client-2`)
		);
		await expect.element(testPage.getByText('the email question')).not.toBeInTheDocument();
	});

	// A contractor Doula not attached to her reads a 404 from the API,
	// which is what refuses.
	it('shows the refusal of a record that cannot be read', async () => {
		await setup({ code: 'NOT_FOUND', message: 'client not found' }, 404);

		await expect.element(testPage.getByText('client not found')).toBeVisible();
		await expect.element(testPage.getByText('the email question')).not.toBeInTheDocument();
	});

	// #2048: the save's name-substitution question, asked here for
	// whichever page saved. What it does is `client-details.svelte.spec.ts`'s.
	it('clears the substitution question when it is canceled', async () => {
		await setup();
		detailsSave.matches = [{ ...record, id: 'client-2', createdAt: '2024-03-04T15:00:00Z', engagements: [], wouldSurvive: true }];
		detailsSave.overrideError = 'refused before';
		detailsSave.isConflictOpen = true;

		await expect.element(testPage.getByRole('dialog', { name: 'Possible duplicate Client' })).toBeVisible();
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		expect(detailsSave.matches).toEqual([]);
		expect(detailsSave.overrideError).toBe('');
	});
});

/**
Walks through the door, with the query a link put on it.
*/
function open(search = '') {
	return () =>
		load({
			params: { practiceId, clientId },
			url: new URL(`https://example.test/practices/${practiceId}/clients/${clientId}/details${search}`)
		} as Parameters<typeof load>[0]);
}

describe('the door', () => {
	it('starts again from the record on file and opens the first question', () => {
		clientDetails.draft.update({ phone: 'typed on an earlier visit' });

		expect(open()).toThrow(expect.objectContaining({
			status: 307,
			location: `/practices/${practiceId}/clients/${clientId}/details/name`
		}));
		expect(clientDetails.draft.answers.phone).toBe('');
		expect(clientDetails.engagementId).toBeUndefined();
	});

	// #2048: each Change on her record names its question.
	it.each([['email'], ['sections/1']])('opens the one question %s a Change on her record names', (slug) => {
		clientDetails.draft.update({ phone: 'typed on an earlier visit' });

		expect(open(`?step=${encodeURIComponent(slug)}`)).toThrow(expect.objectContaining({
			status: 307,
			location: `/practices/${practiceId}/clients/${clientId}/details/${slug}?from=record`
		}));
		expect(clientDetails.draft.answers.phone).toBe('');
	});

	it('opens the first question for a step the journey does not have', () => {
		expect(open('?step=check')).toThrow(expect.objectContaining({
			location: `/practices/${practiceId}/clients/${clientId}/details/name`
		}));
	});

	it('reads the Engagement whose page opened the journey', () => {
		expect(open('?engagement=engagement-1')).toThrow();
		expect(clientDetails.engagementId).toBe('engagement-1');
	});
});
