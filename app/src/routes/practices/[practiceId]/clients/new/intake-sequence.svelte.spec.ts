import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Component } from 'svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { intakeDraft, type IntakeAnswers } from '#lib/intakeDraft.svelte.js';
import { clientSavedMessage, detailsSavedMessage, type ClientNames, type IntakeOrigin } from '#lib/intakeJourney.js';
import { captureLanding } from '#lib/testOutcome.js';
import { toPageState, type RouteFixture } from '../../../../routeFixture.js';
import { seedIntake } from './intakeFixture.js';
import NamePage from './name/+page.svelte';
import DuplicatePage from './duplicate/+page.svelte';
import { fixture as nameFixture } from './name/page.fixture.js';
import { fixture as duplicateFixture } from './duplicate/page.fixture.js';

/*
 * Intake's own behavior (#466, #1611) -- the save from the name question
 * and the duplicate branch. The 320px conformance of its two pages is
 * `route-continuum.svelte.spec.ts`'s, through the same fixtures; this
 * spec is only about what the pages DO.
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
const startWorkHref = `/practices/${practiceId}/clients/client-9/engagement-requests/new`;

interface SetupOptions {
	/** How the page was reached -- a Change link's round trip carries a
	 * query string, and nothing else on the sequence does. */
	search?: string;
	/**
	What the endpoint answers, for the tests that save.
	*/
	respond?: Response;
	/** What has been typed, where a test needs it to differ from the
	 * fixture's own Client. */
	answers?: Partial<IntakeAnswers>;
	/** The screen that opened intake directly (#1609); undefined is the
	 * search. */
	origin?: IntakeOrigin;
}

/*
 * `page` comes from the route's own fixture (#596), through the same
 * `toPageState` the continuum check installs, so the two never measure
 * different screens. The component is passed in rather than inferred
 * because three routes share this setup and each renders its own.
 */
function setupFor(fixture: RouteFixture, Page: Component<never>) {
	// The same one cast `route-continuum.svelte.spec.ts` makes where it
	// mounts a route, and for the same reason its comment gives: a
	// route's props are contravariant, so `Component<never>` is the only
	// type that accepts every route and the mount is where it is undone.
	const RoutePage = Page as Component;
	return async ({ search = '', respond, answers, origin }: SetupOptions = {}) => {
		const state = toPageState(fixture);
		Object.assign(pageState, { ...state, url: new URL(`${state.url.href}${search}`) });
		if (answers) intakeDraft.update(answers);
		intakeDraft.origin = origin;
		if (respond) apiFetchWithSession.mockResolvedValue(respond);
		await render(RoutePage);
		/**
		The body of the first request the page made, decoded.
		*/
		const sent = () =>
			JSON.parse(apiFetchWithSession.mock.calls[0][1].body as string) as Record<string, unknown>;
		return { sent };
	};
}

beforeEach(() => {
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	apiFetchWithSession.mockReset();
	seedIntake();
});

