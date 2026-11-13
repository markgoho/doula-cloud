import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import StaffFeedback from '#lib/components/organisms/StaffFeedback.svelte';
import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
import { toApiResponder, toPageState } from '../../../routeFixture.js';
import { fixture, seedLikeClients, typical } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The Clients list's content and the `page` it reads both come from the
 * route's own fixture (#596), so what this spec asserts on and what the
 * continuum sweep measures are one description. `vi.mock` is hoisted
 * above every import, so the object is declared empty here and filled
 * from the fixture once the imports have run -- the route reads `page`
 * inside its own functions rather than destructuring it at module scope,
 * so the later write is seen. Same installation, through the same
 * `toPageState`, as `route-continuum.svelte.spec.ts`.
 *
 * `pageState.url` ends up a real `URL`, whose `searchParams` getter
 * returns the same `URLSearchParams` instance every read (WHATWG), so a
 * test can set `all` on it directly before `render()` -- mutated before
 * render, not mid-test, since the mock is not Svelte-reactive.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>,
	state: {} as { clientsName?: string; clientsPages?: number }
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));
// Reactive, because a test moves the Name mid-render the way a history entry
// does, and the route reads it through a derived.
class HistoryState {
	clientsName = $state<string>();
	clientsPages = $state<number>();
}
pageState.state = new HistoryState();

const goto = vi.hoisted(() => vi.fn());
// SvelteKit's `replaceState` rewrites the current history entry's state; the
// stand-in does the same to the page state this file installs.
const replaceState = vi.hoisted(() => vi.fn());
// The route's own `afterNavigate` callback, kept so a spec can play a move to
// this list (SvelteKit calls it after every navigation, the first load too).
type Move = { from?: { route: { id: string }; params: Record<string, string> } };
const navigation = vi.hoisted(() => ({
	onMove: undefined as ((move: Move) => void) | undefined,
	onBefore: undefined as (() => void) | undefined
}));
vi.mock('$app/navigation', () => ({
	goto,
	replaceState,
	beforeNavigate: (callback: () => void) => {
		navigation.onBefore = callback;
	},
	afterNavigate: (callback: (move: Move) => void) => {
		navigation.onMove = callback;
	}
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId } = fixture.params;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	replaceState.mockReset();
	replaceState.mockImplementation((_url: string, state: { clientsName?: string; clientsPages?: number }) => {
		pageState.state.clientsPages = state.clientsPages;
	});
	pageState.state.clientsPages = undefined;
	pageState.url.searchParams.delete('all');
	pageState.url.searchParams.delete('mine');
	pageState.url.searchParams.delete('owes');
	pageState.url.searchParams.delete('noContract');
	pageState.url.searchParams.delete('doula');
	pageState.state.clientsName = undefined;
});

// The Staff shell above the list: its bar and its Pilot banner, drawn by the
// same components the layout draws, so the first screen is measured with what
// sits on it. The page is the route as the layout hands it over.
interface ListScreen {
	width: number;
	height: number;
	respond?: typeof typical.respond;
	hasAnyClient?: Promise<boolean>;
	isDoula?: boolean;
}

async function renderListScreen({
	width,
	height,
	respond = typical.respond,
	hasAnyClient = Promise.resolve(true),
	isDoula = true
}: ListScreen) {
	apiFetchWithSession.mockImplementation(toApiResponder({ ...fixture, respond }));
	await testPage.viewport(width, height);
	await render(StaffTopBar, {
		navItems: [{ label: 'Clients', href: `/practices/${practiceId}/clients`, current: true }],
		practices: [],
		currentPracticeId: practiceId,
		firstName: 'Kimiko',
		lastName: 'Nakashima',
		signOut: vi.fn()
	});
	await render(StaffFeedback, { email: 'kimiko@example.test', onSend: vi.fn() });
	await render(Page, {
		data: {
			isContractor: false,
			isOwner: true,
			isDoula,
			hasOtherDoulas: true,
			hasMine: Promise.resolve(true),
			hasAnyClient,
			doulas: Promise.resolve(doulas),
			session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor: false }
		}
	});
}

