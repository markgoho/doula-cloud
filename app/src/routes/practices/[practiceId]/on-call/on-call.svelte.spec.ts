import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { SvelteURL } from 'svelte/reactivity';
import { FROM_BEFORE_TODAY, TOO_MANY_DAYS } from '#lib/onCall.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context, and ListPage's <center-l max="none">
// only lifts the default measure cap through the registered element -- see
// the schedule route's own spec for the full reasoning.
import '#lib/styles/app.css';
import Page from './OnCallScreen.svelte';
import { inReaderZone } from '#lib/testZone.js';
import { practiceSession, toPageState } from '../../../routeFixture.js';
import { pageState } from '#testing/routePage.js';
import { afterNavigate, goto } from '$app/navigation';
import { asContractor, asDoula, data, fixture, roster } from './page.fixture.js';
import type { OnCallPageData } from './+page.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
Object.assign(pageState, toPageState(fixture));

if (!customElements.get('center-l')) registerLayoutPrimitives();

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	snapshot: vi.fn()
}));

const { practiceId } = fixture.params;
const onCallPath = `/practices/${practiceId}/on-call`;

inReaderZone('America/New_York');

// Reactive, because the screen reads the address through a derived.
const setAddress = (query: string) => {
	pageState.url = new SvelteURL(`https://example.test${onCallPath}${query}`);
};

const asVariant = (variant: { pageData?: unknown }) => {
	pageState.data = variant.pageData as Record<string, unknown>;
};

const arrived = () => (vi.mocked(afterNavigate).mock.calls.at(-1)![0] as () => void)();

beforeEach(async () => {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508).
	await testPage.viewport(1440, 900);
	vi.mocked(goto).mockReset();
	vi.mocked(afterNavigate).mockReset();
	Object.assign(pageState, toPageState(fixture));
	setAddress('');
});

afterEach(() => {
	vi.restoreAllMocks();
});

function setup(properties?: Parameters<typeof render<typeof Page>>[1]) {
	return render(Page, properties ?? { data });
}

const emptyData = (overrides: Partial<OnCallPageData> = {}): OnCallPageData => ({
	...data,
	roster: { ...roster, windows: [], noWindow: [], doulas: [] },
	...overrides
});

