import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { SvelteURL } from 'svelte/reactivity';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import type { ListReturn } from '#lib/listReturn.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './ScheduleScreen.svelte';
import { practiceSession, toPageState } from '../../../routeFixture.js';
import { data, fixture, schedulePage } from './page.fixture.js';
import type { SchedulePageData } from './+page.js';
import { pageState } from '#testing/routePage.js';
import { inReaderZone } from '#lib/testZone.js';
import { afterNavigate, beforeNavigate, goto, snapshot } from '$app/navigation';

/*
 * The screen's content and the `page` it reads both come from the route's
 * own fixture (#596), the same installation contracts.svelte.spec.ts uses.
 */
vi.mock('$app/state', () => import('#testing/routePage.js'));
Object.assign(pageState, toPageState(fixture));

if (!customElements.get('center-l')) registerLayoutPrimitives();

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	snapshot: vi.fn()
}));

const [earlierVisit, laterVisit, offVisit] = schedulePage.items;
const { practiceId } = fixture.params;
const schedulePath = `/practices/${practiceId}/schedule`;
const ownSession = fixture.pageData as { session: { staffId: string } };
// A move with no search: the history entry's state says so (#2870).
const NOSEARCH = { state: { scheduleSearch: '' } };

// Reactive, because the screen reads the address through a derived.
const setAddress = (query: string) => {
	pageState.url = new SvelteURL(`https://example.test${schedulePath}${query}`);
};

// A name as the start of a row link's name: the Client, then the Visit's day.
const startsWith = (name: string) => new RegExp(`^${name.replaceAll(/[$()*+.?[\\\]^{|}]/g, String.raw`\$&`)}, `);

beforeEach(async () => {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508).
	await testPage.viewport(1440, 900);
	// The fixture's days are the next 30 from September 7, so "today" is that day.
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2026-09-07T13:00:00Z'));
	apiFetchWithSession.mockReset();
	vi.mocked(goto).mockReset();
	Object.assign(pageState, toPageState(fixture));
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

const emptyData = (view = data.view, filters = data.filters): SchedulePageData => ({
	...data,
	view,
	filters,
	page: { items: [], hasMore: false }
});

const asDoula = () => {
	pageState.data = { session: { ...practiceSession(['doula']).session, hasOtherDoulas: true } };
};

const arrived = () => (vi.mocked(afterNavigate).mock.calls.at(-1)![0] as () => void)();

function setup(properties?: Parameters<typeof render<typeof Page>>[1]) {
	return render(Page, properties ?? { data });
}

// A cell's name as a screen reader hears it: Chromium puts a space either
// side of a visually hidden comma, so the words match with any spacing.
const heard = (text: string) => new RegExp(`^${text.split(/,? /).join(String.raw`\s*,?\s*`)}$`);

describe('the Practice-wide schedule (#263)', () => {
	it('lists every scheduled Visit at the Practice, with when, whose and who is covering', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: earlierVisit.staffName, exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: startsWith(laterVisit.clientName) })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: startsWith(earlierVisit.clientName) })).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: fixture.readyText })).toBeVisible();

		// The instant is carried underneath the words a person reads
		// (ADR-0022), so a screen reader, a hover and a copy-paste all get
		// the real one; `querySelector` because an attribute has no accessible
		// query of its own.
		const whenCell = testPage.getByRole('cell', { name: /^[A-Z][a-z]{2} Sep \d+\s*,\s*\d/ }).first();
		await expect.element(whenCell).toBeVisible();
		expect(whenCell.element().querySelector('time')).toHaveAttribute('datetime', earlierVisit.scheduledAt);
	});

	it('names each row as the way in to its Visit on the Engagement, told apart by its day (#2747)', async () => {
		await setup();

		const link = testPage.getByRole('link', { name: startsWith(earlierVisit.clientName) });
		await expect
			.element(link)
			.toHaveAttribute(
				'href',
				`/practices/${practiceId}/engagements/${earlierVisit.engagementId}#visit-${earlierVisit.visitId}`
			);
		await expect.element(link).toHaveAccessibleName(/^Anne-Marie Ochieng-Whitfield, [A-Z][a-z]{2} Sep \d+, \d+:\d\d[ap]m$/);
	});

	it('says the table’s purpose and what it holds, and states the count and order once', async () => {
		await setup();

		const table = testPage.getByRole('table', { name: 'Visits' });
		await expect.element(table).toBeVisible();
		await expect.element(table).toHaveAccessibleDescription(/^Showing 3 Visits from Mon\sSep\s7 to Tue\sOct\s6, soonest first\.$/);
		expect(testPage.getByText(/^Every Visit this Practice has scheduled/).elements()).toHaveLength(0);
	});

	it('says a Visit’s year when it is not this one', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: /^Mon Jan 4, 2027\s*,\s*\d+:\d\d[ap]m$/ })).toBeVisible();
	});
});

