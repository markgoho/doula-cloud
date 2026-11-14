/*
 * #1093's setting: when a Doula goes on call for a birth, and how long
 * she stays on call past the due date. Read and written by an Owner or
 * an Admin. The screen's job is to show the rule the Practice holds, say
 * what changing it does before the press, and send the choice -- the
 * bounds themselves belong to the BFF, which says them in its own words.
 */
// jscpd:ignore-start -- the route-spec imports and mock preamble every settings screen's spec repeats
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { asDoula, fixture, settings } from './page.fixture.js';
import '#lib/styles/app.css';
import { pageState } from '#testing/routePage.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession
}));
// jscpd:ignore-end

const navigated = vi.hoisted(() => ({
	callback: undefined as undefined | ((navigation: { from: { url: URL } | null }) => void)
}));
vi.mock('$app/navigation', () => ({
	afterNavigate: (callback: (navigation: { from: { url: URL } | null }) => void) => {
		navigated.callback = callback;
	}
}));

const settingsPath = '/api/practices/practice-1/on-call-settings';

const week = () => testPage.getByLabelText('Week of pregnancy', { exact: true });
const grace = () => testPage.getByLabelText('Days on call after the due date', { exact: true });
const save = () => testPage.getByRole('button', { name: 'Save' }).click();
const savedStatus = () => testPage.getByRole('status').filter({ hasText: 'On-call rule saved.' });
const pathsFetched = () => apiFetchWithSession.mock.calls.map((call) => String(call[0]));

beforeEach(() => {
	apiFetchWithSession.mockReset();
	navigated.callback = undefined;
	Object.assign(pageState, toPageState(fixture));
});

async function setup(loadResponse: Response = jsonResponse(settings)) {
	apiFetchWithSession.mockResolvedValueOnce(loadResponse);
	await render(Page, {});
}

