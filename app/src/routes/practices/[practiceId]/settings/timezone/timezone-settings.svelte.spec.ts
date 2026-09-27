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
import { asDoula, fixture } from './page.fixture.js';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See FormPage.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

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

async function setup(loadResponse: Response) {
	apiFetchWithSession.mockResolvedValueOnce(loadResponse);
	await render(Page, {});
}

describe('the Timezone screen', () => {
	it('opens on the zone the Practice holds', async () => {
		await setup(jsonResponse({ timezone: 'America/Denver' }));

		await expect.element(control()).toHaveValue('America/Denver');
		expect(apiFetchWithSession).toHaveBeenCalledWith(timezonePath);
	});

	it('shows a stored zone the seven-entry list does not carry, rather than rendering blank', async () => {
		// The failure this prevents is silent: a select whose value matches
		// no option renders empty, and the next save rewrites her zone to
		// whatever happened to be first.
		await setup(jsonResponse({ timezone: 'America/Indiana/Indianapolis' }));

		await expect.element(control()).toHaveValue('America/Indiana/Indianapolis');
	});

	it('says what changing the zone does to Visits already recorded, before the press', async () => {
		await setup(jsonResponse({ timezone: 'America/New_York' }));

		await expect
			.element(testPage.getByText(/changes how Visits already recorded are typed/))
			.toBeVisible();
	});

	it('sends the chosen zone and confirms it saved', async () => {
		await setup(jsonResponse({ timezone: 'America/New_York' }));
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

	it("shows the BFF's field-level sentence, not its caller-facing summary", async () => {
		// docs/api-design.md section 7 rule 4: `details` is the sentence
		// written for the person and `message` is the one written for the
		// caller. Reading `message` here would put `timezone "X" is not an
		// IANA zone name` beside her control.
		await setup(jsonResponse({ timezone: 'America/New_York' }));
		await expect.element(control()).toHaveValue('America/New_York');

		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'timezone "Nowhere/Atlantis" is not an IANA zone name',
					details: { timezone: 'Enter a timezone from the IANA database, such as America/New_York' }
				},
				400
			)
		);
		await control().selectOptions('Mountain time (Denver)');
		await save();

		// In the summary at the top, as a link to the control it is about.
		await expect
			.element(
				testPage.getByRole('link', {
					name: 'Enter a timezone from the IANA database, such as America/New_York'
				})
			)
			.toBeVisible();
		await expect
			.element(testPage.getByText('timezone "Nowhere/Atlantis" is not an IANA zone name'))
			.not.toBeInTheDocument();
	});

	it('refuses a save with nothing chosen, and sends nothing', async () => {
		await setup(jsonResponse({ timezone: '' }));
		await expect.element(control()).toBeVisible();

		await save();

		await expect
			.element(testPage.getByRole('link', { name: 'Choose the timezone this Practice works in' }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	it('reports a read that failed rather than showing an empty control with no explanation', async () => {
		await setup(jsonResponse('internal error', 500));

		await expect.element(testPage.getByText('internal error')).toBeVisible();
	});

	it('offers an Owner the editable field and a Save button', async () => {
		await setup(jsonResponse({ timezone: 'America/Denver' }));

		await expect.element(control()).toHaveValue('America/Denver');
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});
});

/*
 * #1441: every Staff reads the zone (#1280 widened the GET so
 * `InvoiceSection` could), but only an Owner or Admin can state it. The
 * screen shows anyone else the zone as a fact, with no control that
 * exists only to be refused.
 */
describe('the Timezone screen, as a Doula', () => {
	beforeEach(() => {
		Object.assign(pageState, toPageState({ ...fixture, ...asDoula }));
		return () => {
			Object.assign(pageState, toPageState(fixture));
		};
	});

	it('shows the zone the Practice holds, read-only, with no Save button and no error', async () => {
		await setup(jsonResponse({ timezone: 'America/Denver' }));

		await expect.element(testPage.getByText('Mountain time (Denver)')).toBeVisible();
		await expect
			.element(testPage.getByText('Only a Practice Owner or Admin can change the timezone.'))
			.toBeVisible();
		await expect.element(control()).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('alert')).not.toBeInTheDocument();
	});

	it('says the zone is loading, rather than showing an empty row, until the read answers', async () => {
		apiFetchWithSession.mockReturnValueOnce(new Promise(() => {}));
		await render(Page, {});

		await expect
			.element(testPage.getByRole('status', { name: 'Loading the timezone' }))
			.toBeVisible();
	});

	it('shows a stored zone outside the seven-entry list by its own name', async () => {
		await setup(jsonResponse({ timezone: 'America/Indiana/Indianapolis' }));

		await expect.element(testPage.getByText('America/Indiana/Indianapolis')).toBeVisible();
	});
});
