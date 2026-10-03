import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Component } from 'svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import type { IntakeAnswers } from '#lib/intakeDraft.svelte.js';
import { toPageState, type RouteFixture } from '../../../../../routeFixture.js';
import { clientId, practiceId, record, seedDetails } from './detailsFixture.js';
import DateOfBirthPage from './date-of-birth/+page.svelte';
import EmailPage from './email/+page.svelte';
import SectionPage from './sections/[sectionIndex]/+page.svelte';
import CheckPage from './check/+page.svelte';
import { fixture as dateOfBirthFixture } from './date-of-birth/page.fixture.js';
import { fixture as emailFixture } from './email/page.fixture.js';
import { fixture as sectionFixture } from './sections/[sectionIndex]/page.fixture.js';
import { fixture as checkFixture } from './check/page.fixture.js';

/*
 * A Client's details journey (#1610): each page starts from the value
 * on file, and the check page saves once, through the edit path's two
 * gates. The 320px conformance of each page is the continuum sweep's,
 * through the same fixtures; this spec is only about what the pages DO.
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

const base = `/practices/${practiceId}/clients/${clientId}/details`;
const recordHref = `/practices/${practiceId}/clients/${clientId}`;
const engagementHref = `/practices/${practiceId}/engagements/engagement-1`;

const match = { ...record, id: 'client-2', engagements: [], wouldSurvive: true };

interface SetupOptions {
	/**
	What the endpoint answers, in order, for the tests that save.
	*/
	responses?: Response[];
	/** What has been typed, where a test needs it to differ from the
	 * fixture's own draft. */
	answers?: Partial<IntakeAnswers>;
	/**
	The Engagement whose page opened the journey.
	*/
	engagementId?: string;
}

function setupFor(fixture: RouteFixture, Page: Component<never>) {
	// The cast `route-continuum.svelte.spec.ts` makes where it mounts a
	// route, for the reason its comment gives.
	const RoutePage = Page as Component;
	return async ({ responses = [], answers, engagementId }: SetupOptions = {}) => {
		Object.assign(pageState, toPageState(fixture));
		if (answers) clientDetails.draft.update(answers);
		clientDetails.engagementId = engagementId;
		for (const response of responses) apiFetchWithSession.mockResolvedValueOnce(response);
		await render(RoutePage);
		return { sent };
	};
}

/**
The body of the nth request the page made, decoded.
*/
function sent(call = 0): Record<string, unknown> {
	return JSON.parse(apiFetchWithSession.mock.calls[call][1].body as string) as Record<string, unknown>;
}

beforeEach(() => {
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	apiFetchWithSession.mockReset();
	editMergeDraft.clear();
	seedDetails();
});