describe('each row says what kind of Visit it is (#2870)', () => {
	it('puts the type in its own column after the When, in the Engagement’s words', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Type' })).toBeVisible();
		const headings = testPage.getByRole('columnheader').elements().map((heading) => heading.textContent?.trim());
		expect(headings).toEqual(['Client', 'When', 'Type', 'Doula']);
		for (const label of ['Prenatal', 'Birth', 'Postpartum']) {
			await expect.element(testPage.getByRole('cell', { name: label, exact: true })).toBeVisible();
		}
	});
});

// The heading is drawn once in the table and once above the cards, both always in the document.
const headingCount = (text: string) => testPage.getByText(text, { exact: true }).elements().length;

describe('each day’s Visits under a heading (#2867)', () => {
	it('says each day, with its weekday, over that day’s Visits', async () => {
		await setup();

		expect(headingCount('Sat Sep 12')).toBe(2);
		expect(headingCount('Fri Sep 18')).toBe(2);
		expect(headingCount('Mon Jan 4, 2027')).toBe(2);
		await expect.element(testPage.getByRole('table', { name: 'Visits' }).getByRole('row').first()).toBeVisible();
	});

	it('says Today and Tomorrow from the instant the days were read at', async () => {
		await setup({ data: { ...data, now: new Date('2026-09-11T13:00:00Z') } });

		expect(headingCount('Tomorrow, Sat Sep 12')).toBe(2);
		expect(headingCount('Today, Sat Sep 12')).toBe(0);

		await setup({ data: { ...data, now: new Date('2026-09-12T13:00:00Z') } });

		expect(headingCount('Today, Sat Sep 12')).toBeGreaterThan(0);
	});

	it('says Today again from the new day when the tab comes back to the front', async () => {
		await setup();
		expect(headingCount('Today, Sat Sep 12')).toBe(0);

		vi.setSystemTime(new Date('2026-09-12T13:00:00Z'));
		document.dispatchEvent(new Event('visibilitychange'));

		await expect.poll(() => headingCount('Today, Sat Sep 12')).toBe(2);
	});

	it('keeps the heading it has while the tab is hidden', async () => {
		await setup();
		vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');

		vi.setSystemTime(new Date('2026-09-12T13:00:00Z'));
		document.dispatchEvent(new Event('visibilitychange'));

		expect(headingCount('Today, Sat Sep 12')).toBe(0);
		expect(headingCount('Sat Sep 12')).toBe(2);
	});
});

describe('a booked Visit whose Doula was taken off (#2596)', () => {
	it('reads "No longer on this work" after her name, in words, as a badge', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: heard(`${offVisit.staffName}, No longer on this work`) }))
			.toBeVisible();
		// Once in the table and once in the record view, both always in the document.
		expect(testPage.getByText('No longer on this work').elements()).toHaveLength(2);
	});
});

