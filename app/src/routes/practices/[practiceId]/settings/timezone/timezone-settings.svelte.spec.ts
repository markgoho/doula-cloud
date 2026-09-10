/*
 * #1166's setting: the one zone a Practice's calendar-day math happens
 * in. Read and written by an Owner or an Admin, and stated by a person
 * rather than inherited -- the screen's job is to show the zone the
 * Practice actually holds, say what changing it does to work already
 * recorded, and send the choice.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

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

const timezonePath = '/api/practices/practice-1/timezone';

const control = () =>
	testPage.getByRole('combobox', { name: 'What timezone does this Practice work in?' });
const save = () => testPage.getByRole('button', { name: 'Save' }).click();

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('the Timezone screen', () => {
	it('opens on the zone the Practice holds', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ timezone: 'America/Denver' }));
		await render(Page, {});

		await expect.element(control()).toHaveValue('America/Denver');
		expect(apiFetchWithSession).toHaveBeenCalledWith(timezonePath);
	});

	it('shows a stored zone the seven-entry list does not carry, rather than rendering blank', async () => {
		// The failure this prevents is silent: a select whose value matches
		// no option renders empty, and the next save rewrites her zone to
		// whatever happened to be first.
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ timezone: 'America/Indiana/Indianapolis' })
		);
		await render(Page, {});

		await expect.element(control()).toHaveValue('America/Indiana/Indianapolis');
	});

	it('says what changing the zone does to Visits already recorded, before the press', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ timezone: 'America/New_York' }));
		await render(Page, {});

		await expect
			.element(testPage.getByText(/changes how Visits already recorded are typed/))
			.toBeVisible();
	});

	it('sends the chosen zone and confirms it saved', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ timezone: 'America/New_York' }));
		await render(Page, {});
		await expect.element(control()).toHaveValue('America/New_York');

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ timezone: 'America/Denver' }));
		await control().selectOptions('Mountain time (Denver)');
		await save();

		await expect.element(testPage.getByText('Saved.')).toBeVisible();
		const [path, init] = apiFetchWithSession.mock.calls[1];
		expect(path).toBe(timezonePath);
		expect(init.method).toBe('PUT');
		expect(JSON.parse(init.body)).toEqual({ timezone: 'America/Denver' });
	});

	it("reports the BFF's own refusal against the control", async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ timezone: 'America/New_York' }));
		await render(Page, {});
		await expect.element(control()).toHaveValue('America/New_York');

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('not an IANA zone name', 400));
		await control().selectOptions('Mountain time (Denver)');
		await save();

		await expect.element(testPage.getByText('not an IANA zone name')).toBeVisible();
	});

	it('refuses a save with nothing chosen, and sends nothing', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ timezone: '' }));
		await render(Page, {});
		await expect.element(control()).toBeVisible();

		await save();

		await expect
			.element(testPage.getByText('Choose the timezone this Practice works in'))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	it('reports a read that failed rather than showing an empty control with no explanation', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('internal error', 500));
		await render(Page, {});

		await expect.element(testPage.getByText('internal error')).toBeVisible();
	});
});
