import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { defaultRange } from '#lib/onCall.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { addDays } from '#lib/visitSchedule.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context, and ListPage's <center-l max="none">
// only lifts the default measure cap through the registered element -- see
// the schedule route's own spec for the full reasoning.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';
import type { OnCallPageData } from './+page.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>,
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

if (!customElements.get('center-l')) registerLayoutPrimitives();

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const { practiceId } = fixture.params;

beforeEach(() => {
	goto.mockReset();
});

// `session` merges in from practices/[practiceId]/+layout.ts (#835); this
// route reads it only through its own `load`, but the generated `data`
// prop type requires it.
const sessionStub = {
	practiceId,
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner'],
	isContractor: false,
};

async function setup(pageData: OnCallPageData = data) {
	// Wide enough for DataTable's <table> rather than the <dl> record
	// view its content floor stacks into below 46rem (#508) -- the same
	// call the schedule list's own spec makes, for the same reason. The
	// record view is swept at 320px by the continuum check instead.
	// Above this route's own table floor (#1945, its +page.svelte), so the <table> renders.
	await testPage.viewport(3200, 900);
	return render(Page, {
		params: fixture.params,
		data: { ...pageData, session: sessionStub },
	});
}

describe('the on-call roster', () => {
	it('says how many births are on call, and how many need cover', async () => {
		await setup();

		await expect
			.element(testPage.getByText(/births on call, \d+ needing cover\./))
			.toBeVisible();
	});

	it('names the doula on call for a birth, and the days she carries when she is narrowed', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: /Bo Ng/ }).first())
			.toBeVisible();
	});

	it('shows an uncovered gap as a hole rather than hiding it', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: /nobody covering/ }).first())
			.toBeVisible();
	});

	it('names the colleague covering a gap where one is covering it', async () => {
		await setup();

		await expect
			.element(
				testPage.getByRole('cell', { name: /Fitzgerald covering/ }).first()
			)
			.toBeVisible();
	});

	it('says which days nobody is on call for', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: /Nobody on call/ }).first())
			.toBeVisible();
	});

	it('says plainly why a birth has no window, and never guesses a date', async () => {
		await setup();

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'No due date recorded, so there is no window to work out.',
				})
			)
			.toBeVisible();
		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'There is no on-call window for this birth.',
				})
			)
			.toBeVisible();
	});

	it('names every doula who is on more than one birth in these days', async () => {
		await setup();

		await expect
			.element(testPage.getByText(/on more than one birth in these days\./))
			.toBeVisible();
	});

	// #2031: a roster is a list of rows, so the Doulas are a table.
	it('lists the doulas as a table of name, births and availability', async () => {
		await setup();

		for (const name of ['Doula', 'Births in these days', 'Available'])
			await expect.element(testPage.getByRole('columnheader', { name, exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: '1 birth', exact: true }).first()).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: '3 births', exact: true }).first()).toBeVisible();
	});

	it('marks a doula who cannot be reached as away', async () => {
		await setup();

		await expect.element(testPage.getByText('Away').first()).toBeVisible();
	});

	it('navigates to the days a reader chose, keeping them in the address', async () => {
		await setup();

		// Said relative to today, as the schedule's own spec does:
		// `rosterHref` omits a day that equals the default range, so a
		// literal pair would assert a different URL on the day it matched.
		const { from: today, to: defaultTo } = defaultRange();
		const chosenFrom = addDays(today, 5);
		const chosenTo = addDays(defaultTo, 10);

		await testPage.getByLabelText('From').fill(chosenFrom);
		await testPage.getByLabelText('To').fill(chosenTo);
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(
			`/practices/${practiceId}/on-call?from=${chosenFrom}&to=${chosenTo}`
		);
	});

	it('says the result as the filter bar’s own status line', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/births on call, \d+ needing cover\./);
	});

	it('says so plainly when no birth is on call in the days chosen', async () => {
		await setup({
			roster: {
				from: '2027-01-01',
				to: '2027-01-01',
				windows: [],
				noWindow: [],
				doulas: [],
			},
			range: { from: '2027-01-01', to: '2027-01-01' },
		});

		await expect
			.element(testPage.getByText('No births are on call in these days.'))
			.toBeVisible();
	});
});