describe('the result line, the empty state and the way back', () => {
	it('announces what the narrowed view shows as the filter bar’s status line', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent('Showing 3 Visits from Mon Sep 7 to Tue Oct 6, soonest first.');
	});

	it('says an empty result once on screen, for sight only, and once to a screen reader, with no way back on the default days', async () => {
		await setup({ data: emptyData() });

		const line = testPage.getByRole('status');
		await expect.element(line).toHaveClass('visually-hidden');
		await expect.element(line).toHaveTextContent('No Visits from Mon Sep 7 to Tue Oct 6.');
		await expect.element(testPage.getByText('No Visits from Mon Sep 7 to Tue Oct 6.').last()).toBeVisible();
		expect(testPage.getByRole('link', { name: 'Show the next 30 days' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('table').elements()).toHaveLength(0);
	});

	it('offers the next 30 days first, keeping Mine, from an empty Mine view of other days', async () => {
		setAddress('?days=7&mine=true');
		await setup({ data: emptyData({ preset: 7, isMine: true, doula: '', needsDoula: false, order: '' }, { from: '2026-09-07', to: '2026-09-13', timeZone: 'America/New_York', staffId: 'staff-1' }) });

		await expect.element(testPage.getByText('You have no Visits from Mon Sep 7 to Sun Sep 13.').first()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Show the next 30 days' }))
			.toHaveAttribute('href', `${schedulePath}?mine=true`);
	});

	it('offers the next 30 days from an empty view of other days with nobody narrowed', async () => {
		setAddress('?days=7');
		await setup({ data: emptyData({ preset: 7, isMine: false, doula: '', needsDoula: false, order: '' }, { from: '2026-09-07', to: '2026-09-13', timeZone: 'America/New_York' }) });

		await expect
			.element(testPage.getByRole('link', { name: 'Show the next 30 days' }))
			.toHaveAttribute('href', schedulePath);
	});

	it('says Visits for a picked Doula, and offers every Doula\'s on the days it was', async () => {
		setAddress('?doula=staff-2');
		await setup({ data: emptyData({ preset: 30, isMine: false, doula: 'staff-2', needsDoula: false, order: '' }) });
		await expect.element(testPage.getByText('No Visits for Bo Ng from Mon Sep 7 to Tue Oct 6.').first()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Show every Doula's Visits" }))
			.toHaveAttribute('href', schedulePath);
	});

	it('says a one-day range as "on" that day, in the line and the tag', async () => {
		setAddress('?from=2026-09-14&to=2026-09-14');
		await setup({
			data: emptyData(
				{ preset: 30, chosen: { from: '2026-09-14', to: '2026-09-14' }, isMine: false, doula: '', needsDoula: false, order: '' },
				{ from: '2026-09-14', to: '2026-09-14', timeZone: 'America/New_York' }
			)
		});

		await expect.element(testPage.getByRole('status')).toHaveTextContent('No Visits on Mon Sep 14.');
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Days Mon Sep 14' })).toBeVisible();
	});

	it('names the Doula it does not know, by her place', async () => {
		setAddress('?doula=staff-9');
		await setup({ data: { ...emptyData({ preset: 30, isMine: false, doula: 'staff-9', needsDoula: false, order: '' }), doulas: [] } });

		await expect.element(testPage.getByText('No Visits for that Doula from Mon Sep 7 to Tue Oct 6.').first()).toBeVisible();
	});

	it('counts a Mine result as the reader’s own, even when she is also the picked Doula', async () => {
		setAddress(`?mine=true&doula=${ownSession.session.staffId}`);
		await setup({ data: { ...data, view: { preset: 30, isMine: true, doula: ownSession.session.staffId, needsDoula: false, order: '' } } });

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent('Showing your 3 Visits from Mon Sep 7 to Tue Oct 6, soonest first.');
	});
});

describe('the narrowing', () => {
	it('offers the days as radios, Mine and a Doula, and no From or To until days are chosen', async () => {
		await setup();

		await expect.element(testPage.getByRole('radio', { name: 'Next 30 days' })).toBeChecked();
		await expect.element(testPage.getByRole('radio', { name: 'Next 7 days' })).toBeVisible();
		// What the form sends with no script running: the values the address reads.
		await expect.element(testPage.getByRole('radio', { name: 'Next 7 days' })).toHaveAttribute('value', '7');
		await expect.element(testPage.getByRole('radio', { name: 'Choose days' })).toHaveAttribute('value', 'choose');
		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).toBeVisible();
		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toHaveValue('');
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
		expect(testPage.getByLabelText('From').elements()).toHaveLength(0);
	});

	it('offers any Doula first, as the Clients list does', async () => {
		await setup();

		await expect.element(testPage.getByRole('option', { name: 'Any Doula' })).toBeInTheDocument();
	});

	it('opens From and To on the days the choice before showed, and says both days are included', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('From')).toHaveValue('2026-09-07');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2026-10-06');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveAccessibleDescription('Both days are included.');
	});

	it('keeps what was typed when the radio is chosen again', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('To', { exact: true }).fill('2026-09-20');

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2026-09-20');
	});

	it('starts From and To on the next 7 days when that was the choice before', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2026-09-13');
	});

	it('goes to the address for the next 7 days, Mine and a Doula, so it survives a reload and can be shared', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await testPage.getByRole('checkbox', { name: 'Mine' }).click();
		await testPage.getByRole('combobox', { name: 'Doula' }).selectOptions(earlierVisit.staffName);
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?days=7&mine=true&doula=${earlierVisit.staffId}`, NOSEARCH);
	});

	it('goes to the bare path for the default days and nothing else', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(schedulePath, NOSEARCH);
	});

	it('goes to the chosen days', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('From').fill('2026-09-10');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-09-19');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?from=2026-09-10&to=2026-09-19`, NOSEARCH);
	});

	it('refuses a To day before the From day beside To, moves focus there, and goes nowhere', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('To', { exact: true }).fill('2026-09-01');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expect
			.element(testPage.getByLabelText('To', { exact: true }))
			.toHaveAccessibleDescription('Both days are included. Choose a To day that is the same as the From day or later.');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveFocus();
		expect(goto).not.toHaveBeenCalled();
	});

	it('asks for an empty From day, and puts focus there', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('From').fill('');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expect.element(testPage.getByLabelText('From')).toHaveAccessibleDescription('Both days are included. Enter a From day');
		await expect.element(testPage.getByLabelText('From')).toHaveFocus();
	});

	it('draws a cold address whose To is before its From in place, with Choose days on, and no list', async () => {
		setAddress('?from=2026-09-19&to=2026-09-12');
		await setup({
			data: {
				...data,
				page: { items: [], hasMore: false },
				view: { preset: 30, chosen: { from: '2026-09-19', to: '2026-09-12' }, isMine: false, doula: '', needsDoula: false, order: '' },
				filters: { from: '', to: '', timeZone: 'America/New_York' },
				rangeErrors: { to: 'Choose a To day that is the same as the From day or later.' }
			}
		});

		await expect.element(testPage.getByRole('radio', { name: 'Choose days' })).toBeChecked();
		await expect
			.element(testPage.getByLabelText('To', { exact: true }))
			.toHaveAccessibleDescription('Both days are included. Choose a To day that is the same as the From day or later.');
		expect(testPage.getByRole('table').elements()).toHaveLength(0);
		expect(testPage.getByRole('alert').elements()).toHaveLength(1);
	});

	it('offers a Doula who is not an Owner or Admin Mine and no Doula picker', async () => {
		pageState.data = { session: { ...practiceSession(['doula']).session, hasOtherDoulas: true } };
		await setup({ data: { ...data, doulas: [] } });

		expect(testPage.getByRole('combobox', { name: 'Doula' }).elements()).toHaveLength(0);
		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).toBeVisible();
	});

	it('offers Mine to nobody at a Practice with one Doula, and an Admin who is no Doula never', async () => {
		pageState.data = { session: { ...practiceSession(['admin']).session } };
		await setup();

		expect(testPage.getByRole('checkbox', { name: 'Mine' }).elements()).toHaveLength(0);
		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toBeVisible();
	});

	it('holds the Doula picker’s place until the Doulas land', async () => {
		await setup({});

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toBeDisabled();
		await expect.element(testPage.getByRole('status', { name: 'Loading your schedule' })).toBeVisible();
	});

	it('draws no Doula tag and does not say "Any Doula" while the Doulas load for a shared link', async () => {
		setAddress(`?doula=${laterVisit.staffId}`);
		await setup({});

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toBeDisabled();
		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toHaveValue(laterVisit.staffId);
		await expect.element(testPage.getByRole('option', { name: 'Any Doula' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: /Remove filter: Doula/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it('leaves the picker waiting when the roster could not be read, and still draws the list', async () => {
		await setup({ data: { ...data, doulas: [], isRosterUnavailable: true } });

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toBeDisabled();
		await expect.element(testPage.getByRole('table', { name: 'Visits' })).toBeVisible();
	});

	it('keeps the Doula a shared link names once the Doulas land', async () => {
		setAddress(`?doula=${laterVisit.staffId}`);
		const screen = await setup({});

		await screen.rerender({
			data: { ...data, view: { preset: 30, isMine: false, doula: laterVisit.staffId, needsDoula: false, order: '' } }
		});

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toHaveValue(laterVisit.staffId);
	});

	it('does not hold the Doula pending or count her tag once the read has failed', async () => {
		setAddress(`?doula=${laterVisit.staffId}`);
		await setup({ hasReadFailed: true });

		expect(testPage.getByRole('option', { name: 'Any Doula' }).elements()).toHaveLength(1);
		await expect.element(testPage.getByRole('alert')).toBeVisible();
	});

	it('drops the From and To fields when the next 7 days is chosen over a range in the address', async () => {
		setAddress('?from=2026-09-10&to=2026-09-12');
		await setup({ data: { ...data, view: { preset: 30, chosen: { from: '2026-09-10', to: '2026-09-12' }, isMine: false, doula: '', needsDoula: false, order: '' } } });
		await expect.element(testPage.getByLabelText('From')).toBeVisible();

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();

		expect(testPage.getByLabelText('From').elements()).toHaveLength(0);
		expect(testPage.getByLabelText('To', { exact: true }).elements()).toHaveLength(0);
	});

	describe('for a reader who is not offered the Doula picker', () => {
		it('reads her own id in the address as Mine', async () => {
			asDoula();
			setAddress('?doula=staff-1');
			await setup({ data: { ...data, doulas: [], view: { preset: 30, isMine: true, doula: '', needsDoula: false, order: '' } } });

			await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).toBeChecked();
			await expect.element(testPage.getByRole('link', { name: 'Remove filter: Mine' })).toHaveAttribute(
				'href',
				schedulePath
			);
			expect(testPage.getByRole('link', { name: /Remove filter: Doula/ }).elements()).toHaveLength(0);
		});

		it('ignores any other Doula in the address', async () => {
			asDoula();
			setAddress(`?doula=${laterVisit.staffId}`);
			await setup({ data: { ...data, doulas: [] } });

			expect(testPage.getByRole('link', { name: /^Remove filter/ }).elements()).toHaveLength(0);
			expect(testPage.getByText('Clear filters').elements()).toHaveLength(0);
		});
	});
});

describe('the applied tags', () => {
	it('draws no tag for the default days, and none of Clear filters', async () => {
		await setup();

		expect(testPage.getByRole('list', { name: 'Applied filters' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('link', { name: 'Clear filters' }).elements()).toHaveLength(0);
	});

	it('draws a removable tag for each narrowing, each a link to the list minus that one', async () => {
		setAddress(`?days=7&mine=true&doula=${laterVisit.staffId}`);
		await setup();

		const mine = testPage.getByRole('link', { name: 'Remove filter: Mine' });
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Next 7 days' })).toHaveAttribute(
			'href',
			`${schedulePath}?mine=true&doula=${laterVisit.staffId}`
		);
		await expect.element(mine).toHaveAttribute('href', `${schedulePath}?days=7&doula=${laterVisit.staffId}`);
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Doula Bo Ng' })).toHaveAttribute(
			'href',
			`${schedulePath}?days=7&mine=true`
		);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', schedulePath);
	});

	it('says chosen days with their weekdays, and removes them back to the default days', async () => {
		setAddress('?from=2026-09-07&to=2026-09-19&mine=true');
		await setup();

		const tag = testPage.getByRole('link', { name: 'Remove filter: Days Mon Sep 7 to Sat Sep 19' });
		await expect.element(tag).toHaveTextContent('Days: Mon Sep 7 to Sat Sep 19');
		await expect.element(tag).toHaveAttribute('href', `${schedulePath}?mine=true`);
	});

	it('draws no tag for chosen days that are not a range, and still offers Clear filters', async () => {
		setAddress('?from=2026-09-19&to=2026-09-12');
		await setup();

		expect(testPage.getByRole('link', { name: /^Remove filter/ }).elements()).toHaveLength(0);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toBeVisible();
	});

	it('moves to the list minus the tag by client-side navigation', async () => {
		setAddress('?days=7');
		await setup();

		await testPage.getByRole('link', { name: 'Remove filter: Next 7 days' }).click();

		expect(goto).toHaveBeenCalledWith(schedulePath, NOSEARCH);
	});
});

describe('paging', () => {
	const nextPage = {
		items: [
			{
				visitId: 'visit-9',
				engagementId: 'eng-9',
				clientName: 'Cleo',
				staffId: 'staff-2',
				staffName: 'Bo Ng',
				scheduledAt: '2026-09-25T09:00:00Z',
				isOffEngagement: false,
				type: 'prenatal'
			}
		],
		hasMore: false
	};
	const morePage = { ...schedulePage, hasMore: true, nextCursor: 'cursor-1' };

	it('says the first page is the first of more, and appends the next page, continuing the same narrowing', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse(nextPage));
		await setup({ data: { ...data, page: morePage, filters: { ...data.filters, staffId: 'staff-2' } } });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing the first 3 Visits');

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: /^Cleo, / })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: /^Anne-Marie/ })).toBeVisible();
		const url = new URL(apiFetchWithSession.mock.calls[0][0], 'https://example.test');
		expect(url.pathname).toBe(`/api/practices/${practiceId}/visits`);
		expect(url.searchParams.get('cursor')).toBe('cursor-1');
		expect(url.searchParams.get('staffId')).toBe('staff-2');
	});

	it('says how many there are in all while more exist, and the plain line once Load more has shown every one (#2867)', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse(nextPage));
		await setup({ data: { ...data, page: { ...morePage, visitCount: 4 } } });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing the first 3 of 4 Visits from');

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 4 Visits from');
	});

	it('still says the first of more when the page carries no total (#2867)', async () => {
		await setup({ data: { ...data, page: morePage } });
		const line = testPage.getByRole('status');
		await expect.element(line).toHaveTextContent('Showing the first 3 Visits from');
		await expect.element(line).not.toHaveTextContent(' of ');
	});

	it('says a failed next page through the table, keeps the rows, and shows no raw server text', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));
		await setup({ data: { ...data, page: morePage } });

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('More Visits did not load. Nothing you saved is lost. Try again.');
		await expect.element(testPage.getByRole('button', { name: 'Try again' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
		expect(testPage.getByText('invalid cursor').elements()).toHaveLength(0);
		await expect.element(testPage.getByRole('link', { name: /^Anne-Marie/ })).toBeVisible();
	});

	it('replaces the rows when the narrowing changes, rather than keeping the previous one’s', async () => {
		const screen = await setup();

		await screen.rerender({ data: { ...data, page: { items: [laterVisit], hasMore: false } } });

		await expect.element(testPage.getByRole('link', { name: startsWith(laterVisit.clientName) })).toBeVisible();
		expect(testPage.getByRole('link', { name: /^Anne-Marie/ }).elements()).toHaveLength(0);
	});
});

