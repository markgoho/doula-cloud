/*
 * #1093's setting: when a doula goes on call for a birth, and how long
 * she stays on call past the due date. Read and written by an Owner or
 * an Admin. The screen's job is to show the rule the Practice holds, say
 * what changing it does to births already under way, and send the
 * choice -- the bounds themselves belong to the BFF, which says them in
 * its own words.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, settings } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

const settingsPath = '/api/practices/practice-1/on-call-settings';

const week = () => testPage.getByLabelText('Week of pregnancy', { exact: true });
const grace = () => testPage.getByLabelText('Days on call after the due date', { exact: true });
const save = () => testPage.getByRole('button', { name: 'Save' }).click();

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(loadResponse: Response = jsonResponse(settings)) {
	apiFetchWithSession.mockResolvedValueOnce(loadResponse);
	await render(Page, {});
}

describe('the on-call rule screen', () => {
	it('opens on the rule the Practice holds', async () => {
		await setup();

		await expect.element(week()).toHaveValue(37);
		await expect.element(grace()).toHaveValue(14);
		expect(apiFetchWithSession).toHaveBeenCalledWith(settingsPath);
	});

	it('says what changing the rule does to births already under way, before the press', async () => {
		await setup();

		await expect
			.element(
				testPage.getByText(
					/moves every on-call window this practice is carrying, including births already under way/
				)
			)
			.toBeVisible();
	});

	it('asks for no week under the rule that has none', async () => {
		await setup(
			jsonResponse({ startRule: 'attachment_granted', startWeek: 37, graceDays: 14 })
		);

		await expect.element(testPage.getByLabelText('Week of pregnancy', { exact: true })).not.toBeInTheDocument();
		await expect.element(grace()).toBeVisible();
	});

	it('sends the rule a person chose, and says it saved', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ startRule: 'gestational_week', startWeek: 39, graceDays: 10 })
		);

		await week().fill('39');
		await grace().fill('10');
		await save();

		expect(apiFetchWithSession).toHaveBeenLastCalledWith(settingsPath, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ startRule: 'gestational_week', startWeek: 39, graceDays: 10 })
		});
		await expect.element(testPage.getByText('Saved.')).toBeVisible();
	});

	it('switches to the day a doula takes the birth, and stops sending a week for it', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ startRule: 'attachment_granted', startWeek: 37, graceDays: 14 })
		);

		await testPage.getByLabelText('From the day a doula takes the birth').click();
		await save();

		await expect.element(testPage.getByLabelText('Week of pregnancy', { exact: true })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Saved.')).toBeVisible();
	});

	it('puts the BFF’s own refusal on the control it names', async () => {
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

		await week().fill('12');
		await save();

		await expect.element(testPage.getByText('Enter a week from 20 to 42.').first()).toBeVisible();
	});

	it('says a rule it could not read, rather than showing a rule nobody stated', async () => {
		await setup(jsonResponse('internal error', 500));

		await expect.element(testPage.getByText('internal error').first()).toBeVisible();
	});
});