/*
 * The Clients table sizes itself from its values (ADR-0024): Name, Money, Date
 * and Doula are as wide as their longest value and never wrap, Status takes
 * the rest, and the list is records below the width the table needs. These
 * specs hold that at the windows a laptop and a desktop give, on the typical
 * page and on the demo Practice's busiest rows, and across the filter states.
 */
const kimiko = { value: 'staff-kimiko', label: 'Kimiko Nakashima' };
const lena = { value: 'staff-lena', label: 'Lena Vasquez' };
const doulas = [kimiko, lena];

const table = () => testPage.getByRole('table');
const group = () => testPage.getByRole('group', { name: 'Clients', exact: true });
const isTableShown = () => getComputedStyle(document.querySelector('table.table-view')!).display !== 'none';
const rowHeights = () =>
	testPage
		.getByRole('row')
		.elements()
		.slice(1)
		.map((row) => row.getBoundingClientRect().height);

const busiest = () => jsonResponse({ items: seedLikeClients, hasMore: false });
const datasets = [
	['the demo Practice’s busiest rows', busiest],
	['typical pilot data', typical.respond]
] as const;

async function ready() {
	// Both trees are in the DOM; the table's rows are there whichever is drawn.
	await vi.waitFor(() => expect(document.querySelectorAll('table.table-view tbody tr').length).toBeGreaterThan(0));
	await document.fonts.ready;
}

const px = (value: string) => Number(value.replace('px', ''));

// The words an element shows: from its first visible text to its last, leaving out text that is
// only for a screen reader, which a clip hides but a range still measures.
const wordsOf = (element: Element): { getBoundingClientRect: () => DOMRect; getClientRects: () => DOMRect[] } => {
	const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
	const boxes: DOMRect[] = [];
	for (let node = walker.nextNode(); node; node = walker.nextNode()) {
		if (node.parentElement!.closest('.visually-hidden')) continue;
		const range = document.createRange();
		range.selectNodeContents(node);
		boxes.push(...range.getClientRects());
	}
	return {
		getClientRects: () => boxes,
		getBoundingClientRect: () => {
			if (boxes.length === 0) return new DOMRect();
			const left = Math.min(...boxes.map((box) => box.left));
			const top = Math.min(...boxes.map((box) => box.top));
			const right = Math.max(...boxes.map((box) => box.right));
			const bottom = Math.max(...boxes.map((box) => box.bottom));
			return new DOMRect(left, top, right - left, bottom - top);
		}
	};
};

// Whether an element's words sit on one line: every box they make starts within half a line of the first.
function isOneLine(element: Element) {
	const tops = [...wordsOf(element).getClientRects()].map((box) => box.top);
	const line = px(getComputedStyle(element).lineHeight);
	return tops.length === 0 || Math.max(...tops) - Math.min(...tops) < line / 2;
}

// The table's two thresholds, in the container's own width: records below the
// first, the table with the Doula folded under the Date and two-line rows from
// it, the Doula in a column of her own and one-line rows from the second.
const FLOOR_PX = 55 * 16;
const WIDE_PX = 75.2 * 16;
const WINDOWS = [880, 960, 1024, 1100, 1180, 1280, 1366, 1440, 1920];

const containerWidth = () => document.querySelector('.clients-table')!.getBoundingClientRect().width;
const isDrawn = (element: Element) => element.getClientRects().length > 0;
const drawn = (selector: string) => [...document.querySelectorAll<HTMLElement>(selector)].filter((element) => isDrawn(element));

// What `--space-2` resolves to in the table's own container.
function space2() {
	const probe = document.createElement('div');
	probe.style.paddingInlineStart = 'var(--space-2)';
	document.querySelector('.clients-table')!.append(probe);
	const value = px(getComputedStyle(probe).paddingInlineStart);
	probe.remove();
	return value;
}

const headerLefts = () =>
	Object.fromEntries(drawn('table.table-view thead th').map((th) => [th.textContent, th.getBoundingClientRect().left]));