describe('the on-call rule screen', () => {
	it('opens on the rule the Practice holds', async () => {
		await setup();

		await expect.element(week()).toHaveValue('37');
		await expect.element(grace()).toHaveValue('14');
		expect(apiFetchWithSession).toHaveBeenCalledWith(settingsPath);
	});

	it('asks for whole numbers as text with a numeric keyboard, and says the bounds', async () => {
		await setup();

		await expect.element(week()).toHaveAttribute('type', 'text');
		await expect.element(week()).toHaveAttribute('inputmode', 'numeric');
		await expect.element(grace()).toHaveAttribute('type', 'text');
		await expect.element(grace()).toHaveAttribute('inputmode', 'numeric');
		await expect
			.element(
				testPage.getByText(
					'From 20 to 42. 37 is the usual answer: on call opens three weeks before the due date.'
				)
			)
			.toBeVisible();
		await expect
			.element(testPage.getByText('From 0 to 42. 14 covers a birth that comes after the due date.'))
			.toBeVisible();
		await expect
			.element(
				testPage.getByText(
					"One birth can start differently. Change it in the On call section of that birth's page."
				)
			)
			.toBeVisible();
	});

	it('draws a skeleton and no 37 or 14 before the read lands', async () => {
		apiFetchWithSession.mockReturnValueOnce(new Promise(() => {}));
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'On-call rule' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Loading the on-call rule')).toBeVisible();
		expect(week().query()).toBeNull();
		expect(grace().query()).toBeNull();
	});

	it('says who last changed the rule, in a time element', async () => {
		await setup();

		await expect.element(testPage.getByText('Last changed by Dee Whitlock,')).toBeVisible();
		const when = testPage.getByText('Last changed by Dee Whitlock,').getByRole('time');
		await expect.element(when).toHaveAttribute('datetime', '2026-11-12T15:02:11Z');
	});

	it('offers every change to the rule beside who last made one', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'See every change' }))
			.toHaveAttribute('href', '/practices/practice-1/activity');
	});

	it('says so when the rule has never changed', async () => {
		// eslint-disable-next-line unicorn/no-null -- the BFF sends null when the rule never changed
		await setup(jsonResponse({ ...settings, lastChanged: null }));

		await expect
			.element(testPage.getByText('Not changed since this Practice was set up.'))
			.toBeVisible();
	});

	it('says what saving does to the births this Practice is carrying, before the press', async () => {
		await setup();

		await expect
			.element(
				testPage.getByText(
					'Saving this moves the on-call days of every birth this Practice is carrying, births on call today included. A birth with its own start keeps that start.'
				)
			)
			.toBeVisible();
	});

	it('asks for no week under the rule that has none', async () => {
		await setup(jsonResponse({ ...settings, startRule: 'attachment_granted' }));

		expect(week().query()).toBeNull();
		await expect.element(grace()).toBeVisible();
	});

	it('draws the week field under the answer it belongs to, and drops it when another is chosen', async () => {
		await setup();
		await expect.element(week()).toBeVisible();

		await testPage.getByLabelText('From the day a Doula takes the birth').click();

		expect(week().query()).toBeNull();
	});

	it('sends the rule a person chose, and says what saved', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({
				startRule: 'gestational_week',
				startWeek: 39,
				graceDays: 10,
				lastChanged: { staffName: 'Dee Whitlock', at: new Date().toISOString() }
			})
		);

		await week().fill('39');
		await grace().fill('10');
		await save();

		expect(apiFetchWithSession).toHaveBeenLastCalledWith(settingsPath, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ startRule: 'gestational_week', startWeek: 39, graceDays: 10 })
		});
		await expect
			.element(savedStatus())
			.toHaveTextContent(
				'On-call rule saved. On call now opens at week 39 of pregnancy and runs 10 days past the due date, for every birth without its own start.'
			);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toHaveFocus();
		await expect.element(testPage.getByText('just now')).toBeVisible();
	});

	it('switches to the day a Doula takes the birth, and stops sending a week for it', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ ...settings, startRule: 'attachment_granted' })
		);

		await testPage.getByLabelText('From the day a Doula takes the birth').click();
		await save();

		expect(week().query()).toBeNull();
		await expect
			.element(savedStatus())
			.toHaveTextContent(/opens on the day a Doula takes the birth and runs 14 days/);
	});

	it('says nothing about a save until one lands, and drops it when the next one starts', async () => {
		await setup();
		await expect.element(week()).toHaveValue('37');
		expect(savedStatus().query()).toBeNull();

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(settings));
		await save();
		await expect.element(savedStatus()).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'The on-call rule could not be saved.',
					details: { graceDays: 'Enter a number of days from 0 to 42.' }
				},
				400
			)
		);
		await save();

		await expect.element(testPage.getByText('Enter a number of days from 0 to 42.').first()).toBeVisible();
		expect(savedStatus().query()).toBeNull();
	});

	it('says a refused week on its field and in the summary, with the typed values kept', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'The on-call rule could not be saved.',
					details: { startWeek: 'Enter a week from 20 to 42.' }
				},
				400
			)
		);

		await week().fill('50');
		await grace().fill('9');
		await save();

		await expect.element(testPage.getByText('Enter a week from 20 to 42.').first()).toBeVisible();
		await expect
			.element(testPage.getByRole('alert').filter({ hasText: 'Enter a week from 20 to 42.' }).first())
			.toBeVisible();
		await expect.element(week()).toHaveValue('50');
		await expect.element(grace()).toHaveValue('9');
		await expect
			.element(testPage.getByRole('link', { name: 'Enter a week from 20 to 42.' }))
			.toHaveAttribute('href', '#on-call-start-week');
	});

	it('says a rule it could not read, and offers no Save', async () => {
		await setup(jsonResponse('internal error', 500));

		await expect
			.element(testPage.getByRole('alert').filter({ hasText: 'internal error' }))
			.toBeVisible();
		expect(testPage.getByRole('button', { name: 'Save' }).query()).toBeNull();
		expect(week().query()).toBeNull();
	});
});

const effectPath = (week: number, grace = 14) =>
	`${settingsPath}/effect?startRule=gestational_week&startWeek=${week}&graceDays=${grace}`;