describe('when the read fails (T8)', () => {
	it('says what happened in place of the list, keeps the bar and its narrowing, and tries again', async () => {
		setAddress('?days=7');
		const onRetry = vi.fn(async () => {});
		await setup({ hasReadFailed: true, onRetry });

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('Your schedule did not load. Nothing you saved is lost.');
		await expect.element(testPage.getByRole('radio', { name: 'Next 7 days' })).toBeChecked();
		expect(testPage.getByRole('table').elements()).toHaveLength(0);

		await testPage.getByRole('button', { name: 'Try again' }).click();

		await vi.waitFor(() => expect(onRetry).toHaveBeenCalledOnce());
	});
});

// T3 (#2747): Back from an Engagement puts her on the same rows, where she was.
const kept = () =>
	vi.mocked(snapshot).mock.calls.at(-1)![0] as { capture: () => ListReturn; restore: (held: ListReturn) => void };

const leaveTo = (hash: string, type = 'link') => {
	for (const [leave] of vi.mocked(beforeNavigate).mock.calls) {
		leave({ type, to: { url: new URL(`https://example.test/practices/${practiceId}/engagements/eng-1${hash}`) } } as never);
	}
};

describe('coming back from an Engagement', () => {
	const morePage = { ...schedulePage, hasMore: true, nextCursor: 'cursor-1' };
	const nextPage = {
		items: [{ ...laterVisit, visitId: 'visit-9', clientName: 'Cleo' }],
		hasMore: false
	};

	it('keeps how many pages were open, how far down she was and the Visit she left for', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse(nextPage));
		await setup({ data: { ...data, page: morePage } });
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('link', { name: /^Cleo, / })).toBeVisible();

		leaveTo('#visit-visit-9');

		expect(kept().capture()).toEqual({ pages: 2, scrollY: window.scrollY, rowId: 'visit-9' });
	});

	it('keeps no row for a move that is not to a Visit', async () => {
		await setup();

		leaveTo('');

		expect(kept().capture().rowId).toBeUndefined();
	});

	it('reads as many pages again, scrolls back, and lands on the row she left from', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		apiFetchWithSession.mockResolvedValue(jsonResponse(nextPage));
		const screen = await setup({});
		kept().restore({ pages: 2, scrollY: 120, rowId: 'visit-9' });

		await screen.rerender({ data: { ...data, page: morePage } });

		const left = testPage.getByRole('link', { name: /^Cleo, / });
		await expect.element(left).toBeVisible();
		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
		await expect.element(left).toHaveFocus();
	});

	it('restores at once when its read has already landed, and stops reading when the pages run out', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [], hasMore: false }));
		await setup({ data: { ...data, page: morePage } });

		kept().restore({ pages: 3, scrollY: 80 });

		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 80));
		expect(apiFetchWithSession).toHaveBeenCalledOnce();
	});

	it('waits for a "Load more" already in flight and carries on to the pages she had open', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		const second = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(second.promise).mockResolvedValue(jsonResponse(nextPage));
		await setup({ data: { ...data, page: morePage } });
		await testPage.getByRole('button', { name: 'Load more' }).click();
		kept().restore({ pages: 3, scrollY: 120 });

		second.resolve(jsonResponse({ items: [{ ...laterVisit, visitId: 'visit-8', clientName: 'Dara' }], hasMore: true, nextCursor: 'cursor-2' }));

		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
		expect(apiFetchWithSession).toHaveBeenCalledTimes(2);
		await expect.element(testPage.getByRole('link', { name: /^Cleo, / })).toBeVisible();
	});

	it('waits for the read of the entry she came back to', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		const screen = await setup({ data, isReading: true });

		kept().restore({ pages: 1, scrollY: 120 });
		expect(scrollTo).not.toHaveBeenCalled();

		await screen.rerender({ isReading: false, data: { ...data } });

		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
	});

	it('drops the place it holds when the read fails, and when she moves before it lands', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		const screen = await setup({});
		kept().restore({ pages: 1, scrollY: 120 });
		await screen.rerender({ hasReadFailed: true });
		kept().restore({ pages: 1, scrollY: 120 });
		await screen.rerender({ hasReadFailed: false, data });
		leaveTo('', 'goto');
		await screen.rerender({ data: { ...data, page: morePage } });

		expect(scrollTo).not.toHaveBeenCalled();
	});

	it('stops short of scrolling when she moves while its pages load', async () => {
		const second = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValue(second.promise);
		const scrollTo = vi.spyOn(globalThis, 'scrollTo');
		scrollTo.mockImplementation(() => {});
		await setup({ data: { ...data, page: morePage } });
		kept().restore({ pages: 2, scrollY: 120 });
		await vi.waitFor(() => expect(apiFetchWithSession).toHaveBeenCalled());
		leaveTo('', 'goto');

		second.resolve(jsonResponse(nextPage));

		await expect.element(testPage.getByRole('link', { name: /^Cleo, / })).toBeVisible();
		expect(scrollTo).not.toHaveBeenCalled();
	});

	it('lands on the result line, not the top, after a change of narrowing once its list has loaded', async () => {
		setAddress('');
		const screen = await setup();
		arrived();
		await expect.element(testPage.getByRole('status')).not.toHaveFocus();

		pageState.url.searchParams.set('days', '7');
		arrived();
		await screen.rerender({ data: { ...data, view: { ...data.view, preset: 7 } } });

		await expect.element(testPage.getByRole('status')).toHaveFocus();
	});
});