describe('the table at the windows a laptop and a desktop give', () => {
	for (const [name, respond] of datasets) {
		it.each(WINDOWS)(`draws ${name} as records below the floor and as a table of equal rows from it, at %ipx`, async (width) => {
			await renderListScreen({ width, height: 900, respond });
			await ready();

			expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(document.documentElement.clientWidth);
			const given = containerWidth();
			expect(isTableShown(), `a ${given}px container`).toBe(given >= FLOOR_PX);
			if (!isTableShown()) {
				await expect.element(group()).toBeVisible();
				return;
			}

			const heights = rowHeights();
			expect(Math.max(...heights) - Math.min(...heights), `row heights ${heights.map((height) => Math.round(height))} at ${width}px`).toBeLessThanOrEqual(4);
			if (given >= WIDE_PX) expect(Math.max(...heights), `a one-line row at ${width}px`).toBeLessThan(48);
			else expect(Math.min(...heights), `a two-line row at ${width}px`).toBeGreaterThan(56);

			for (const header of drawn('table.table-view thead th')) {
				expect(isOneLine(header), `header "${header.textContent}" breaks at ${width}px`).toBe(true);
				expect(header.textContent, `a header broke mid-word at ${width}px`).toMatch(/^[A-Za-z]+$/);
			}
			const heads = drawn('table.table-view thead th');
			const gutter = 2 * space2();
			const leading = heads.slice(0, -1);
			for (const [index, head] of leading.entries()) {
				const between = px(getComputedStyle(head).paddingInlineEnd) + px(getComputedStyle(heads[index + 1]!).paddingInlineStart);
				expect(between, `the gutter after "${head.textContent}" at ${width}px`).toBeGreaterThanOrEqual(gutter - 0.5);
			}

			for (const row of document.querySelectorAll<HTMLElement>('table.table-view tbody tr')) {
				const cells = ([...row.children] as HTMLElement[]).filter((cell) => isDrawn(cell));
				const before = cells.slice(0, -1);
				for (const [index, cell] of before.entries()) {
					expect(
						wordsOf(cell).getBoundingClientRect().right,
						`"${cell.textContent}" overlaps its neighbor at ${width}px`
					).toBeLessThanOrEqual(cells[index + 1]!.getBoundingClientRect().left + 0.5);
				}
				for (const one of [cells[0]!, ...row.querySelectorAll('.money, .date, .doula-names')]) {
					if (isDrawn(one)) expect(isOneLine(one), `"${one.textContent}" wraps at ${width}px`).toBe(true);
				}
				for (const date of row.querySelectorAll('.date')) {
					const own = date.closest('td')!;
					expect(date.getBoundingClientRect().right, `a date overruns its cell at ${width}px`).toBeLessThanOrEqual(own.getBoundingClientRect().right + 0.5);
				}
			}
		});
	}

	it('folds the Doula under the Date between the floor and the wide band, and draws her own column from it', async () => {
		for (const [width, folded] of [[1024, true], [1440, false]] as const) {
			await renderListScreen({ width, height: 900 });
			await ready();
			expect(drawn('table.table-view .folded-doula').length > 0, `folded at ${width}px`).toBe(folded);
			expect(Object.keys(headerLefts()).includes('Doula'), `a Doula column at ${width}px`).toBe(!folded);
			cleanup();
		}
	});

	it.each(datasets)('shows at least six whole one-line rows on a 1280x800 window, on %s', async (_name, respond) => {
		await renderListScreen({ width: 1280, height: 800, respond });
		await ready();

		const rows = [...document.querySelectorAll<HTMLElement>('table.table-view tbody tr')];
		const whole = rows.filter((row) => row.getBoundingClientRect().bottom <= window.innerHeight);
		expect(whole.length, `whole rows by bottom edge: ${rows.map((row) => Math.round(row.getBoundingClientRect().bottom))}`).toBeGreaterThanOrEqual(6);
		for (const row of rows) expect(row.getBoundingClientRect().height, 'a one-line row at 1280px').toBeLessThan(48);
	});

	// The tags are one row of their own, drawn only while a filter is on: it costs a row and a half.
	it.each(datasets)('shows at least five whole one-line rows on a 1280x800 window with one filter applied, on %s', async (_name, respond) => {
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width: 1280, height: 800, respond });
		await ready();

		const rows = [...document.querySelectorAll<HTMLElement>('table.table-view tbody tr')];
		const whole = rows.filter((row) => row.getBoundingClientRect().bottom <= window.innerHeight);
		expect(whole.length, `whole rows by bottom edge: ${rows.map((row) => Math.round(row.getBoundingClientRect().bottom))}`).toBeGreaterThanOrEqual(5);
		for (const row of rows) expect(row.getBoundingClientRect().height, 'a one-line row at 1280px').toBeLessThan(48);
	});

	it('is a table wherever a window holds the floor, and records or a table at no width in between that a wider one lacks', async () => {
		const shown: boolean[] = [];
		for (const width of WINDOWS) {
			await renderListScreen({ width, height: 900 });
			await ready();
			shown.push(isTableShown());
			cleanup();
		}
		expect(shown.slice(1), `table shown at ${WINDOWS}: ${shown}`).toStrictEqual(shown.slice(1).map(() => true));
		expect(shown[0]).toBe(false);
	});

	it.each([320, 375, 640, 768])('draws records, with no sideways scroll, at %ipx', async (width) => {
		await renderListScreen({ width, height: 900, respond: busiest });
		await expect.element(group()).toBeVisible();
		expect(isTableShown()).toBe(false);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(document.documentElement.clientWidth);
	});

	it('lets records fill a wide container instead of stopping at a reading measure', async () => {
		await renderListScreen({ width: 768, height: 900, respond: busiest });
		await expect.element(group()).toBeVisible();
		const cards = [...group().element().querySelectorAll('.record')].map((record) => record.getBoundingClientRect());
		const row = cards.filter((card) => Math.abs(card.top - cards[0]!.top) <= 1);
		const spanned = Math.max(...row.map((card) => card.right)) - Math.min(...row.map((card) => card.left));
		expect(spanned, `${row.length} card(s) across ${containerWidth()}px`).toBeGreaterThan(containerWidth() * 0.95);
	});

	it('gives records a second column where the container holds two', async () => {
		await renderListScreen({ width: 880, height: 900, respond: busiest });
		await expect.element(group()).toBeVisible();
		const [first, second] = [...group().element().querySelectorAll('.record')].map((record) => record.getBoundingClientRect());
		expect(second!.left).toBeGreaterThan(first!.right - 1);
		expect(Math.abs(second!.top - first!.top)).toBeLessThanOrEqual(1);
	});
});

