import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
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
	await testPage.viewport(1440, 900);
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

	it('marks a doula who cannot be reached as away', async () => {
		await setup();

		await expect.element(testPage.getByText('Away').first()).toBeVisible();
	});

	it('navigates to the days a reader chose, keeping them in the address', async () => {
		await setup();

		await testPage.getByLabelText('From').fill('2026-11-01');
		await testPage.getByLabelText('To').fill('2026-11-30');
		await testPage.getByRole('button', { name: 'Show' }).click();

		expect(goto).toHaveBeenCalledWith(
			`/practices/${practiceId}/on-call?from=2026-11-01&to=2026-11-30`
		);
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