const clockEdges = (isYearDay: boolean) =>
	testPage
		.getByRole('cell', { name: /[ap]m$/ })
		.elements()
		.filter((cell) => /\d{4},/.test(cell.textContent!) === isYearDay)
		.map((cell) => Math.round(cell.querySelector('.clock')!.getBoundingClientRect().right));

describe('the Visits’ days and clocks', () => {
	it('sets the day and the clock in tracks of their own, so every clock ends on one edge', async () => {
		await setup();

		expect(new Set(clockEdges(false)).size).toBe(1);
		// A day that carries a year runs past the day's track, the one exception.
		expect(clockEdges(true).length).toBeGreaterThan(0);
	});
});

const ZONE_NOTE = "Times are in this Practice's time zone, Eastern Time.";
const whenOf = (visit: { scheduledAt: string }) =>
	testPage.getByRole('cell').elements().find((cell) => cell.querySelector('time')?.getAttribute('datetime') === visit.scheduledAt)!.textContent;

describe('the Practice’s own days and times, whatever zone the reader is in (#2867)', () => {
	describe('a reader in Tokyo', () => {
		inReaderZone('Asia/Tokyo');

		it('reads each Visit’s day and time in the Practice’s zone and is told which zone it is', async () => {
			await setup();

			await expect.element(testPage.getByText(ZONE_NOTE)).toBeVisible();
			expect(whenOf(earlierVisit)).toBe('Sat Sep 12, 10:30am');
			expect(whenOf(laterVisit)).toBe('Fri Sep 18, 10:05pm');
			expect(headingCount('Fri Sep 18')).toBe(2);
		});

		it('says the zone once, as plain text that is no stop in the tab order', async () => {
			await setup();

			const note = testPage.getByText(ZONE_NOTE);
			expect(note.elements()).toHaveLength(1);
			expect(note.element().closest('a, button, [tabindex]')).toBeNull();
		});
	});

	describe('a reader in Denver', () => {
		inReaderZone('America/Denver');

		it('reads the same words as the Practice and is told the zone', async () => {
			await setup();

			await expect.element(testPage.getByText(ZONE_NOTE)).toBeVisible();
			expect(whenOf(laterVisit)).toBe('Fri Sep 18, 10:05pm');
		});
	});

	describe('a reader in the Practice’s own zone', () => {
		inReaderZone('America/New_York');

		it('adds nothing', async () => {
			await setup();

			await expect.element(testPage.getByRole('table', { name: 'Visits' })).toBeVisible();
			expect(testPage.getByText(/time zone/).elements()).toHaveLength(0);
			expect(whenOf(laterVisit)).toBe('Fri Sep 18, 10:05pm');
		});
	});
});