const withoutMoney = {
	items: seedLikeClients.map((client) => ({
		...client,
		openEngagements: client.openEngagements?.map((line) => ({
			...line,
			invoiceStatus: undefined,
			invoiceAmountCents: undefined,
			daysOverdue: undefined
		}))
	})),
	hasMore: false
};

async function lefts(queries: Record<string, string>, body: unknown, width: number, name = '') {
	for (const [key, value] of Object.entries(queries)) pageState.url.searchParams.set(key, value);
	pageState.state.clientsName = name || undefined;
	await renderListScreen({ width, height: 900, respond: () => jsonResponse(body, 200) });
	await ready();
	const result = headerLefts();
	cleanup();
	for (const key of Object.keys(queries)) pageState.url.searchParams.delete(key);
	return result;
}

describe('the columns between filter states', () => {
	const shortest = seedLikeClients.toSorted((a, b) => a.name.length - b.name.length)[0]!;

	it.each([1024, 1280])('keeps Name and Status where they were in every filter state, at %ipx', async (width) => {
		const everyone = { items: seedLikeClients, hasMore: false };
		const states = {
			default: await lefts({}, everyone, width),
			owes: await lefts({ owes: 'true' }, { items: seedLikeClients.slice(0, 3), hasMore: false }, width),
			noContract: await lefts({ noContract: 'true' }, withoutMoney, width),
			doula: await lefts({ doula: kimiko.value }, everyone, width),
			name: await lefts({}, { items: [shortest], hasMore: false }, width, shortest.name)
		};

		for (const [state, columns] of Object.entries(states)) {
			for (const column of ['Name', 'Status'] as const) {
				expect(Math.abs(columns[column]! - states.default[column]!), `${column} moved under ${state} at ${width}px: ${JSON.stringify(columns)}`).toBeLessThanOrEqual(1);
			}
		}
		expect(Object.keys(states.doula)).not.toContain('Doula');
		expect(Object.keys(states.noContract)).not.toContain('Money');
	});

	it.each([
		[1280, 2],
		[1024, 2]
	])('reserves the count line its longest wording needs and holds the table still across filter states, at %ipx', async (width, lines) => {
		const tops: number[] = [];
		const heights: number[] = [];
		const states = [
			[{}, ''],
			[{ owes: 'true' }, ''],
			[{ doula: kimiko.value, noContract: 'true', owes: 'true' }, ''],
			[{}, shortest.name]
		] as const;
		for (const [queries, name] of states) {
			for (const [key, value] of Object.entries(queries)) pageState.url.searchParams.set(key, value);
			pageState.state.clientsName = name || undefined;
			await renderListScreen({ width, height: 800, respond: () => jsonResponse({ items: [shortest], hasMore: true }, 200) });
			await ready();
			const status = document.querySelector('#clients-result')!;
			heights.push(document.querySelector('.count')!.getBoundingClientRect().height / px(getComputedStyle(status).lineHeight));
			tops.push(document.querySelector('table.table-view thead')!.getBoundingClientRect().top);
			cleanup();
			for (const key of Object.keys(queries)) pageState.url.searchParams.delete(key);
		}

		// The tags share the count's row: one tag fits inside the room the count holds, so
		// neither it nor a typed name moves the table. Three tags take more of the row than
		// the count can wrap beside, so that row grows, by less than a row of tags of its own.
		const [rest, owes, three, name] = heights;
		for (const height of [rest!, owes!, name!]) expect(height, `count line heights ${heights}`).toBeCloseTo(lines, 0);
		expect(three!, `count line heights ${heights}`).toBeGreaterThanOrEqual(lines - 0.5);
		for (const top of [tops[1]!, tops[3]!]) expect(Math.abs(top - tops[0]!), `table tops ${tops}`).toBeLessThanOrEqual(1);
		expect(tops[2]! - tops[0]!, `the three tags' row, table tops ${tops}`).toBeLessThan(70);
	});

	it('keeps Money and Date where they were when a Doula is chosen, and Date follows Status when Money is left out', async () => {
		const body = { items: seedLikeClients, hasMore: false };
		const base = await lefts({}, body, 1440);
		const oneDoula = await lefts({ doula: kimiko.value }, body, 1440);
		const noContract = await lefts({ noContract: 'true' }, withoutMoney, 1440);

		expect(Object.keys(oneDoula)).toStrictEqual(['Name', 'Status', 'Money', 'Date']);
		expect(Object.keys(noContract)).toStrictEqual(['Name', 'Status', 'Date', 'Doula']);
		for (const column of ['Money', 'Date'] as const) {
			expect(Math.abs(oneDoula[column]! - base[column]!), `${column} moved under a Doula`).toBeLessThanOrEqual(1);
		}
		expect(Math.abs(noContract.Date! - base.Money!), 'Date took Money’s place').toBeLessThanOrEqual(1);
	});
});

