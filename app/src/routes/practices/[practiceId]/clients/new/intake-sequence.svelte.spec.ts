import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Component } from 'svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { intakeDraft, type IntakeAnswers } from '#lib/intakeDraft.svelte.js';
import { toPageState, type RouteFixture } from '../../../../routeFixture.js';
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
const detailHref = `/practices/${practiceId}/clients/client-9`;

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
	return async ({ search = '', respond, answers }: SetupOptions = {}) => {
		const state = toPageState(fixture);
		Object.assign(pageState, { ...state, url: new URL(`${state.url.href}${search}`) });
		if (answers) intakeDraft.update(answers);
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

describe('the first question', () => {
	const setup = setupFor(nameFixture, NamePage);

	it('sends Continue on to the next question', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/date-of-birth`);
	});

	// The Change round trip: a question reached from the summary returns
	// there rather than walking the rest of the sequence again.
	it('sends Continue back to the summary on a Change round trip', async () => {
		await setup({ search: '?from=check' });

		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/check`);
	});

	// ADR-0017: only a given name is required, and a form that refuses to
	// save without a surname is a form that loses the record.
	it('refuses a Client with no given name, naming the field', async () => {
		await setup({ answers: { givenName: '' } });

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
		await setup({ respond: jsonResponse({ id: 'client-9' }, 201) });

		await testPage.getByRole('button', { name: 'Save and come back later' }).click();

		expect(goto).toHaveBeenCalledWith(detailHref);
	});

	it('sends the whole record, not only what this page asked', async () => {
		const { sent } = await setup({ respond: jsonResponse({ id: 'client-9' }, 201) });

		await testPage.getByRole('button', { name: 'Save and come back later' }).click();

		expect(sent()).toMatchObject({
			givenName: 'Anne-Marie',
			addressPostalCode: '14472',
			dateOfBirth: '1988-02-09',
			override: false
		});
		expect((sent().fieldValues as Record<string, unknown>).birthplace).toBe(
			'Strong Memorial Hospital'
		);
	});
});

describe('the summary', () => {
	const setup = setupFor(checkFixture, CheckPage);

	it('lists every question asked, with a way back to each', async () => {
		await setup();

		// The label and its Change link's visually-hidden text both say it.
		await expect.element(testPage.getByText('ZIP code').first()).toBeVisible();
		// Once as the summary's section heading, once as the rail's own
		// step label -- the rail is the same journey, listed beside it.
		await expect
			.element(testPage.getByText('What this Client wants from continuous labor support').first())
			.toBeVisible();
	});

	// The save is free, so a question nobody answered is still a row --
	// its Change link is how the rest of the sequence stays reachable
	// from the end of it.
	it('says so where a question was not answered', async () => {
		await setup();

		await expect.element(testPage.getByText('Not answered').first()).toBeVisible();
	});

	it('sends a refused save to the duplicate check rather than showing an error', async () => {
		await setup({ respond: jsonResponse({ matches: [{ id: 'client-1' }] }, 409) });

		await testPage.getByRole('button', { name: 'Save this Client' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate`);
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
		const { sent } = await setup({ respond: jsonResponse({ id: 'client-9' }, 201) });

		await testPage.getByLabelText('No, this is a different person').click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(sent().override).toBe(true);
		// The LAST call, not merely one of them: clearing the draft empties
		// `matches`, and a reactive empty-matches guard on this page would
		// fire on the way out and send a reader who had just saved to the
		// summary instead of to the Client.
		expect(goto).toHaveBeenLastCalledWith(detailHref);
	});

	it('reviews the changes before writing them to a Client already on file', async () => {
		await setup({ answers: { phone: '+1 (585) 555-0199' } });

		await testPage.getByLabelText('Anne-Marie Ochieng-Whitfield').first().click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith(`${base}/duplicate?match=client-1`);
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