// The table's day headings, in order: a rowgroup header has no accessible role the locators reach, so the
// table's own markup is read (no accessible signal for it).
const dayHeadings = () =>
	[...document.querySelectorAll('.table-view th[scope="rowgroup"]')].map((heading) => heading.textContent);

const latestData = (): SchedulePageData => ({
	...data,
	view: { ...data.view, order: 'latest' },
	filters: { ...data.filters, order: 'latest' },
	page: { ...schedulePage, items: schedulePage.items.toReversed() }
});

describe('the reader’s order (#2869)', () => {
	it('draws an Order select, Soonest first by default, as the last field before Apply', async () => {
		await setup();

		const order = testPage.getByRole('combobox', { name: 'Order' });
		await expect.element(order).toHaveValue('');
		await expect.element(order).toHaveDisplayValue('Soonest first');
		const doula = testPage.getByRole('combobox', { name: 'Doula' }).element();
		const apply = testPage.getByRole('button', { name: 'Apply' }).element();
		expect(doula.compareDocumentPosition(order.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(order.element().compareDocumentPosition(apply) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('goes to ?order=latest on Apply, beside the other narrowing, and leaves it out for Soonest first', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await testPage.getByRole('combobox', { name: 'Order' }).selectOptions('Latest first');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7&order=latest`, NOSEARCH);

		await testPage.getByRole('combobox', { name: 'Order' }).selectOptions('Soonest first');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7`, NOSEARCH);
	});

	it('draws no tag for the applied order: Clear filters is drawn beside the other tag and removes both', async () => {
		setAddress('?days=7&order=latest');
		await setup({ data: { ...latestData(), view: { ...data.view, preset: 7, order: 'latest' } } });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Order Latest first' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Next 7 days' }))
			.toHaveAttribute('href', `${schedulePath}?order=latest`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', schedulePath);
		await expect.element(testPage.getByRole('combobox', { name: 'Order' })).toHaveValue('latest');
	});

	it('keeps the order on the links an empty result offers', async () => {
		setAddress('?days=7&order=latest');
		await setup({
			data: emptyData(
				{ preset: 7, isMine: false, doula: '', needsDoula: false, order: 'latest' },
				{ from: '2026-09-07', to: '2026-09-13', timeZone: 'America/New_York', order: 'latest' }
			)
		});

		await expect
			.element(testPage.getByRole('link', { name: 'Show the next 30 days' }))
			.toHaveAttribute('href', `${schedulePath}?order=latest`);
	});

	it('ends the result line on "latest first", and marks When descending', async () => {
		setAddress('?order=latest');
		await setup({ data: latestData() });

		await expect.element(testPage.getByRole('status')).toHaveTextContent(/, latest first\.$/);
		await expect.element(testPage.getByRole('columnheader', { name: 'When' })).toHaveAttribute('aria-sort', 'descending');
		expect(testPage.getByRole('columnheader', { name: 'Client' }).element()).not.toHaveAttribute('aria-sort');
	});

	it('marks When ascending by default', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'When' })).toHaveAttribute('aria-sort', 'ascending');
	});

	it('runs the day headings newest first, one heading for each day', async () => {
		setAddress('?order=latest');
		await setup({ data: latestData() });

		expect(dayHeadings()).toEqual(['Mon Jan 4, 2027', 'Fri Sep 18', 'Sat Sep 12']);
	});

	it('continues the same order and one day heading when Load more adds the rest of a day', async () => {
		setAddress('?order=latest');
		const [first, second] = schedulePage.items.toReversed();
		const sameDay = { ...second!, visitId: 'visit-9', scheduledAt: '2026-09-19T01:00:00Z' };
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [sameDay], hasMore: false }));
		await setup({
			data: { ...latestData(), page: { items: [first!, second!], hasMore: true, nextCursor: 'cursor-1' } }
		});

		await testPage.getByRole('button', { name: 'Load more' }).click();

		const url = new URL(apiFetchWithSession.mock.calls[0][0], 'https://example.test');
		expect(url.searchParams.get('order')).toBe('latest');
		expect(url.searchParams.get('cursor')).toBe('cursor-1');
		await expect.element(testPage.getByRole('link', { name: startsWith(second!.clientName) }).first()).toBeVisible();
		expect(dayHeadings().filter((day) => day === 'Fri Sep 18')).toHaveLength(1);
	});
});