describe('a wide window', () => {
	it.each(datasets)('keeps Money, Date and Doula near the status words at 1920px on %s', async (_name, respond) => {
		await renderListScreen({ width: 1920, height: 900, respond });
		await ready();

		const rows = [...document.querySelectorAll<HTMLElement>('table.table-view tbody tr')];
		const longest = Math.max(...rows.map((row) => wordsOf(row.querySelector(':scope > td.grows')!).getBoundingClientRect().right));
		const money = Math.min(...rows.map((row) => row.querySelector(':scope > td.numeric')!.getBoundingClientRect().left));
		// The status words end within four gutter steps (--space-2) of the Money, measured at
		// 12px: the Status column is its busiest line wide, never the frame's slack.
		const bound = 4 * space2();
		expect(money - longest, `${money - longest}px from the longest status to the Money`).toBeLessThanOrEqual(bound);
		expect(containerWidth() - document.querySelector('table.table-view')!.getBoundingClientRect().width, 'the table stops where its columns are satisfied').toBeGreaterThan(0);
	});
});

describe('the Date and the last column', () => {
	it('gives “Ended” room before its day, in the table', async () => {
		await renderListScreen({ width: 1440, height: 900, respond: busiest });
		await ready();

		const lead = table().getByText('Ended', { exact: true }).first().element();
		const day = lead.nextElementSibling!;
		const gap = day.getBoundingClientRect().left - (wordsOf(lead).getBoundingClientRect().right);
		expect(gap).toBeGreaterThanOrEqual(4);
	});

	it.each([320, 640])('draws a card’s date and its Doula a line each, with no dot between, at %ipx', async (width) => {
		await renderListScreen({ width, height: 900, respond: typical.respond });
		await expect.element(group()).toBeVisible();

		const meta = group().element().querySelector('.compact-meta')!;
		expect(getComputedStyle(meta.querySelector('.compact-separator')!).display).toBe('none');
		const date = meta.querySelector('.date')!.getBoundingClientRect();
		const doula = wordsOf(meta.querySelector('.doula-names')!).getBoundingClientRect();
		expect(doula.top, 'the Doula under the date').toBeGreaterThanOrEqual(date.bottom - 1);
		expect(Math.abs(doula.left - date.left), 'on the same left edge').toBeLessThanOrEqual(1);
	});

	it('leaves the last column its end inset, and the first its start edge', async () => {
		await renderListScreen({ width: 1440, height: 900, respond: busiest });
		await ready();

		const row = document.querySelector('table.table-view tbody tr')!;
		const last = row.lastElementChild as HTMLElement;
		const first = row.firstElementChild as HTMLElement;
		expect(px(getComputedStyle(last).paddingInlineEnd)).toBeGreaterThan(0);
		expect(px(getComputedStyle(first).paddingInlineEnd)).toBeGreaterThan(0);
		expect(px(getComputedStyle(first).paddingInlineStart)).toBe(0);
		const words = wordsOf(last.querySelector('.doula-names')!).getBoundingClientRect();
		expect(last.getBoundingClientRect().right - words.right).toBeGreaterThan(0);
	});
});