describe('the on-call roster', () => {
	it('says how many births are on call, how many need cover and who is on two at once, as the bar’s status', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(
				/Showing 6 births on call from Sat\sNov\s14 to Sun\sDec\s13, tonight first\. 3 need cover, 2 of them tonight\. 1 Doula on two births at once\./
			);
	});

	it('lists the births as a table of Client, who is on call, the days, the due date and the cover', async () => {
		await setup();

		const births = testPage.getByRole('table', { name: 'Births on call' });
		await expect.element(births).toBeVisible();
		for (const name of ['Client', 'On call', 'Days on call', 'Due', 'Cover'])
			await expect.element(births.getByRole('columnheader', { name, exact: true })).toBeVisible();
		await expect.element(births.getByRole('columnheader', { name: 'Due' })).toHaveAttribute('aria-sort', 'ascending');
	});

	it('opens each birth at its On call section', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Bea Okafor' }).first())
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/eng-2#on-call`);
	});

	it('draws the births on call tonight under their own heading, then the rest', async () => {
		await setup();

		await expect.element(testPage.getByText('Tonight, Sat Nov 14').first()).toBeVisible();
		await expect.element(testPage.getByText('Starting later').first()).toBeVisible();
		// A rowgroup header has no accessible role the locators reach: the table's own markup is read.
		const headings = [...document.querySelectorAll('th[scope="rowgroup"]')].map((heading) => heading.textContent?.trim());
		expect(headings).toEqual(['Tonight, Sat Nov 14', 'Starting later']);
	});

	it('says who holds tonight, and that nobody does', async () => {
		await setup();

		await expect.element(testPage.getByText('Tonight: Jo Park, covering for Ana Ruiz').first()).toBeVisible();
		await expect.element(testPage.getByText('Nobody tonight').first()).toBeVisible();
		await expect.element(testPage.getByText('Tonight: Persephone Vandermeulen-Achterberg, CD(DONA)').first()).toBeVisible();
	});

	it('flags a Doula on two births at once beside her name, with the other birth and the days they share', async () => {
		await setup();

		await expect.element(testPage.getByText('On 2 births at once').first()).toBeVisible();
		await expect.element(testPage.getByText('Also Bea Okafor, until Sun Nov 22').first()).toBeVisible();
	});

	it('says a birth needs cover with a badge, and says everything that is missing', async () => {
		await setup();

		await expect.element(testPage.getByText('Needs cover').first()).toBeVisible();
		await expect.element(testPage.getByText(/cannot be reached\s+Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am, nobody covering/).first()).toBeVisible();
		await expect.element(testPage.getByText(/Nobody on call\s+Sun Nov 29 to Thu Dec 3/).first()).toBeVisible();
		await expect.element(testPage.getByText('Covered', { exact: true }).first()).toBeVisible();
	});

	it('names the colleague covering a gap, even on a covered birth', async () => {
		await setup();

		await expect.element(testPage.getByText(/Marguerite Okonkwo-Fitzgerald covering/).first()).toBeVisible();
	});

	it('says a birth with no Doula yet, and one with its own start', async () => {
		await setup();

		await expect.element(testPage.getByText('No Doula yet').first()).toBeVisible();
		await expect.element(testPage.getByText('On call from week 38 for this birth, not week 37.').first()).toBeVisible();
		await expect.element(testPage.getByText('On call from the day a Doula took this birth, not from week 37.').first()).toBeVisible();
	});

	it('says every birth on call tonight has a Doula holding it, on her own line when she is the only one', async () => {
		await setup();

		await expect.element(testPage.getByText('Tonight: Henrietta Oyelaran-Whitcombe').first()).toBeVisible();
	});

	it('names the Order’s first option as the result line does, by whether today is in the days shown', async () => {
		await setup();
		await expect.element(testPage.getByLabelText('Order').getByRole('option').first()).toHaveTextContent('Tonight first');
	});

	it('names it "Soonest due first" once the days shown do not include today', async () => {
		setAddress('?from=2026-11-21&to=2026-11-28');
		await setup({ data: { ...data, view: { preset: 30, chosen: { from: '2026-11-21', to: '2026-11-28' }, isMine: false, doula: '', needsCover: false, order: '' }, filters: { from: '2026-11-21', to: '2026-11-28' } } });

		await expect.element(testPage.getByLabelText('Order').getByRole('option').first()).toHaveTextContent('Soonest due first');
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/, soonest due first\./);
	});

	it('says "You" where the Doula who cannot be reached is the reader, and her name to anyone else', async () => {
		await setup();

		await expect.element(testPage.getByText(/You cannot be reached\s+Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am, nobody covering/).first()).toBeVisible();
		const doulas = testPage.getByRole('table', { name: 'Doulas (2)' });
		await expect.element(doulas.getByText('You', { exact: true }).first()).toBeVisible();
	});

	it('says her name to a reader who is not her', async () => {
		const session = (pageState.data as { session: object }).session;
		pageState.data = { session: { ...session, staffId: 'staff-9' } };

		await setup();

		await expect
			.element(testPage.getByText(/Persephone Vandermeulen-Achterberg, CD\(DONA\) cannot be reached\s+Sat Nov 14/).first())
			.toBeVisible();
		expect(testPage.getByText(/^You cannot be reached/).elements()).toHaveLength(0);
	});

	it('says the days on call and the due date as days, each carrying its exact day', async () => {
		await setup();

		await expect.element(testPage.getByText('Until Wed Dec 9').first()).toBeVisible();
		// An attribute has no accessible query of its own: the exact day under the words (ADR-0022).
		const due = testPage.getByRole('table', { name: 'Births on call' }).element().querySelector('time[datetime="2026-11-25"]');
		expect(due?.textContent).toBe('Wed Nov 25');
	});

	it('states the rule in force and, to an Owner, the way to change it', async () => {
		await setup();

		await expect
			.element(testPage.getByText('On call opens at week 37 of pregnancy and runs 14 days past the due date.'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Change the on-call rule' }))
			.toHaveAttribute('href', `/practices/${practiceId}/settings/on-call`);
	});

	it('lists the births with no due date under their own heading, with the Doula on each', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'No days on call yet' })).toBeVisible();
		await expect
			.element(testPage.getByText("These births have no due date, so they have no on-call days. Add the due date on the birth's page."))
			.toBeVisible();
		await expect.element(testPage.getByRole('table', { name: 'Births with no days on call yet (2)' })).toBeVisible();
	});

	it('lists the Doulas as a table of who is on call for what and who cannot be reached', async () => {
		await setup();

		await expect.element(testPage.getByRole('table', { name: 'Doulas (2)' })).toBeVisible();
		for (const name of ['Doula', 'On call for', 'Cannot be reached'])
			await expect.element(testPage.getByRole('columnheader', { name, exact: true }).last()).toBeVisible();
		await expect.element(testPage.getByText('Bea Okafor, covering').first()).toBeVisible();
		await expect.element(testPage.getByText('Anne-Marie Ochieng-Whitfield and Bea Okafor, until Sun Nov 22').first()).toBeVisible();
	});

	it('says in the Doulas table whether a gap is covered, and by whom', async () => {
		await setup();
		const doulasTable = testPage.getByRole('table', { name: 'Doulas (2)' });
		await expect.element(doulasTable.getByText(/Anne-Marie Ochieng-Whitfield, nobody covering/).first()).toBeVisible();

		const covered = {
			...data,
			roster: {
				...roster,
				windows: roster.windows.map((row) => ({
					...row,
					gaps: row.gaps.map((gap) => (gap.id === 'gap-1' ? { ...gap, coveringStaffId: 'staff-2', coveringStaffName: 'Bo Ng' } : gap))
				})),
				doulas: roster.doulas.map((doula, index) =>
					index === 0 ? { ...doula, unreachable: doula.unreachable.map((gap) => ({ ...gap, isCovered: true })) } : doula
				)
			}
		};
		await setup({ data: covered });

		await expect.element(testPage.getByRole('table', { name: 'Doulas (2)' }).last().getByText(/Anne-Marie Ochieng-Whitfield, Bo Ng covering/).first()).toBeVisible();
	});

	it('draws no table of one Doula and no flag that cannot fire', async () => {
		await setup({ data: { ...data, roster: { ...roster, doulas: roster.doulas.slice(0, 1) } } });

		expect(testPage.getByRole('heading', { name: 'Doulas' }).elements()).toHaveLength(0);
	});

	it('draws no Doulas table under Mine or a Doula', async () => {
		await setup({ data: { ...data, view: { ...data.view, doula: 'staff-1' } } });

		expect(testPage.getByRole('heading', { name: 'Doulas' }).elements()).toHaveLength(0);
	});

	it('says once, under the result line, which zone the clock times are in when the Practice’s is another', async () => {
		const session = (pageState.data as { session: object }).session;
		pageState.data = { session: { ...session, timezone: 'Asia/Tokyo' } };

		await setup();

		await expect.element(testPage.getByText(/Times are in this Practice's time zone, Japan.*\./)).toBeVisible();
	});

	it('draws no zone note for a reader in the Practice’s zone', async () => {
		await setup();

		expect(testPage.getByText(/Times are in this Practice/).elements()).toHaveLength(0);
	});

	it('draws no zone note where no clock time is on the screen', async () => {
		const session = (pageState.data as { session: object }).session;
		pageState.data = { session: { ...session, timezone: 'Asia/Tokyo' } };

		await setup({
			data: {
				...data,
				roster: { ...roster, windows: roster.windows.map((row) => ({ ...row, gaps: [] })), doulas: roster.doulas.map((doula) => ({ ...doula, unreachable: [] })) }
			}
		});

		expect(testPage.getByText(/Times are in this Practice/).elements()).toHaveLength(0);
	});
});

