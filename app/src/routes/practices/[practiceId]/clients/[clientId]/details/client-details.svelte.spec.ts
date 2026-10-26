import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { createRawSnippet, type Component } from 'svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import type { IntakeAnswers } from '#lib/intakeDraft.svelte.js';
import { knownAsFrom, RECORD_QUERY } from '#lib/intakeJourney.js';
import { captureLanding } from '#lib/testOutcome.js';
import { toPageState, type RouteFixture } from '../../../../../routeFixture.js';
import { clientId, practiceId, record, seedDetails } from './detailsFixture.js';
import Layout from './+layout.svelte';
import { detailsSave } from './detailsSave.svelte.js';
import NamePage from './name/+page.svelte';
import DateOfBirthPage from './date-of-birth/+page.svelte';
import EmailPage from './email/+page.svelte';
import SectionPage from './sections/[sectionIndex]/+page.svelte';
import CheckPage from './check/+page.svelte';
import { fixture as nameFixture } from './name/page.fixture.js';
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
// What the screen the save lands on says (#1710).
const savedMessage = `${knownAsFrom(record)}'s details saved.`;

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
	/**
	The query the page was reached with, such as a Change's `?from=record`.
	*/
	search?: string;
}

function setupFor(fixture: RouteFixture, Page: Component<never>) {
	// The cast `route-continuum.svelte.spec.ts` makes where it mounts a
	// route, for the reason its comment gives.
	const RoutePage = Page as Component;
	return async ({ responses = [], answers, engagementId, search = '' }: SetupOptions = {}) => {
		const state = toPageState(fixture);
		Object.assign(pageState, { ...state, url: new URL(`${state.url.href}${search}`) });
		if (answers) clientDetails.draft.update(answers);
		clientDetails.engagementId = engagementId;
		for (const response of responses) apiFetchWithSession.mockResolvedValueOnce(response);
		await render(RoutePage);
		// The layout draws the save's one dialog (#2048). Its record and the
		// Practice's template are already seeded, so it reads nothing.
		await render(Layout, { children: createRawSnippet(() => ({ render: () => '<span></span>' })) });
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
	// The save's dialog is module state, so a test that leaves it open
	// would leave the next test's page inert behind it.
	detailsSave.isConflictOpen = false;
	detailsSave.cancel();
	seedDetails();
});