const boxes = () =>
	[...document.querySelectorAll<HTMLElement>('body *')].map((element) => {
		const rect = element.getBoundingClientRect();
		return [element, [rect.left, rect.top, rect.width, rect.height]] as const;
	});

describe('the focused “Skip to results” link', () => {
	it.each([320, 375, 1280, 1920])('moves nothing and covers no control or label, at %ipx', async (width) => {
		await renderListScreen({ width, height: 900 });
		await ready();

		const skip = document.querySelector<HTMLElement>('a.skip')!;
		const before = boxes();
		skip.focus();
		await expect.element(testPage.getByRole('link', { name: 'Skip to results' })).toHaveFocus();
		for (const [element, box] of before) {
			if (element === skip) continue;
			const rect = element.getBoundingClientRect();
			expect([rect.left, rect.top, rect.width, rect.height], `${element.tagName} "${element.textContent?.trim().slice(0, 20)}" moved at ${width}px`).toStrictEqual(box);
		}

		const box = skip.getBoundingClientRect();
		expect(box.width).toBeGreaterThan(0);
		expect(box.height, 'a target of at least 24px').toBeGreaterThanOrEqual(24);
		const others = [...document.querySelectorAll<HTMLElement>('input, label, button, select, summary, a, legend, h1, [role="status"]')].filter(
			(element) => element !== skip && !element.closest('.visually-hidden') && element.getClientRects().length > 0
		);
		expect(others.length).toBeGreaterThan(5);
		for (const other of others) {
			const rect = other.getBoundingClientRect();
			const isOverlapping = rect.left < box.right - 0.5 && rect.right > box.left + 0.5 && rect.top < box.bottom - 0.5 && rect.bottom > box.top + 0.5;
			expect(isOverlapping, `the skip link covers "${other.textContent?.trim() || other.tagName}" at ${width}px: ${JSON.stringify(box)} vs ${JSON.stringify(rect)} ${other.outerHTML.slice(0, 120)}`).toBe(false);
		}
	});
});

const chOf = (element: Element) => {
	const probe = document.createElement('span');
	probe.style.cssText = `display:inline-block;inline-size:1ch;font-size:${getComputedStyle(element).fontSize}`;
	element.append(probe);
	const width = probe.getBoundingClientRect().width;
	probe.remove();
	return width;
};