describe('the roster’s filter bar', () => {
	it('chooses the days with the Schedule’s own control, and steps them', async () => {
		await setup();

		for (const name of ['Tonight', 'Next 7 days', 'Next 30 days', 'Choose days'])
			await expect.element(testPage.getByRole('radio', { name })).toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'Next 30 days' })).toBeChecked();
		await expect.element(testPage.getByRole('link', { name: /^Later, Mon Dec 14 to Tue Jan 12, 2027$/ })).toBeVisible();
	});

	it('draws no Earlier at today, and keeps its place so Later does not move', async () => {
		await setup();

		expect(testPage.getByRole('link', { name: /^Earlier/ }).elements()).toHaveLength(0);
		await expect.element(testPage.getByText('Earlier')).not.toBeVisible();
	});

	it('steps later without a full reload, keeping every other filter', async () => {
		setAddress('?cover=needed&days=7');
		await setup({ data: { ...data, view: { ...data.view, preset: 7, needsCover: true } } });

		await testPage.getByRole('link', { name: /^Later, / }).click();

		expect(goto).toHaveBeenCalledWith(`${onCallPath}?from=2026-11-21&to=2026-11-27&cover=needed`, undefined);
	});

	it('applies Tonight when it is chosen', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Tonight' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${onCallPath}?days=tonight`, { replaceState: false, reset: false }));
	});

	it('offers an Owner Needing cover and the Doula, and applies them as they are chosen', async () => {
		await setup();

		await expect.element(testPage.getByRole('group', { name: 'Show only' })).toBeVisible();
		await testPage.getByRole('checkbox', { name: 'Needing cover' }).click();
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${onCallPath}?cover=needed`, { replaceState: false, reset: false }));
		await testPage.getByLabelText('Doula', { exact: true }).selectOptions('staff-2');
		await expect.element(testPage.getByLabelText('Doula', { exact: true })).toHaveValue('staff-2');
	});

	it('offers the order, and applies it', async () => {
		await setup();

		await testPage.getByLabelText('Order').selectOptions('cover');

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`${onCallPath}?order=cover`, { replaceState: false, reset: false }));
	});

	it('offers an employed Doula Mine and the order, and neither Needing cover nor a Doula nor the rule’s link', async () => {
		asVariant(asDoula);
		await setup();

		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).toBeVisible();
		expect(testPage.getByRole('checkbox', { name: 'Needing cover' }).elements()).toHaveLength(0);
		expect(testPage.getByLabelText('Doula', { exact: true }).elements()).toHaveLength(0);
		expect(testPage.getByRole('link', { name: 'Change the on-call rule' }).elements()).toHaveLength(0);
		await expect
			.element(testPage.getByText('On call opens at week 37 of pregnancy and runs 14 days past the due date.'))
			.toBeVisible();
	});

	it('offers a contractor the order alone, over her own births', async () => {
		asVariant(asContractor);
		await setup({ data: asContractor.props!.data as OnCallPageData });

		expect(testPage.getByRole('checkbox').elements()).toHaveLength(0);
		await expect.element(testPage.getByLabelText('Order')).toBeVisible();
		await expect.element(testPage.getByRole('table', { name: 'Doulas (1)' })).toBeVisible();
		await expect.element(testPage.getByText('On call for no birth in these days: Bo Ng.')).toBeVisible();
		expect(testPage.getByText('On 2 births at once').elements()).toHaveLength(0);
	});

	it('draws a tag for each filter, named for what pressing it does, and Clear filters', async () => {
		setAddress('?days=tonight&cover=needed&doula=staff-2&order=cover');
		await setup({ data: { ...data, view: { preset: 1, isMine: false, doula: 'staff-2', needsCover: true, order: 'cover' } } });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Tonight' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Needing cover' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Doula that Doula' }).or(testPage.getByRole('link', { name: /^Remove filter: Doula / }))).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', onCallPath);
	});

	it('draws the chosen days as a tag, and the next seven days as another', async () => {
		setAddress('?from=2026-11-21&to=2026-12-05');
		await setup({ data: { ...data, view: { preset: 30, chosen: { from: '2026-11-21', to: '2026-12-05' }, isMine: false, doula: '', needsCover: false, order: '' } } });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Days Sat Nov 21 to Sat Dec 5' })).toBeVisible();
	});

	it('removes a filter in one action', async () => {
		setAddress('?days=7');
		await setup({ data: { ...data, view: { ...data.view, preset: 7 } } });

		await testPage.getByRole('link', { name: 'Remove filter: Next 7 days' }).click();

		expect(goto).toHaveBeenCalledWith(onCallPath, undefined);
	});

	it('opens From and To on the days the choice before showed, and says the longest range before it is hit', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('From', { exact: true })).toHaveValue('2026-11-14');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2026-12-13');
		await expect.element(testPage.getByText('Both days are included. Up to 92 days, starting today or later.')).toBeVisible();
		await expect.element(testPage.getByLabelText('From', { exact: true })).toHaveAttribute('min', '2026-11-14');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveAttribute('max', '2027-02-13');
		expect(goto).not.toHaveBeenCalled();
	});

	it('navigates to the days a reader typed, keeping them in the address', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('From', { exact: true }).fill('2026-11-20');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-12-20');
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(`${onCallPath}?from=2026-11-20&to=2026-12-20`, undefined);
	});

	it('answers a To before From in place, beside the fields, keeping the days typed', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('From', { exact: true }).fill('2026-11-25');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-11-20');
		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('Choose a To day that is the same as the From day or later.');
		await expect.element(testPage.getByLabelText('From', { exact: true })).toHaveValue('2026-11-25');
		expect(goto).not.toHaveBeenCalled();
	});

	it('words a range over 92 days in place, and a From before today', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('To', { exact: true }).fill('2027-06-01');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		await expect.element(testPage.getByRole('alert')).toHaveTextContent(TOO_MANY_DAYS);

		await testPage.getByLabelText('From', { exact: true }).fill('2026-11-01');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		await expect.element(testPage.getByRole('alert')).toHaveTextContent(FROM_BEFORE_TODAY);
		expect(goto).not.toHaveBeenCalled();
	});

	it('says the BFF’s refusal of the days in the same place, with the form kept and no births read', async () => {
		setAddress('?from=2026-11-14&to=2027-06-01');
		await setup({
			data: {
				view: { preset: 30, chosen: { from: '2026-11-14', to: '2027-06-01' }, isMine: false, doula: '', needsCover: false, order: '' },
				now: data.now,
				rangeErrors: { to: TOO_MANY_DAYS }
			}
		});

		await expect.element(testPage.getByRole('alert')).toHaveTextContent(TOO_MANY_DAYS);
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2027-06-01');
		expect(testPage.getByRole('table', { name: 'Births on call' }).elements()).toHaveLength(0);
	});

	it('puts the controls back in step with the address after a move', async () => {
		setAddress('?days=7&cover=needed&order=cover');
		await setup();

		arrived();

		await expect.element(testPage.getByRole('radio', { name: 'Next 7 days' })).toBeChecked();
		await expect.element(testPage.getByRole('checkbox', { name: 'Needing cover' })).toBeChecked();
		await expect.element(testPage.getByLabelText('Order')).toHaveValue('cover');
	});
});

describe('when there is nothing to show', () => {
	it('says an empty result once, and offers the way back to the days the screen opens on', async () => {
		setAddress('?days=7');
		await setup({ data: emptyData({ view: { ...data.view, preset: 7 }, filters: { from: '2026-11-14', to: '2026-11-20' } }) });

		await expect.element(testPage.getByText('No births on call from Sat Nov 14 to Fri Nov 20.').first()).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Show the next 30 days' })).toHaveAttribute('href', onCallPath);
	});

	it('says Needing cover found none, and offers every birth', async () => {
		setAddress('?cover=needed');
		await setup({ data: emptyData({ view: { ...data.view, needsCover: true } }) });

		await expect.element(testPage.getByText('No births need cover from Sat Nov 14 to Sun Dec 13.').first()).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Show every birth' })).toBeVisible();
	});

	it('draws no way on when nothing is narrowed', async () => {
		await setup({ data: emptyData() });

		await expect.element(testPage.getByText('No births on call from Sat Nov 14 to Sun Dec 13.').first()).toBeVisible();
		expect(testPage.getByRole('link', { name: /^Show / }).elements()).toHaveLength(0);
	});
});

describe('while the roster is read', () => {
	it('draws the bar and a skeleton, and says what it is loading', async () => {
		await setup({});

		await expect.element(testPage.getByRole('radio', { name: 'Next 30 days' })).toBeChecked();
		await expect.element(testPage.getByRole('status', { name: 'Loading who is on call' })).toBeVisible();
		expect(testPage.getByRole('table').elements()).toHaveLength(0);
	});

	it('says what happened when the read failed, keeps the bar, and tries again', async () => {
		setAddress('?days=7');
		const onRetry = vi.fn(async () => {});
		await setup({ hasReadFailed: true, onRetry });

		await expect.element(testPage.getByText('Who is on call did not load. Nothing you saved is lost.')).toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'Next 7 days' })).toBeChecked();
		await testPage.getByRole('button', { name: 'Try again' }).click();

		expect(onRetry).toHaveBeenCalledOnce();
	});
});

describe('how a Doula reads it', () => {
	it('opens an employed Doula’s link on Mine, with the tag to clear it', async () => {
		asVariant({ pageData: practiceSession(['doula']) });
		pageState.data = { session: { ...practiceSession(['doula']).session, hasOtherDoulas: true } };
		setAddress('?mine=true');

		await setup({ data: { ...data, view: { ...data.view, isMine: true }, filters: { from: '2026-11-14', to: '2026-12-13', staffId: 'staff-1' } } });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Mine' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/Showing your 6 births on call/);
	});
});