describe('the first question, her name', () => {
	const setup = setupFor(nameFixture, NamePage);

	it('starts from the name on file', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: /'s name\?$/ })).toBeVisible();
		await expect.element(testPage.getByLabelText('Given name')).toHaveValue(record.givenName);
		await expect.element(testPage.getByLabelText('Family name (optional)')).toHaveValue(record.familyName);
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

	// ADR-0017: a record needs a given name and nothing else.
	it('refuses a blank given name', async () => {
		await setup({ answers: { givenName: '  ' } });

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Enter a given name' })).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('goes on to the date of birth', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/date-of-birth`);
	});
});

/*
 * A Change on her record (#2048) opens one question, through the door,
 * with `?from=record`: the reader came to change one thing, so the page
 * saves it and goes back to the record with a status Notice.
 */
describe('a question a Change on her record opened', () => {
	const setup = setupFor(nameFixture, NamePage);
	const fromRecord = `?${RECORD_QUERY}`;

	it('is a journey of one, with Back to the record and a Save', async () => {
		await setup({ search: fromRecord });

		await expect
			.element(testPage.getByRole('link', { name: 'Back', exact: true }))
			.toHaveAttribute('href', recordHref);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Continue' })).not.toBeInTheDocument();
		// QuestionPage draws no rail for a journey of one step.
		await expect.element(testPage.getByRole('link', { name: /Date of birth/ })).not.toBeInTheDocument();
	});

	it('saves the change and says so on her record', async () => {
		const { sent } = await setup({ search: fromRecord, responses: [jsonResponse(record)] });

		await testPage.getByLabelText('Given name').fill('Annie');
		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Save' }).click();

		expect(sent()).toMatchObject({ givenName: 'Annie', email: record.email, override: false });
		await vi.waitFor(() => expect(landing).toEqual({ href: recordHref, message: "Annie's details saved." }));
	});

	// The duplicate check moved here from the Edit form: a name that
	// matches another Client asks "Is this the same person?", with Back
	// to this question.
	it('asks whether a matching name is the same person', async () => {
		await setup({
			search: fromRecord,
			responses: [jsonResponse({ matches: [match], substitution: false }, 409)]
		});

		await testPage.getByRole('button', { name: 'Save' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate`);
		expect(editMergeDraft.matches).toEqual([match]);
		expect(editMergeDraft.backHref).toBe(`${base}/name${fromRecord}`);
	});

	it('asks the substitution question in the layout’s dialog', async () => {
		await setup({
			search: fromRecord,
			responses: [jsonResponse({ matches: [match], substitution: true }, 409), jsonResponse(record)]
		});

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		await vi.waitFor(() => expect(landing).toEqual({ href: recordHref, message: savedMessage }));
	});

	it('shows a refused save on the question', async () => {
		await setup({
			search: fromRecord,
			responses: [jsonResponse({ code: 'INTERNAL', message: 'the save went wrong' }, 500)]
		});

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('There is a problem')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('the date of birth', () => {
	const setup = setupFor(dateOfBirthFixture, DateOfBirthPage);

	it('starts from the value on file and says it is optional', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: /date of birth\? \(optional\)$/ }))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Year')).toHaveValue('1988');
	});

	it('sends Back to the name', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Back', exact: true }))
			.toHaveAttribute('href', `${base}/name`);
	});

	// #1705: GOV.UK's "Error: " title prefix is for a page that was
	// really refused, never for a first visit.
	it('titles the page "Error: " only while a refusal is on it', async () => {
		await setup();
		expect(document.title).toMatch(/^What is .+'s date of birth\?/);

		await testPage.getByLabelText('Day').fill('32');
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.poll(() => document.title).toMatch(/^Error: What is .+'s date of birth\?/);

		await testPage.getByLabelText('Day').fill('3');
		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.poll(() => document.title).toMatch(/^What is .+'s date of birth\?/);
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
		await expect.element(testPage.getByText('cancels any portal invite', { exact: false })).not.toBeInTheDocument();
	});

	// What the Edit form said until #2048: a new address revokes an
	// invite still waiting at the old one.
	it('says a new address cancels a waiting portal invite', async () => {
		await setup();

		await testPage.getByRole('textbox').fill('anne@example.com');

		await expect.element(testPage.getByText('cancels any portal invite', { exact: false })).toBeVisible();
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

		const landing = captureLanding(goto);
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
		// #1710: her record says the save happened.
		expect(landing).toEqual({ href: recordHref, message: savedMessage });
	});

	it('returns to the Engagement page that opened the journey, and says the save happened there', async () => {
		await setup({ responses: [jsonResponse(record)], engagementId: 'engagement-1' });

		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(landing).toEqual({ href: engagementHref, message: savedMessage });
	});

	// #2048: a question cleared on its page clears the value on file, and
	// the row says so before the save.
	it('clears the value on file where a question was cleared', async () => {
		const { sent } = await setup({ responses: [jsonResponse(record)], answers: { email: '' } });

		await expect.element(testPage.getByText(record.email)).not.toBeInTheDocument();
		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(sent().email).toBe('');
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

	// Gate two: a possible duplicate is asked on the journey's own page,
	// with Back to this one.
	it('opens the journey’s duplicate page for a possible duplicate', async () => {
		await setup({ responses: [jsonResponse({ matches: [match], substitution: false }, 409)] });

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate`);
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
		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		await vi.waitFor(() => expect(landing).toEqual({ href: recordHref, message: savedMessage }));
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
		// #1705: no "Error: " title until the save is really refused.
		expect(document.title).toMatch(/^Check /);

		await testPage.getByRole('button', { name: 'Save these details' }).click();

		await expect.element(testPage.getByText('There is a problem')).toBeVisible();
		await expect
			.element(testPage.getByText("this client's data has been erased and cannot be edited").first())
			.toBeVisible();
		await expect.poll(() => document.title).toMatch(/^Error: Check /);
		expect(goto).not.toHaveBeenCalled();
	});
});