// The distinct lines an unseen wording is drawn on, clipped or not.
const linesOf = (element: Element) => new Set([...wordsOf(element).getClientRects()].map((box) => Math.round(box.top))).size;

// The count line is a reading line, capped at 75ch, and its held room is what the
// longest wording takes wrapped at that cap, never a line more.
describe('the count line’s measure and its held room', () => {
	it.each([1280, 1440, 1920])('never runs wider than 75ch and wraps its longest wording, at %ipx', async (width) => {
		await renderListScreen({ width, height: 800 });
		await ready();

		const status = document.querySelector<HTMLElement>('#clients-result')!;
		const ruler = document.querySelector<HTMLElement>('.ruler .status')!;
		const cap = 75 * chOf(status);
		expect(status.getBoundingClientRect().width, 'the result line').toBeLessThanOrEqual(cap + 0.5);
		expect(ruler.getBoundingClientRect().width, 'the held wording').toBeLessThanOrEqual(cap + 0.5);
		expect(linesOf(ruler), 'the longest wording wraps instead of running past the cap').toBe(2);
	});

	it.each([320, 375, 640, 768, 880, 960, 1024, 1100, 1280, 1440, 1920])('holds no more lines than the longest wording takes wrapped, at %ipx', async (width) => {
		await renderListScreen({ width, height: 800 });
		await ready();

		const status = document.querySelector<HTMLElement>('#clients-result')!;
		const held = document.querySelector('.count')!.getBoundingClientRect().height / px(getComputedStyle(status).lineHeight);
		const needed = linesOf(document.querySelector('.ruler .status')!);
		expect(held, `${held} lines held, ${needed} needed at ${width}px`).toBeLessThan(needed + 0.5);
		expect(held, `${held} lines held, ${needed} needed at ${width}px`).toBeGreaterThan(Math.min(needed, 2) - 0.5);
	});
});

describe('the header’s tint', () => {
	it('reaches the same inset past the first column’s text as past the last column’s, with the text on the page’s edge', async () => {
		await renderListScreen({ width: 1440, height: 900, respond: busiest });
		await ready();

		const heads = drawn('table.table-view thead th');
		const first = heads[0]!;
		const last = heads.at(-1)!;
		const inset = px(getComputedStyle(last).paddingInlineEnd);
		expect(inset).toBeGreaterThan(0);
		expect(px(getComputedStyle(first).paddingInlineStart), 'the text stays on the page’s edge').toBe(0);
		expect(first.getBoundingClientRect().left).toBeCloseTo(document.querySelector('h1')!.getBoundingClientRect().left, 0);

		const middle = first.getBoundingClientRect().top + first.getBoundingClientRect().height / 2;
		const left = first.getBoundingClientRect().left;
		expect(document.elementsFromPoint(left - inset / 2, middle), 'tint inside the inset').toContain(first);
		expect(document.elementsFromPoint(left - inset - 2, middle), 'no tint past it').not.toContain(first);
		expect(getComputedStyle(first, '::before').backgroundColor).toBe(getComputedStyle(first).backgroundColor);
		expect(px(getComputedStyle(first, '::before').width)).toBeCloseTo(inset, 0);
	});
});

describe('the record cards’ fields', () => {
	it.each(datasets)('draws no empty line box in any card, at 320, 375, 640 and 768px, on %s', async (_name, respond) => {
		for (const width of [320, 375, 640, 768]) {
			await renderListScreen({ width, height: 740, respond });
			await expect.element(group()).toBeVisible();
			const cards = [...group().element().querySelectorAll('.record')];
			expect(cards.length).toBeGreaterThan(6);
			for (const card of cards) {
				for (const line of card.querySelectorAll('.record-title, .compact-lead, .compact-meta, .record-money')) {
					expect(line.textContent!.trim(), `an empty line in "${card.textContent}" at ${width}px`).not.toBe('');
					expect(line.getBoundingClientRect().height, `an empty line box at ${width}px`).toBeGreaterThan(10);
				}
			}
			cleanup();
		}
	});
});