describe('the first question', () => {
	const setup = setupFor(dateOfBirthFixture, DateOfBirthPage);

	it('starts from the value on file and says it is optional', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: /date of birth\? \(optional\)$/ }))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Year')).toHaveValue('1988');
	});

	it.each([
		['her record', undefined, recordHref],
		['the Engagement page', 'engagement-1', engagementHref]
	])('sends Back to %s that opened it', async (_screen, engagementId, href) => {
		await setup({ engagementId });

		await expect
			.element(testPage.getByRole('link', { name: 'Back', exact: true }))
			.toHaveAttribute('href', href);
	});

	// The journey saves once, at its check page: the record already
	// exists, so there is nothing to keep for later.
	it('offers Continue and no save of its own', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/email`);
		await expect
			.element(testPage.getByRole('button', { name: 'Save and come back later' }))
			.not.toBeInTheDocument();
	});
});

describe('a question with a value on file', () => {
	const setup = setupFor(emailFixture, EmailPage);

	it('shows the value on file', async () => {
		await setup();

		await expect.element(testPage.getByRole('textbox')).toHaveValue(record.email);
	});
});

describe('a Practice section', () => {
	const setup = setupFor(sectionFixture, SectionPage);

	it('asks the Practice’s own questions, starting from the answers on file', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: /continuous labor support \(optional\)$/ }))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Partner')).toBeChecked();
	});
});

describe('the check page', () => {
	const setup = setupFor(checkFixture, CheckPage);

	it('saves one fact as one edit, with the record on file around it', async () => {
		const { sent } = await setup({ responses: [jsonResponse(record)] });

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
		expect(apiFetchWithSession.mock.calls[0][0]).toBe(`/api/practices/${practiceId}/clients/${clientId}`);
		expect(apiFetchWithSession.mock.calls[0][1].method).toBe('PUT');
		expect(sent()).toMatchObject({
			givenName: record.givenName,
			email: record.email,
			phone: '+1 (585) 555-0142',
			override: false
		});
		expect(goto).toHaveBeenCalledWith(recordHref);
	});

	it('returns to the Engagement page that opened the journey', async () => {
		await setup({ responses: [jsonResponse(record)], engagementId: 'engagement-1' });

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(goto).toHaveBeenCalledWith(engagementHref);
	});

	// A blank never replaces a value on file: the row says what the save
	// keeps, and the save keeps it.
	it('keeps the value on file where a question was left blank', async () => {
		const { sent } = await setup({ responses: [jsonResponse(record)], answers: { email: '' } });

		await expect.element(testPage.getByText(record.email).first()).toBeVisible();
		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(sent().email).toBe(record.email);
	});

	it('saves a Practice-defined value, and keeps one the Practice no longer asks', async () => {
		const { sent } = await setup({
			responses: [jsonResponse(record)],
			answers: { fieldValues: { ...record.fieldValues as object, hopes: 'A calm, dim room' } }
		});

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(sent().fieldValues).toMatchObject({
			hopes: 'A calm, dim room',
			retired: 'Answered before the Practice stopped asking'
		});
	});

	// Gate two: a possible duplicate is asked on the edit path's own page,
	// with Back to this one.
	it('opens the edit path’s duplicate page for a possible duplicate', async () => {
		await setup({ responses: [jsonResponse({ matches: [match], substitution: false }, 409)] });

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(goto).toHaveBeenCalledWith(`${recordHref}/edit/duplicate`);
		expect(editMergeDraft.matches).toEqual([match]);
		expect(editMergeDraft.fields.phone).toBe('+1 (585) 555-0142');
		expect(editMergeDraft.backHref).toBe(`${base}/check`);
	});

	// Gate one: the name on file now exactly matches another Client's.
	it('saves with the one override once a substitution is answered', async () => {
		const { sent } = await setup({
			responses: [jsonResponse({ matches: [match], substitution: true }, 409), jsonResponse(record)]
		});

		await testPage.getByRole('button', { name: 'Save these details' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(recordHref));
		expect(sent(1).override).toBe(true);
	});

	it('keeps the dialog open over a refused override', async () => {
		await setup({
			responses: [
				jsonResponse({ matches: [match], substitution: true }, 409),
				jsonResponse({ matches: [match], substitution: true }, 409)
			]
		});

		await testPage.getByRole('button', { name: 'Save these details' }).click();
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		await expect
			.element(testPage.getByRole('dialog').getByText('The Client record could not be saved.'))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('clears a refusal when the dialog is canceled', async () => {
		await setup({ responses: [jsonResponse({ matches: [match], substitution: true }, 409)] });

		await testPage.getByRole('button', { name: 'Save these details' }).click();
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
	});

	// edit.go refuses a record erased or merged while the journey was open.
	it('shows the refusal of a record that can no longer be edited', async () => {
		await setup({
			responses: [
				jsonResponse({ code: 'ALREADY_EXISTS', message: "this client's data has been erased and cannot be edited" }, 409)
			]
		});

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		await expect.element(testPage.getByText('There is a problem')).toBeVisible();
		await expect
			.element(testPage.getByText("this client's data has been erased and cannot be edited").first())
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
