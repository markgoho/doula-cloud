import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import { toPageState } from '../../../../routeFixture.js';
import { seedIntake } from './intakeFixture.js';
import NamePage from './name/+page.svelte';
import CheckPage from './check/+page.svelte';
import DuplicatePage from './duplicate/+page.svelte';
import { fixture as nameFixture } from './name/page.fixture.js';
import { fixture as checkFixture } from './check/page.fixture.js';
import { fixture as duplicateFixture } from './duplicate/page.fixture.js';

/*
 * The sequence's own behaviour (#466) -- the free save, the Change round
 * trip and the duplicate branch. The 320px conformance of each of its
 * eight pages is `route-continuum.svelte.spec.ts`'s, through the same
 * fixtures; this spec is only about what the pages DO.
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

const practiceId = 'practice-1';
const base = `/practices/${practiceId}/clients/new`;

beforeEach(() => {
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	apiFetchWithSession.mockReset();
	seedIntake();
});

function at(fixture: typeof nameFixture, search = '') {
	const state = toPageState(fixture);
	Object.assign(pageState, { ...state, url: new URL(`${state.url.href}${search}`) });
}

describe('the first question', () => {
	it('sends Continue on to the next question', async () => {
		at(nameFixture);
		await render(NamePage);

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/date-of-birth`);
	});

	// The Change round trip: a question reached from the summary returns
	// there rather than walking the rest of the sequence again.
	it('sends Continue back to the summary on a Change round trip', async () => {
		at(nameFixture, '?from=check');
		await render(NamePage);

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/check`);
	});

	// ADR-0017: only a given name is required, and a form that refuses to
	// save without a surname is a form that loses the record.
	it('refuses a Client with no given name, naming the field', async () => {
		at(nameFixture);
		intakeDraft.update({ givenName: '' });
		await render(NamePage);

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByText('There is a problem')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Enter the Client's given name" }))
			.toHaveAttribute('href', '#intake-given-name');
		expect(goto).not.toHaveBeenCalled();
	});

	// #466 removed #497's wait for all four match keys: the save is free
	// from every page of the sequence.
	it('saves from the first question and lands on the Client', async () => {
		at(nameFixture);
		apiFetchWithSession.mockResolvedValue(jsonResponse({ id: 'client-9' }, 201));
		await render(NamePage);

		await testPage.getByRole('button', { name: 'Save and come back later' }).click();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients/client-9`);
	});

	it('sends the whole record, not only what this page asked', async () => {
		at(nameFixture);
		apiFetchWithSession.mockResolvedValue(jsonResponse({ id: 'client-9' }, 201));
		await render(NamePage);

		await testPage.getByRole('button', { name: 'Save and come back later' }).click();

		const [, init] = apiFetchWithSession.mock.calls[0];
		const body = JSON.parse(init.body as string);
		expect(body).toMatchObject({
			givenName: 'Anne-Marie',
			addressPostalCode: '14472',
			dateOfBirth: '1988-02-09',
			override: false
		});
		expect(body.fieldValues.birthplace).toBe('Strong Memorial Hospital');
	});
});

describe('the summary', () => {
	it('lists every question asked, with a way back to each', async () => {
		at(checkFixture);
		await render(CheckPage);

		// The label and its Change link's visually-hidden text both say it.
		await expect.element(testPage.getByText('ZIP code').first()).toBeVisible();
		// Once as the summary's section heading, once as the rail's own
		// step label -- the rail is the same journey, listed beside it.
		await expect
			.element(testPage.getByText('What this Client wants from continuous labor support').first())
			.toBeVisible();
	});

	it('sends a refused save to the duplicate check rather than showing an error', async () => {
		at(checkFixture);
		apiFetchWithSession.mockResolvedValue(jsonResponse({ matches: [{ id: 'client-1' }] }, 409));
		await render(CheckPage);

		await testPage.getByRole('button', { name: 'Save this Client' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate`);
	});
});

describe('the duplicate check', () => {
	it('offers every match plus a different person', async () => {
		at(duplicateFixture);
		await render(DuplicatePage);

		// Two Clients with the same name is the case this page exists for.
		await expect
			.element(testPage.getByLabelText('Anne-Marie Ochieng-Whitfield').first())
			.toBeVisible();
		await expect.element(testPage.getByLabelText('No, this is a different person')).toBeVisible();
	});

	it('refuses to go on until one of them is chosen', async () => {
		at(duplicateFixture);
		await render(DuplicatePage);

		await testPage.getByRole('button', { name: 'Continue' }).click();

		// GOV.UK asks for the message twice -- once in the summary at the
		// top of the page, again against the group itself -- and the
		// summary's entry links to the first control in the group.
		await expect
			.element(testPage.getByRole('link', { name: 'Choose whether this is the same person' }))
			.toHaveAttribute('href', '#intake-same-person-client-1');
		await expect
			.element(testPage.getByText('Choose whether this is the same person').last())
			.toBeVisible();
	});

	// ADR-0017's one deliberate override: it skips the match query
	// entirely rather than asking again.
	it('re-sends with override when a different person is chosen', async () => {
		at(duplicateFixture);
		apiFetchWithSession.mockResolvedValue(jsonResponse({ id: 'client-9' }, 201));
		await render(DuplicatePage);

		await testPage.getByLabelText('No, this is a different person').click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		const [, init] = apiFetchWithSession.mock.calls[0];
		expect(JSON.parse(init.body as string).override).toBe(true);
		// The LAST call, not merely one of them: clearing the draft empties
		// `matches`, and a reactive empty-matches guard on this page would
		// fire on the way out and send a reader who had just saved to the
		// summary instead of to the Client.
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients/client-9`);
	});

	it('reviews the changes before writing them to a Client already on file', async () => {
		at(duplicateFixture);
		await render(DuplicatePage);

		await testPage.getByLabelText('Anne-Marie Ochieng-Whitfield').first().click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate?match=client-1`);
	});
});