describe('the preview before saving', () => {
	it('names the births a typed week would take off call today, and the one that keeps its own start', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({
				stopToday: [
					{ engagementId: 'e1', clientName: 'Camille Boyd' },
					{ engagementId: 'e2', clientName: 'Morgan Grier' },
					{ engagementId: 'e3', clientName: 'Casey Estrada' }
				],
				startToday: [],
				keepOwnStart: [{ engagementId: 'e4', clientName: 'Quinn Estrada' }]
			})
		);

		await week().fill('38');

		await expect
			.element(
				testPage.getByText(
					'Saving this takes 3 births off call today: Camille Boyd, Morgan Grier and Casey Estrada.'
				)
			)
			.toBeVisible();
		await expect
			.element(
				testPage.getByText('1 birth keeps the start set on that birth: Quinn Estrada.')
			)
			.toBeVisible();
		expect(pathsFetched()).toContain(effectPath(38));
	});

	it('says when it puts a birth on call, and when it changes none', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({
				stopToday: [],
				startToday: [{ engagementId: 'e1', clientName: 'Tessa Moreau' }],
				keepOwnStart: []
			})
		);
		await week().fill('36');
		await expect
			.element(testPage.getByText('Saving this puts 1 birth on call today: Tessa Moreau.'))
			.toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ stopToday: [], startToday: [], keepOwnStart: [] })
		);
		await week().fill('35');
		await expect
			.element(testPage.getByText('Saving this changes no birth on call today.'))
			.toBeVisible();
	});

	it('says every birth with its own start keeps it, by name', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({
				stopToday: [],
				startToday: [],
				keepOwnStart: [
					{ engagementId: 'e1', clientName: 'Quinn Estrada' },
					{ engagementId: 'e2', clientName: 'Casey Estrada' }
				]
			})
		);

		await grace().fill('10');

		await expect
			.element(
				testPage.getByText(
					'2 births keep the start set on each of them: Quinn Estrada and Casey Estrada.'
				)
			)
			.toBeVisible();
	});

	it('asks nothing while a typed value could not be saved, and says nothing until Save', async () => {
		await setup();

		await week().fill('50');
		await new Promise((resolve) => setTimeout(resolve, 500));

		expect(pathsFetched()).toEqual([settingsPath]);
		expect(testPage.getByText(/Saving this (takes|changes|puts)/).query()).toBeNull();
	});

	it('asks nothing while the typed rule is the one the Practice holds', async () => {
		await setup();

		await week().fill('38');
		await week().fill('37');
		await new Promise((resolve) => setTimeout(resolve, 500));

		expect(pathsFetched().filter((path) => path.includes('/effect'))).toEqual([]);
	});

	it('says nothing when the preview cannot be read, and Save still works', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('internal error', 500));

		await week().fill('38');
		await new Promise((resolve) => setTimeout(resolve, 500));

		expect(testPage.getByText(/Saving this (takes|changes|puts)/).query()).toBeNull();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});
});

describe('the back link', () => {
	it('goes to Settings when the screen was not opened from the roster', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Settings' }))
			.toHaveAttribute('href', '/practices/practice-1/settings');
	});

	it('goes to the roster it came from, with its filters kept', async () => {
		await setup();
		navigated.callback?.({
			from: { url: new URL('https://example.test/practices/practice-1/on-call?days=tonight&mine=true') }
		});

		await expect
			.element(testPage.getByRole('link', { name: 'On call', exact: true }))
			.toHaveAttribute('href', '/practices/practice-1/on-call?days=tonight&mine=true');
	});

	it('goes to Settings when it came from anywhere else', async () => {
		await setup();
		navigated.callback?.({ from: { url: new URL('https://example.test/practices/practice-1/settings') } });

		await expect
			.element(testPage.getByRole('link', { name: 'Settings' }))
			.toHaveAttribute('href', '/practices/practice-1/settings');
	});
});

describe('a reader who is not an Owner or an Admin', () => {
	it('is told who can change the rule, offered the roster, and never asked for the rule', async () => {
		Object.assign(pageState, toPageState({ ...fixture, ...asDoula }));
		await render(Page, {});

		await expect
			.element(testPage.getByText('Only a Practice Owner or Admin can change the on-call rule.'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'See who is on call' }))
			.toHaveAttribute('href', '/practices/practice-1/on-call');
		expect(testPage.getByRole('button', { name: 'Save' }).query()).toBeNull();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});
});
