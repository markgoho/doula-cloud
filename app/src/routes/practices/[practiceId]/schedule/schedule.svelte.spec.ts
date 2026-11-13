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
	vi.setSystemTime(new Date(2026, 8, 7, 9, 0));
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
		const whenCell = testPage.getByRole('cell', { name: /^[A-Z][a-z]{2} Sep \d+, / }).first();
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
		await expect.element(table).toHaveAccessibleDescription(/^Showing 3 Visits from Mon Sep 7 to Tue Oct 6, soonest first\.$/);
		expect(testPage.getByText(/^Every Visit this Practice has scheduled/).elements()).toHaveLength(0);
	});

	it('says a Visit’s year when it is not this one', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: /^Mon Jan 4, 2027, \d+:\d\d[ap]m$/ })).toBeVisible();
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

	it('offers the next 30 days from an empty narrower view, keeping Mine and the Doula', async () => {
		setAddress('?days=7&mine=true');
		await setup({ data: emptyData({ preset: 7, isMine: true, doula: '' }, { from: '2026-09-07', to: '2026-09-13', staffId: 'staff-1' }) });

		await expect.element(testPage.getByText('You have no Visits from Mon Sep 7 to Sun Sep 13.').first()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Show the next 30 days' }))
			.toHaveAttribute('href', `${schedulePath}?mine=true`);
	});

	it('says Visits for a picked Doula, and for Mine beside someone else', async () => {
		setAddress('?doula=staff-2');
		await setup({ data: emptyData({ preset: 30, isMine: false, doula: 'staff-2' }) });
		await expect.element(testPage.getByText('No Visits for Bo Ng from Mon Sep 7 to Tue Oct 6.').first()).toBeVisible();
	});

	it('names the Doula it does not know, by her place', async () => {
		setAddress('?doula=staff-9');
		await setup({ data: { ...emptyData({ preset: 30, isMine: false, doula: 'staff-9' }), doulas: [] } });

		await expect.element(testPage.getByText('No Visits for that Doula from Mon Sep 7 to Tue Oct 6.').first()).toBeVisible();
	});

	it('counts a Mine result as the reader’s own, even when she is also the picked Doula', async () => {
		setAddress(`?mine=true&doula=${ownSession.session.staffId}`);
		await setup({ data: { ...data, view: { preset: 30, isMine: true, doula: ownSession.session.staffId } } });

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent('Showing 3 of your Visits from Mon Sep 7 to Tue Oct 6, soonest first.');
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

	it('opens From and To on the days the choice before showed, and says the To day is included', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('From')).toHaveValue('2026-09-07');
		await expect.element(testPage.getByLabelText('To')).toHaveValue('2026-10-06');
		await expect.element(testPage.getByLabelText('To')).toHaveAccessibleDescription('This day is included.');
	});

	it('keeps what was typed when the radio is chosen again', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('To').fill('2026-09-20');

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('To')).toHaveValue('2026-09-20');
	});

	it('starts From and To on the next 7 days when that was the choice before', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();

		await expect.element(testPage.getByLabelText('To')).toHaveValue('2026-09-13');
	});

	it('goes to the address for the next 7 days, Mine and a Doula, so it survives a reload and can be shared', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await testPage.getByRole('checkbox', { name: 'Mine' }).click();
		await testPage.getByRole('combobox', { name: 'Doula' }).selectOptions(earlierVisit.staffName);
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?days=7&mine=true&doula=${earlierVisit.staffId}`);
	});

	it('goes to the bare path for the default days and nothing else', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(schedulePath);
	});

	it('goes to the chosen days', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('From').fill('2026-09-10');
		await testPage.getByLabelText('To').fill('2026-09-19');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?from=2026-09-10&to=2026-09-19`);
	});

	it('refuses a To day before the From day beside To, moves focus there, and goes nowhere', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('To').fill('2026-09-01');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expect
			.element(testPage.getByLabelText('To'))
			.toHaveAccessibleDescription('This day is included. Choose a To day that is the same as the From day or later.');
		await expect.element(testPage.getByLabelText('To')).toHaveFocus();
		expect(goto).not.toHaveBeenCalled();
	});

	it('asks for an empty From day, and puts focus there', async () => {
		await setup();
		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await testPage.getByLabelText('From').fill('');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expect.element(testPage.getByLabelText('From')).toHaveAccessibleDescription('Enter a From day');
		await expect.element(testPage.getByLabelText('From')).toHaveFocus();
	});

	it('draws a cold address whose To is before its From in place, with Choose days on, and no list', async () => {
		setAddress('?from=2026-09-19&to=2026-09-12');
		await setup({
			data: {
				...data,
				page: { items: [], hasMore: false },
				view: { preset: 30, chosen: { from: '2026-09-19', to: '2026-09-12' }, isMine: false, doula: '' },
				filters: { from: '', to: '' },
				rangeErrors: { to: 'Choose a To day that is the same as the From day or later.' }
			}
		});

		await expect.element(testPage.getByRole('radio', { name: 'Choose days' })).toBeChecked();
		await expect
			.element(testPage.getByLabelText('To'))
			.toHaveAccessibleDescription('This day is included. Choose a To day that is the same as the From day or later.');
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

	it('leaves the picker waiting when the roster could not be read, and still draws the list', async () => {
		await setup({ data: { ...data, doulas: [], isRosterUnavailable: true } });

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toBeDisabled();
		await expect.element(testPage.getByRole('table', { name: 'Visits' })).toBeVisible();
	});

	it('keeps the Doula a shared link names once the Doulas land', async () => {
		setAddress(`?doula=${laterVisit.staffId}`);
		const screen = await setup({});

		await screen.rerender({
			data: { ...data, view: { preset: 30, isMine: false, doula: laterVisit.staffId } }
		});

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toHaveValue(laterVisit.staffId);
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

		expect(goto).toHaveBeenCalledWith(schedulePath);
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
				isOffEngagement: false
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

	it('says a failed next page through the table, keeps the rows, and shows no raw server text', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));
		await setup({ data: { ...data, page: morePage } });

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('More Visits did not load. Nothing you saved is lost. Try again.');
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

describe('the Visits’ days and clocks', () => {
	it('sets the day and the clock in tracks of their own, so every clock ends on one edge', async () => {
		await setup();

		const clocks = testPage.getByRole('cell', { name: /[ap]m$/ }).elements().map((cell) => {
			const clock = cell.querySelector('.clock')!;
			return Math.round(clock.getBoundingClientRect().right);
		});
		expect(new Set(clocks).size).toBe(1);
	});
});