describe('the name question', () => {
	const setup = setupFor(nameFixture, NamePage);

	// #1611: one question, one button, and the save opens the Start work
	// form for the Client it saved.
	// #1710: and the Start work form says the Client is saved.
	it('saves the Client and opens the Start work form, which says the Client is saved', async () => {
		await setup({ respond: jsonResponse({ id: 'client-9' }, 201) });
		const message = clientSavedMessage(intakeDraft.answers);

		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Save and continue' }).click();

		await expect.poll(() => landing.href).toBe(startWorkHref);
		expect(landing.message).toBe(message);
	});

	it('offers no later steps and no save for later', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Save and continue' })).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Save and come back later' }))
			.not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Continue', exact: true })).not.toBeInTheDocument();
		await expect.element(testPage.getByText(/Step 1 of 1/)).not.toBeInTheDocument();
	});

	it('says which names are optional', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Family name (optional)')).toBeVisible();
		await expect.element(testPage.getByLabelText('Preferred name (optional)')).toBeVisible();
	});

	// What the search carried is saved with the name, so the page lists
	// it before the save: nothing is saved that she cannot see.
	it('lists what the search carried before the save', async () => {
		await setup();

		await expect.element(testPage.getByText('From your search, also saved with the name:')).toBeVisible();
		await expect.element(testPage.getByText('Feb 9, 1988')).toBeVisible();
		await expect
			.element(testPage.getByText('anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com'))
			.toBeVisible();
		await expect.element(testPage.getByText('+1 (585) 555-0142')).toBeVisible();
	});

	it('lists only what the search carried', async () => {
		await setup({ answers: { dateOfBirth: '', phone: '' } });

		await expect.element(testPage.getByText('Email address')).toBeVisible();
		await expect.element(testPage.getByText('Date of birth')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Phone number')).not.toBeInTheDocument();
	});

	it('lists nothing when the search carried nothing', async () => {
		await setup({ answers: { dateOfBirth: '', email: '', phone: '' } });

		await expect.element(testPage.getByRole('button', { name: 'Save and continue' })).toBeVisible();
		await expect
			.element(testPage.getByText('From your search, also saved with the name:'))
			.not.toBeInTheDocument();
	});

	it('saves the carried values with the name', async () => {
		const { sent } = await setup({ respond: jsonResponse({ id: 'client-9' }, 201) });

		await testPage.getByRole('button', { name: 'Save and continue' }).click();

		expect(sent()).toMatchObject({
			givenName: 'Anne-Marie',
			familyName: 'Ochieng-Whitfield',
			dateOfBirth: '1988-02-09',
			email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
			phone: '+1 (585) 555-0142',
			override: false
		});
	});

	it('sends a save with a match to the duplicate check rather than showing an error', async () => {
		await setup({ respond: jsonResponse({ matches: [{ id: 'client-1' }] }, 409) });

		await testPage.getByRole('button', { name: 'Save and continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate`);
	});

	// #488: a refusal the server names lands on the field it is about.
	it('points a refusal the server names at its field', async () => {
		await setup({
			respond: jsonResponse(
				{
					message: 'The Client record could not be saved.',
					details: { givenName: 'Given name is too long' }
				},
				400
			)
		});

		await testPage.getByRole('button', { name: 'Save and continue' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Given name is too long' }))
			.toHaveAttribute('href', '#intake-given-name');
		expect(goto).not.toHaveBeenCalled();
	});

	// #1609: Back goes to the screen that opened the name question -- the
	// overview or the Clients list of an empty Practice open it directly,
	// and the search opens it everywhere else.
	it.each([
		['the search', undefined, `/practices/${practiceId}/clients/search`],
		['the overview', 'overview', `/practices/${practiceId}`],
		['the Clients list', 'clients', `/practices/${practiceId}/clients`]
	] as const)('sends Back to %s that opened it', async (_screen, origin, href) => {
		await setup({ origin });

		await expect
			.element(testPage.getByRole('link', { name: 'Back', exact: true }))
			.toHaveAttribute('href', href);
	});

	// ADR-0017: only a given name is required, and a form that refuses to
	// save without a surname is a form that loses the record.
	it('refuses a Client with no given name, naming the field', async () => {
		await setup({ answers: { givenName: '' } });

		await testPage.getByRole('button', { name: 'Save and continue' }).click();

		await expect.element(testPage.getByText('There is a problem')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Enter the Client's given name" }))
			.toHaveAttribute('href', '#intake-given-name');
		await expect.element(testPage.getByLabelText('Given name')).toHaveAttribute('aria-invalid', 'true');
		expect(apiFetchWithSession).not.toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
	});

	// #1705: GOV.UK's "Error: " title prefix is for a page that was
	// really refused, never for a first visit.
	it('titles the page "Error: " only while a refusal is on it', async () => {
		await setup({ answers: { givenName: '' }, respond: jsonResponse({ id: 'client-9' }, 201) });

		expect(document.title).toMatch(/^What is the Client's name\?/);

		await testPage.getByRole('button', { name: 'Save and continue' }).click();
		await expect.poll(() => document.title).toMatch(/^Error: What is the Client's name\?/);

		await testPage.getByLabelText('Given name').fill('Pat');
		await testPage.getByRole('button', { name: 'Save and continue' }).click();
		await expect.poll(() => document.title).toMatch(/^What is the Client's name\?/);
	});

	it('keeps what is typed in the draft', async () => {
		await setup({ answers: { givenName: '', familyName: '', preferredName: '' } });

		await testPage.getByLabelText('Given name').fill('Pat');
		await testPage.getByLabelText('Family name (optional)').fill('Client');
		await testPage.getByLabelText('Preferred name (optional)').fill('P');

		expect(intakeDraft.answers).toMatchObject({ givenName: 'Pat', familyName: 'Client', preferredName: 'P' });
	});
});

describe('the duplicate check', () => {
	const setup = setupFor(duplicateFixture, DuplicatePage);

	it('offers every match plus a different person', async () => {
		await setup();

		// Two Clients with the same name is the case this page exists for.
		await expect
			.element(testPage.getByLabelText('Anne-Marie Ochieng-Whitfield').first())
			.toBeVisible();
		await expect.element(testPage.getByLabelText('No, this is a different person')).toBeVisible();
	});

	it('refuses to go on until one of them is chosen', async () => {
		await setup();
		// #1705: no "Error: " title until the page is really refused.
		expect(document.title).toMatch(/^Is this the same person\?/);

		await testPage.getByRole('button', { name: 'Continue' }).click();
		await expect.poll(() => document.title).toMatch(/^Error: Is this the same person\?/);

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

	it('sends Back to the name question', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Back', exact: true }))
			.toHaveAttribute('href', `${base}/name`);
	});

	// ADR-0017's one deliberate override: it skips the match query
	// entirely rather than asking again, and opens the Start work form
	// (#1611).
	it('re-sends with override when a different person is chosen', async () => {
		const { sent } = await setup({ respond: jsonResponse({ id: 'client-9' }, 201) });
		const message = clientSavedMessage(intakeDraft.answers);

		await testPage.getByLabelText('No, this is a different person').click();
		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(sent().override).toBe(true);
		await expect.poll(() => landing.message).toBe(message);
		// The LAST call, not merely one of them: clearing the draft empties
		// `matches`, and a reactive empty-matches guard on this page would
		// fire on the way out and send a reader who had just saved to the
		// name question instead of to the Start work form.
		expect(goto).toHaveBeenLastCalledWith(startWorkHref);
	});

	it('reviews the changes before writing them to a Client already on file', async () => {
		await setup({ answers: { phone: '+1 (585) 555-0199' } });

		await testPage.getByLabelText('Anne-Marie Ochieng-Whitfield').first().click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate?match=client-1`);
	});

	// #1710: the save leaves this page, so her record says it happened.
	it('writes the reviewed changes to the Client on file, and her record says so', async () => {
		const { sent } = await setup({
			search: '?match=client-1',
			answers: { phone: '+1 (585) 555-0199' },
			respond: jsonResponse({ id: 'client-1' })
		});

		const landing = captureLanding(goto);
		await testPage.getByRole('button', { name: 'Save changes to this record' }).click();

		await expect.poll(() => landing.href).toBe(`/practices/${practiceId}/clients/client-1`);
		expect(sent().phone).toBe('+1 (585) 555-0199');
		expect(landing.message).toBe(detailsSavedMessage(sent() as unknown as ClientNames));
	});

	// ADR-0017's "This is her" with nothing to propose: what was typed is
	// already what is on file, so there is nothing to confirm and nothing
	// to write, and a review screen listing no changes would be a page
	// asking a question with one answer.
	it('goes straight to the record when nothing typed differs from it', async () => {
		await setup();

		await testPage.getByLabelText('Anne-Marie Ochieng-Whitfield').first().click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients/client-1`);
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});
});