describe('Needing a new Doula (#2863)', () => {
	const narrowed = { ...data.view, needsDoula: true };

	it('offers the box to an Owner, named for the address', async () => {
		await setup();

		await expect.element(testPage.getByRole('checkbox', { name: 'Needing a new Doula' })).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Needing a new Doula' })).toHaveAttribute('name', 'needsDoula');
	});

	it('offers an Admin who is no Doula the box with no Mine', async () => {
		pageState.data = { session: { ...practiceSession(['admin']).session } };
		await setup();

		await expect.element(testPage.getByRole('checkbox', { name: 'Needing a new Doula' })).toBeVisible();
		expect(testPage.getByRole('checkbox', { name: 'Mine' }).elements()).toHaveLength(0);
	});

	it('offers a Doula no box, and ignores the address', async () => {
		asDoula();
		setAddress('?needsDoula=true');
		await setup({ data: { ...data, doulas: [] } });

		expect(testPage.getByRole('checkbox', { name: 'Needing a new Doula' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('link', { name: /Remove filter: Needing/ }).elements()).toHaveLength(0);
	});

	it('goes to the address with needsDoula=true when the box is ticked and applied', async () => {
		await setup();

		await testPage.getByRole('checkbox', { name: 'Needing a new Doula' }).click();
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?needsDoula=true`, NOSEARCH);
	});

	it('draws a tag with a ×, says the narrowed rows, and ticks the box', async () => {
		setAddress('?needsDoula=true');
		await setup({ data: { ...data, view: narrowed, page: { items: [offVisit], hasMore: false, visitCount: 1 } } });

		await expect.element(testPage.getByRole('checkbox', { name: 'Needing a new Doula' })).toBeChecked();
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Needing a new Doula' })).toHaveAttribute('href', schedulePath);
		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing 1 Visit that needs a new Doula from Mon\sSep\s7 to Tue\sOct\s6, soonest\sfirst\.$/);
	});

	it('keeps the narrowing in another tag’s ×, and "Clear filters" removes it', async () => {
		setAddress('?days=7&needsDoula=true');
		await setup({ data: { ...data, view: { ...narrowed, preset: 7 } } });

		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Next 7 days' }))
			.toHaveAttribute('href', `${schedulePath}?needsDoula=true`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', schedulePath);
	});

	it('says how many wait after the count when the rows are not narrowed to them', async () => {
		await setup({ data: { ...data, page: { ...schedulePage, visitCount: 3, offEngagementCount: 1 } } });

		await expect.element(testPage.getByRole('status')).toHaveTextContent(/soonest first\. 1 needs a new Doula\.$/);
	});

	it('says that no Visit is waiting, and offers the next 30 days from other days', async () => {
		setAddress('?days=7&needsDoula=true');
		await setup({
			data: emptyData({ ...narrowed, preset: 7 }, { from: '2026-09-07', to: '2026-09-13', timeZone: 'America/New_York' })
		});

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^No Visit is waiting for a new Doula from Mon\sSep\s7 to Sun\sSep\s13\.$/);
		await expect
			.element(testPage.getByRole('link', { name: 'Show the next 30 days' }))
			.toHaveAttribute('href', `${schedulePath}?needsDoula=true`);
	});
});
