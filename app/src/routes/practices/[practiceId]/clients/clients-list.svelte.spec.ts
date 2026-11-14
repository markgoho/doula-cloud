import { page as testPage, userEvent } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { SvelteURL } from 'svelte/reactivity';
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
import { TYPICAL_CLIENTS } from '../../../typicalPilot.js';
import { goto, invalidate, pageState } from '#testing/routePage.js';
import { snapshot } from '$app/navigation';
import type { ListReturn } from '#lib/listReturn.js';
import { toApiResponder, toPageState } from '../../../routeFixture.js';
import { clients, fixture, seedLikeClients, typical } from './page.fixture.js';
import { heldClientsName } from '#lib/clientsReturn.js';
import { dueMonthDays, dueMonthOptions } from '#lib/clientsDue.js';
import { dayIn } from '#lib/zonedTime.js';

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
vi.mock('$app/state', () => import('#testing/routePage.js'));
Object.assign(pageState, toPageState(fixture));
// Reactive, because a test moves the Name mid-render the way a history entry
// does, and the route reads it through a derived.
class HistoryState {
	clientsName = $state<string>();
	clientsPages = $state<number>();
}
const historyState = new HistoryState();
pageState.state = historyState;

// SvelteKit's `replaceState` rewrites the current history entry's state; the
// stand-in does the same to the page state this file installs.
const replaceState = vi.hoisted(() => vi.fn());
// The route's own `afterNavigate` callback, kept so a spec can play a move to
// this list (SvelteKit calls it after every navigation, the first load too).
type Move = { from?: { route: { id: string }; params: Record<string, string> } };
const navigation = vi.hoisted(() => {
	const befores: ((leaving?: { to: { url: URL } }) => void)[] = [];
	return {
		onMove: undefined as ((move: Move) => void) | undefined,
		befores,
		onBefore: (leaving?: { to: { url: URL } }) => {
			for (const before of befores) before(leaving ?? ({} as never));
		}
	};
});
vi.mock('$app/navigation', async () => ({
	...(await import('#testing/routePage.js')),
	replaceState,
	snapshot: vi.fn(),
	beforeNavigate: (callback: (leaving?: { to: { url: URL } }) => void) => {
		navigation.befores.push(callback);
	},
	afterNavigate: (callback: (move: Move) => void) => {
		navigation.onMove = callback;
	}
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId } = fixture.params;

function textResponse(body: string): Response {
	return jsonResponse(body, 403);
}

function requestUrl(callIndex = 0): string {
	return apiFetchWithSession.mock.calls[callIndex][0] as string;
}

function requestInit(callIndex = 0): RequestInit {
	return apiFetchWithSession.mock.calls[callIndex][1] as RequestInit;
}

// #1829: a Client's name is a Client's, and a URL is logged. Whatever the
// page navigated to or asked the BFF for, no address holds it.
function expectNoNameInAnyAddress(name: string) {
	const addresses = [
		...goto.mock.calls.map((call) => String(call[0])),
		...apiFetchWithSession.mock.calls.map((call) => String(call[0]))
	];
	expect(addresses.length).toBeGreaterThan(0);
	for (const address of addresses) {
		expect(decodeURIComponent(address)).not.toContain(name);
	}
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	navigation.befores.length = 0;
	vi.mocked(snapshot).mockClear();
	replaceState.mockReset();
	replaceState.mockImplementation((_url: string, state: { clientsName?: string; clientsPages?: number }) => {
		historyState.clientsPages = state.clientsPages;
	});
	historyState.clientsPages = undefined;
	pageState.url.searchParams.delete('all');
	pageState.url.searchParams.delete('mine');
	pageState.url.searchParams.delete('owes');
	pageState.url.searchParams.delete('noContract');
	pageState.url.searchParams.delete('doula');
	pageState.url.searchParams.delete('order');
	for (const key of ['due', 'dueFrom', 'dueTo']) pageState.url.searchParams.delete(key);
	historyState.clientsName = undefined;
});

// What every filter-bar navigation carries beside its address (#1829): the
// Name, in the history entry and never in the address. Blank clears it.
const noName = { state: { clientsName: '' } };

// The filter bar submits (it holds a text field and a select), so a change to
// a control reaches the address only when Apply is pressed.
const applyFilters = () => testPage.getByRole('button', { name: 'Apply' }).click();

// A `response` left undefined answers every fetch from the fixture's own
// `respond()`, per request rather than once -- the same reason
// `toApiResponder` exists. A test that needs different content passes its
// own `Response` and gets it on every call, which is what every "Load
// more"/failure case below still wants.
interface Caller {
	isContractor?: boolean;
	isDoula?: boolean;
	// Whether anyone else holds the Doula role (#2675).
	hasOtherDoulas?: boolean;
	// Whether she holds a live Client of her own.
	hasMine?: boolean;
	// What `+page.ts` read with `all=true` (#1609).
	hasAnyClient?: boolean;
	// The Practice's Doulas, which +page.ts reads for an Owner or an Admin.
	doulas?: { value: string; label: string }[];
	// Every Doula on the roster, which names a Doula who holds no Client; the options by default.
	everyDoula?: { value: string; label: string }[];
	// Whether she may choose a Doula; true where the options are given.
	canChooseDoula?: boolean;
	// Whether the list opens on "Mine" for her.
	opensOnMine?: boolean;
	// Holds the roster back until the test lets it land.
	roster?: Promise<{ value: string; label: string }[]>;
	// Answers each request by its address, for a list read in pages.
	responder?: (path: string) => Promise<Response>;
}

async function setup(
	response?: Response,
	{
		isContractor = false,
		isDoula = false,
		hasOtherDoulas = true,
		hasMine = true,
		hasAnyClient = true,
		doulas = [],
		roster,
		everyDoula = doulas,
		canChooseDoula = doulas.length > 0 || roster !== undefined,
		opensOnMine = false,
		responder
	}: Caller = {}
) {
	// DataTable's own content floor (#508) shows its record cards below
	// 55rem, and this file's assertions are about the <table> specifically.
	// A desktop window is past it (#2022), so the <table> renders.
	await testPage.viewport(1440, 900);
	if (responder) {
		apiFetchWithSession.mockImplementation(responder);
	} else if (response) {
		apiFetchWithSession.mockResolvedValue(response);
	} else {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	}
	// The generated `data` prop merges practices/[practiceId]/+layout.ts's
	// `session` (#835) into +page.ts's own ContractorGate, the way
	// SvelteKit really does at runtime -- this route never reads it, but
	// rendering the component directly still needs the full merged shape.
	await render(Page, {
		data: {
			isContractor,
			isOwner: false,
			isDoula,
			hasOtherDoulas,
			hasMine: Promise.resolve(hasMine),
			hasAnyClient: Promise.resolve(hasAnyClient),
			doulas: roster ?? Promise.resolve(doulas),
			everyDoula: roster ?? Promise.resolve(everyDoula),
			canChooseDoula,
			opensOnMine,
			session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor, timezone: 'America/New_York' }
		}
	});
}

// What a person does in the moment before the page has taken over (the mount
// has drawn the controls and its effects have not yet run) is in the controls,
// not in the drafts: it must survive, and nothing is unavailable meanwhile.
describe('before the page has taken over', () => {
	it('keeps a name typed and a box ticked in that moment, and applies them', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await testPage.viewport(1440, 900);
		const target = document.createElement('div');
		document.body.append(target);
		const app = mount(Page, {
			target,
			props: {
				data: {
					isContractor: false,
					isOwner: false,
					isDoula: false,
					hasOtherDoulas: true,
					hasMine: Promise.resolve(true),
					hasAnyClient: Promise.resolve(true),
					doulas: Promise.resolve([]),
					everyDoula: Promise.resolve([]),
					canChooseDoula: false,
					opensOnMine: false,
					session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor: false, timezone: 'America/New_York' }
				}
			}
		});

		const name = nameField().element() as HTMLInputElement;
		const owes = testPage.getByRole('checkbox', { name: 'Who owe money' }).element() as HTMLInputElement;
		expect(name.disabled).toBe(false);
		expect((testPage.getByRole('button', { name: 'Apply' }).element() as HTMLButtonElement).disabled).toBe(false);
		name.value = 'Zelda';
		owes.checked = true;

		flushSync();

		await expect.element(nameField()).toHaveValue('Zelda');
		await expect.element(testPage.getByRole('checkbox', { name: 'Who owe money' })).toBeChecked();
		await applyFilters();
		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?owes=true`, { state: { clientsName: 'Zelda' } });
		await unmount(app);
		target.remove();
	});
});

describe('clients list screen', () => {
	it('sends "Find or add a Client" to the search that fronts intake, not straight to intake (#498, #539)', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	// #1609: at a Practice that holds no Client record the search can find
	// nobody, so the link opens the name question and names only that
	// errand. Which one it is comes from +page.ts's `all=true` count; the
	// rows here are the filtered list and decide nothing.
	it('opens the name question as "Add a Client" at a Practice with no Client record', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });

		await expect
			.element(testPage.getByRole('link', { name: 'Add a Client', exact: true }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?from=clients`);
		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.not.toBeInTheDocument();
	});

	// The Staff roster's "Invite a Staff member" is the start button; this
	// screen's one action looks like it, in both of its forms.
	it('draws "Find or add a Client" as the start button', async () => {
		await setup();
		await expect.element(testPage.getByRole('link', { name: 'Find or add a Client' })).toHaveClass('start');
	});

	it('draws "Add a Client" as the start button too', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });
		await expect.element(testPage.getByRole('link', { name: 'Add a Client', exact: true })).toHaveClass('start');
	});

	it('keeps the search when the filtered list is empty but the Practice holds a Client not yet in care', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: true });

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	// The Portal invite tests moved to the Client hub with the column
	// (#2022, `client-detail.svelte.spec.ts`).

	it('shows the empty message when there are no Clients', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
	});

	it('says in plain words that the Clients did not load, and keeps the server\'s text off the screen', async () => {
		await setup(textResponse('Server rejected the Clients list request'));

		await expect
			.element(testPage.getByText('Your Clients did not load.', { exact: true }))
			.toBeVisible();
		// The button says it; the sentence above it does not say it twice.
		await expect.element(testPage.getByText('Try again', { exact: true })).toBeVisible();
		expect(testPage.getByText('Try again', { exact: true }).elements()).toHaveLength(1);
		await expect.element(testPage.getByText('Server rejected the Clients list request')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	it('loads the list again from "Try again" after a failure', async () => {
		await setup(textResponse('Server rejected the Clients list request'));
		await expect.element(testPage.getByText(/Your Clients did not load/)).toBeVisible();

		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByRole('table')).toBeVisible();
		await expect.element(testPage.getByText(/Your Clients did not load/)).not.toBeInTheDocument();
	});

	it('reads the roster behind the Doula filter again from "Try again"', async () => {
		await setup(textResponse('Server rejected the Clients list request'));
		await expect.element(testPage.getByText(/Your Clients did not load/)).toBeVisible();
		invalidate.mockClear();

		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByRole('table')).toBeVisible();
		expect(invalidate).toHaveBeenCalledWith('app:clients');
	});
});

// #539 (ADR-0017): a contractor Doula originates nothing at a Practice she
// contracts for -- neither errand behind "Find or add a Client" is hers --
// and her narrowed list is already her route to a Client she is attached
// to, so the control is gone rather than merely relabeled for her.
describe('a contractor Doula without the owner or admin role', () => {
	it('does not see "Find or add a Client" at all', async () => {
		await setup(undefined, { isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: 'Find or add a Client' }))
			.not.toBeInTheDocument();
	});

	// #2603: she is attached only through an accepted Offer (ADR-0008), so
	// a switch that adds Clients with no work could never add a row for her.
	it('is not offered the switch that adds Clients with no work', async () => {
		await setup(undefined, { isContractor: true });

		await expect.element(testPage.getByRole('link', { name: 'Find or add a Client' })).not.toBeInTheDocument();
		expect(testPage.getByRole('checkbox').elements()).toHaveLength(0);
	});

	it('names why her list is empty rather than reusing "No Clients yet."', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { isContractor: true });

		await expect
			.element(
				testPage.getByText('Work reaches you as an Offer, so there are no Clients here yet.')
			)
			.toBeVisible();
	});

	it('links her empty list to #501\'s explain-only door', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: 'How to add Clients of your own' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/search`);
	});

	it('does not show the explainer link once she has attached Clients', async () => {
		await setup(undefined, { isContractor: true });

		await expect
			.element(testPage.getByRole('link', { name: 'How to add Clients of your own' }))
			.not.toBeInTheDocument();
	});
});

describe('a non-contractor with an empty list', () => {
	it('shows the plain empty message with no explainer link', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: false });

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'How to add Clients of your own' }))
			.not.toBeInTheDocument();
	});

	// #1706: why, beside `emptyMessage` in +page.svelte.
	it('says no Client is in care yet, and how to see everyone, at a Practice whose Clients have no work', async () => {
		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: true });

		await expect
			.element(testPage.getByText('No Client is in care yet. Turn on "Also show anyone not yet in care" to list them.'))
			.toBeVisible();
		await expect.element(testPage.getByText('No Clients yet.')).not.toBeInTheDocument();
	});

	// With "Also show anyone not yet in care" on there is no filter left to point at, so an
	// empty list there reads the plain message whatever the count said.
	it('keeps the plain empty message while "Also show anyone not yet in care" is on', async () => {
		pageState.url.searchParams.set('all', 'true');

		await setup(jsonResponse({ items: [], hasMore: false }), { hasAnyClient: true });

		await expect.element(testPage.getByText('No Clients yet.')).toBeVisible();
	});
});

// #499: the default view is Clients in care -- ListHandler's own
// default -- and a visible toggle switches to everyone, including a
// Client whose only Request was refused.
describe('the "see everyone" toggle', () => {
	it('defaults to the Clients-in-care filter, unchecked and no `all` param on the wire', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' }))
			.not.toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('reflects `?all=true` from the URL: checked, and threaded through to the fetch', async () => {
		pageState.url.searchParams.set('all', 'true');

		await setup();

		await expect.element(testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' })).toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?all=true`);
	});

	it('navigates to `?all=true` when switched on from the default view', async () => {
		await setup();

		await testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' }).click();
		expect(goto).not.toHaveBeenCalled();
		await applyFilters();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?all=true`, noName);
	});

	// The filter bar submits, with one Apply, because it holds a text field
	// and a select (GOV.UK's filter pattern, ADR-0021); it says what the list
	// now holds.
	it('sits in the list’s filter bar, which says what the list holds', async () => {
		await setup();

		const bar = testPage.getByRole('group', { name: 'Choose which Clients to show' });
		await expect.element(bar.getByRole('checkbox', { name: 'Also show anyone not yet in care' })).toBeVisible();
		await expect.element(bar.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing \d+ Clients?/);
	});

	it('is a form that reaches the same address with no script: named fields carrying the words the address reads', async () => {
		await setup(undefined, { isDoula: true });

		for (const [label, name] of [
			['Also show anyone not yet in care', 'all'],
			['Mine', 'mine'],
			['Who owe money', 'owes'],
			['Active, with no Contract', 'noContract']
		]) {
			const box = testPage.getByRole('checkbox', { name: label });
			await expect.element(box).toHaveAttribute('name', name);
			await expect.element(box).toHaveAttribute('value', 'true');
		}
		// No `name` on the field: a no-script GET submit would put the Name in the address (#1829).
		await expect.element(testPage.getByLabelText('Name')).not.toHaveAttribute('name');
		const form = testPage.getByRole('group', { name: 'Choose which Clients to show' }).element().closest('form')!;
		expect(form.method).toBe('get');
		expect(new URL(form.action).pathname).toBe(`/practices/${practiceId}/clients`);
	});

	it('navigates back to the default view when switched off', async () => {
		pageState.url.searchParams.set('all', 'true');
		await setup();

		await testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' }).click();
		await applyFilters();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients`, noName);
	});
});

// #2624: an employee Doula can narrow the list to her own Clients.
const mine = () => testPage.getByRole('checkbox', { name: 'Mine' });

describe('the "Mine" switch', () => {
	it('is offered to a Doula, off, with no `mine` param on the wire', async () => {
		await setup(undefined, { isDoula: true });

		await expect.element(mine()).not.toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
		await expect.element(testPage.getByRole('group', { name: 'Choose which Clients to show' })).toBeVisible();
	});

	it('is not offered to a Staff member who holds no Doula role', async () => {
		await setup();

		await expect.element(mine()).not.toBeInTheDocument();
	});

	it('is not offered to the Practice\'s only Doula, for whom it changes nothing (#2675)', async () => {
		await setup(undefined, { isDoula: true, hasOtherDoulas: false });

		await expect.element(mine()).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('group', { name: 'Choose which Clients to show' })).toBeVisible();
	});

	it('ignores `?mine=true` for the Practice\'s only Doula', async () => {
		pageState.url.searchParams.set('mine', 'true');

		await setup(undefined, { isDoula: true, hasOtherDoulas: false });

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('is not offered to a contractor Doula, whose list is already only hers', async () => {
		await setup(undefined, { isContractor: true, isDoula: true });

		await expect.element(mine()).not.toBeInTheDocument();
	});

	it('navigates to `?mine=true` when switched on', async () => {
		await setup(undefined, { isDoula: true });

		await mine().click();
		await applyFilters();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?mine=true`, noName);
	});

	// The round trip a walk once seemed to break (the box came back unticked
	// and the list unfiltered): tick, press Apply, and the address the page
	// then reads carries `mine=true` and shows the box ticked.
	it('comes back ticked and narrowed once the address it navigated to is the page’s', async () => {
		await setup(undefined, { isDoula: true });

		await mine().click();
		await applyFilters();
		const target = new URL(goto.mock.lastCall![0] as string, 'https://example.test');
		expect(target.searchParams.get('mine')).toBe('true');

		cleanup();
		apiFetchWithSession.mockClear();
		pageState.url.search = target.search;
		await setup(undefined, { isDoula: true });

		await expect.element(mine()).toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?mine=true`);
	});

	it('reflects `?mine=true`: checked, sent to the BFF, worded as hers, with the no-care switch left out', async () => {
		pageState.url.searchParams.set('mine', 'true');

		await setup(undefined, { isDoula: true });

		await expect.element(mine()).toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?mine=true`);
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing \d+ of your Clients/);
		await expect.element(testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' })).not.toBeInTheDocument();
	});

	it('goes back to the whole list when switched off', async () => {
		pageState.url.searchParams.set('mine', 'true');
		await setup(undefined, { isDoula: true });

		await mine().click();
		await applyFilters();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients`, noName);
	});

	it('says so when none of the Clients are hers', async () => {
		pageState.url.searchParams.set('mine', 'true');

		await setup(jsonResponse({ items: [], hasMore: false }), { isDoula: true });

		await expect.element(testPage.getByRole('status')).toHaveTextContent('You are not the Doula for any Client yet. Clear the filters to widen the list.');
	});

	it('ignores `?mine=true` for a Staff member who cannot use it', async () => {
		pageState.url.searchParams.set('mine', 'true');

		await setup();

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});
});

// #2724: two more switches in the same bar, each carried in the address.
const owing = () => testPage.getByRole('checkbox', { name: 'Who owe money' });
const noContract = () => testPage.getByRole('checkbox', { name: 'Active, with no Contract' });
const alsoShow = () => testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' });
const nobody = jsonResponse({ items: [], hasMore: false });

describe('the "owe money" and "no Contract" switches', () => {
	it('are offered off, with nothing on the wire', async () => {
		await setup();

		await expect.element(owing()).not.toBeChecked();
		await expect.element(noContract()).not.toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('leave out the money switch for a contractor Doula, whose whole bar is hidden', async () => {
		await setup(undefined, { isContractor: true, isDoula: true });

		await expect.element(owing()).not.toBeInTheDocument();
		await expect.element(noContract()).not.toBeInTheDocument();
	});

	it('navigate to `?owes=true` and `?noContract=true` when switched on', async () => {
		await setup();

		await owing().click();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?owes=true`, noName);

		await owing().click();
		await noContract().click();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?noContract=true`, noName);
	});

	it('carry every param already on, `all` included', async () => {
		pageState.url.searchParams.set('owes', 'true');
		pageState.url.searchParams.set('all', 'true');
		await setup(undefined, { isDoula: true });

		await noContract().click();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?owes=true&noContract=true&all=true`, noName);

		await mine().click();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?mine=true&owes=true&noContract=true&all=true`, noName);

		await owing().click();
		await noContract().click();
		await mine().click();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?all=true`, noName);
	});

	it('read `?owes=true`: checked, sent to the BFF, worded, with the no-care switch left out', async () => {
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		await expect.element(owing()).toBeChecked();
		await expect.element(noContract()).not.toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?owes=true`);
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing \d+ Clients? who owes? money, soonest due first, then most recently ended\.$/);
		await expect.element(alsoShow()).not.toBeInTheDocument();
	});

	it('read `?noContract=true` and `?mine=true` together, and send all three words', async () => {
		pageState.url.searchParams.set('owes', 'true');
		pageState.url.searchParams.set('noContract', 'true');
		pageState.url.searchParams.set('mine', 'true');

		await setup(undefined, { isDoula: true });

		await expect.element(noContract()).toBeChecked();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?mine=true&owes=true&noContract=true`);
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing \d+ of your active Clients who owes? money and ha(ve|s) no Contract, soonest due first, then most recently ended\.$/);
	});

	it('ignores `?owes=true` for a contractor Doula, whom the BFF would refuse', async () => {
		pageState.url.searchParams.set('owes', 'true');

		await setup(undefined, { isContractor: true, isDoula: true });

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('says plainly that nobody owes money', async () => {
		pageState.url.searchParams.set('owes', 'true');

		await setup(nobody);

		await expect.element(testPage.getByText('No Client owes money. Clear the filters to widen the list.').first()).toBeVisible();
	});

	it('says plainly that no Client is missing a Contract', async () => {
		pageState.url.searchParams.set('noContract', 'true');

		await setup(nobody);

		await expect.element(testPage.getByText('No active Client is missing a Contract. Clear the filters to widen the list.').first()).toBeVisible();
		// H8: heard once, from the live result line; the empty state draws it for sight only.
		expect(testPage.getByRole('paragraph').filter({ hasText: 'No active Client is missing a Contract' }).elements()).toHaveLength(0);
	});

	it('keeps the no-work switch while neither narrowing is on', async () => {
		await setup();

		await expect.element(alsoShow()).toBeVisible();
	});
});

// The Doula select and the Name field, carried in the address as `doula`
// and `q` and applied together with every other filter.
const doulaField = () => testPage.getByLabelText('Doula', { exact: true });
const openFold = () => testPage.getByText('More filters and order').click();
const nameField = () => testPage.getByLabelText('Name', { exact: true });
const kimiko = { value: 'staff-kimiko', label: 'Kimiko Nakashima' };
const lena = { value: 'staff-lena', label: 'Lena Vasquez' };
const doulas = [kimiko, lena];

describe('the "Doula" select', () => {
	it('lists "Any Doula" first, then the Practice\'s Doulas, with nothing on the wire', async () => {
		await setup(undefined, { doulas });

		await expect.element(doulaField()).toHaveValue('');
		await expect.element(doulaField().getByRole('option').first()).toHaveTextContent('Any Doula');
		await expect.element(doulaField().getByRole('option', { name: kimiko.label })).toBeInTheDocument();
		await expect.element(doulaField().getByRole('option', { name: lena.label })).toBeInTheDocument();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
		await expect.element(doulaField()).toHaveAttribute('name', 'doula');
	});

	it('is not drawn when the Practice\'s Doulas could not be read, as for a Doula who may not read the roster', async () => {
		await setup();

		await expect.element(doulaField()).not.toBeInTheDocument();
	});

	it('is not drawn for a contractor Doula, whose whole bar is hidden', async () => {
		await setup(undefined, { isContractor: true, isDoula: true, doulas });

		await expect.element(doulaField()).not.toBeInTheDocument();
	});

	it('navigates to `?doula=<id>` once chosen and applied, and back when "Any Doula" is chosen', async () => {
		await setup(undefined, { doulas });

		await doulaField().selectOptions(kimiko.label);
		expect(goto).not.toHaveBeenCalled();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?doula=${kimiko.value}`, noName);

		await doulaField().selectOptions('Any Doula');
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients`, noName);
	});

	it('reads `?doula=<id>`: selected, sent to the BFF, and worded as hers', async () => {
		pageState.url.searchParams.set('doula', kimiko.value);

		await setup(undefined, { doulas });

		await expect.element(doulaField()).toHaveValue(kimiko.value);
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?doula=${kimiko.value}`);
		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing Kimiko Nakashima's \d+ Clients?, soonest due first, then most recently ended\.$/);
	});

	it('says "that Doula" until the roster names her', async () => {
		pageState.url.searchParams.set('doula', 'staff-gone');

		await setup(undefined, { doulas });

		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing that Doula's \d+ Clients?/);
	});

	it('says plainly that nobody matched, and how to widen the list', async () => {
		pageState.url.searchParams.set('doula', kimiko.value);

		await setup(nobody, { doulas });

		await expect
			.element(testPage.getByText('Kimiko Nakashima has no Clients. Clear the filters to widen the list.').first())
			.toBeVisible();
	});

	// The BFF ANDs the filters, so mine and a Doula is a narrower list. The
	// select stays drawn, so Apply never moves when "Mine" is ticked.
	it('stays drawn and is sent with "Mine" while both are on', async () => {
		pageState.url.searchParams.set('mine', 'true');
		pageState.url.searchParams.set('doula', kimiko.value);

		await setup(undefined, { isDoula: true, doulas });

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?mine=true&doula=${kimiko.value}`);
		await expect.element(doulaField()).toHaveValue(kimiko.value);
	});

	it('carries a chosen Doula into the address with "Mine"', async () => {
		await setup(undefined, { isDoula: true, doulas });

		await doulaField().selectOptions(kimiko.label);
		await mine().click();
		await applyFilters();

		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?mine=true&doula=${kimiko.value}`, noName);
	});
});

const orderField = () => testPage.getByLabelText('Order', { exact: true });

describe('the "Order" select (#2869)', () => {
	const base = `/practices/${practiceId}/clients`;

	it('offers the list\'s own order first, then the name orders and most owed, with nothing on the wire', async () => {
		await setup();

		await expect.element(orderField()).toHaveValue('');
		await expect.element(orderField()).toHaveAttribute('name', 'order');
		const options = orderField().getByRole('option');
		await expect.element(options.nth(0)).toHaveTextContent('Soonest due first');
		await expect.element(options.nth(1)).toHaveTextContent('Name, A to Z');
		await expect.element(options.nth(2)).toHaveTextContent('Name, Z to A');
		await expect.element(options.nth(3)).toHaveTextContent('Most owed first');
		await expect.element(options.nth(4)).toHaveTextContent('Most overdue first');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('is not drawn for a contractor Doula, whose whole bar is hidden, and her address never sends "owed"', async () => {
		pageState.url.searchParams.set('order', 'owed');

		await setup(undefined, { isContractor: true, isDoula: true });

		await expect.element(orderField()).not.toBeInTheDocument();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
		pageState.url.searchParams.delete('order');
	});

	it('navigates to `?order=` once chosen and applied, and back when the list\'s own is chosen', async () => {
		await setup();

		await orderField().selectOptions('Name, A to Z');
		expect(goto).not.toHaveBeenCalled();
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`${base}?order=name`, noName);

		await orderField().selectOptions('Most owed first');
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`${base}?order=owed`, noName);

		await orderField().selectOptions('Soonest due first');
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(base, noName);
	});

	it('travels with every other filter, last in the address', async () => {
		await setup(undefined, { doulas });

		await orderField().selectOptions('Name, Z to A');
		await doulaField().selectOptions(lena.label);
		await owing().click();
		await applyFilters();

		expect(goto).toHaveBeenLastCalledWith(`${base}?owes=true&doula=${lena.value}&order=name-desc`, noName);
	});

	it('clears a held Name search the reader emptied without an input event, on Apply', async () => {
		historyState.clientsName = 'Camille';

		await setup();

		await expect.element(nameField()).toHaveValue('Camille');
		(nameField().element() as HTMLInputElement).value = '';
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(base, noName);
	});

	it('is kept by a Name search, which rides in the history entry and sends the order in its body', async () => {
		pageState.url.searchParams.set('order', 'name');
		historyState.clientsName = 'Camille';

		await setup();

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients/list`);
		expect(JSON.parse(requestInit().body as string)).toMatchObject({ name: 'Camille', order: 'name' });
		await nameField().fill('Cam');
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`${base}?order=name`, { state: { clientsName: 'Cam' } });
		pageState.url.searchParams.delete('order');
	});

	it.each([
		['name', 'Name, A to Z', 'Name', 'ascending', 'by name, A to Z'],
		['name-desc', 'Name, Z to A', 'Name', 'descending', 'by name, Z to A'],
		['owed', 'Most owed first', 'Money', 'descending', 'most owed first'],
		['overdue', 'Most overdue first', 'Money', 'descending', 'most overdue first'],
		['', 'Soonest due first', 'Date', 'ascending', 'soonest due first, then most recently ended']
	])('reads `?order=%s`: selected, sent to the BFF, sorted on %s %s and worded "%s"', async (order, label, column, direction, words) => {
		if (order) pageState.url.searchParams.set('order', order);

		await setup();

		await expect.element(orderField()).toHaveValue(order);
		expect(requestUrl()).toBe(order ? `/api/practices/${practiceId}/clients?order=${order}` : `/api/practices/${practiceId}/clients`);
		await expect.element(testPage.getByRole('columnheader', { name: column })).toHaveAttribute('aria-sort', direction);
		expect(testPage.getByRole('columnheader').elements().filter((header) => header.hasAttribute('aria-sort'))).toHaveLength(1);
		await expect.element(status()).toHaveTextContent(new RegExp(String.raw`, ${words}\.$`));
		pageState.url.searchParams.delete('order');
		expect(label).not.toBe('');
	});

	it('reads an order the list does not offer as its own', async () => {
		pageState.url.searchParams.set('order', 'newest');

		await setup();

		await expect.element(orderField()).toHaveValue('');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
		await expect.element(tags()).not.toBeInTheDocument();
		pageState.url.searchParams.delete('order');
	});

	it('draws no tag for the order: its filter\'s tag keeps the order, and "Clear filters" is drawn and removes both', async () => {
		pageState.url.searchParams.set('order', 'name');
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		await expect.element(tags().getByRole('link', { name: 'Remove filter: Order Name, A to Z' })).not.toBeInTheDocument();
		await expect
			.element(tags().getByRole('link', { name: 'Remove filter: Who owe money' }))
			.toHaveAttribute('href', `${base}?order=name`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', base);
		pageState.url.searchParams.delete('order');
		pageState.url.searchParams.delete('owes');
	});

	it('draws no tag for a bare order, and "Clear filters" returns to the default order', async () => {
		pageState.url.searchParams.set('order', 'owed');

		await setup();

		await expect.element(tags().getByRole('link', { name: 'Remove filter: Order Most owed first' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', base);
		pageState.url.searchParams.delete('order');
	});

	it('goes back to what the address says when another tag is removed, and Apply sends what the select shows', async () => {
		pageState.url.searchParams.set('owes', 'true');
		await setup();

		await orderField().selectOptions('Name, A to Z');
		await tags().getByRole('link', { name: 'Remove filter: Who owe money' }).click();
		pageState.url.searchParams.delete('owes');
		navigation.onBefore();
		navigation.onMove?.({});

		await expect.element(orderField()).toHaveValue('');
		await applyFilters();
		expect(String(goto.mock.lastCall![0])).not.toContain('order=');
	});

	it('is sent again by "Load more", whose cursor belongs to it', async () => {
		pageState.url.searchParams.set('order', 'name');
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' }));
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ items: [clients[1]], hasMore: false }));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('rowheader', { name: clients[1].name })).toBeVisible();
		expect(requestUrl(1)).toBe(`/api/practices/${practiceId}/clients?order=name&cursor=cursor-1`);
		pageState.url.searchParams.delete('order');
	});
});

// A phone's first screen is the list, not the bar: the filters fold behind
// one summary that counts what is on, and open when any is.
describe('the filter bar on a phone', () => {
	it('keeps the checkboxes, the Name and Apply in view, and folds the Doula select, "Also show" and Order behind one summary', async () => {
		await setup(undefined, { doulas });
		await testPage.viewport(414, 900);

		await expect.element(testPage.getByText('More filters and order')).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Who owe money' })).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Active, with no Contract' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(nameField()).toBeVisible();
		await expect.element(doulaField()).not.toBeVisible();
		await expect.element(testPage.getByLabelText('Order', { exact: true })).not.toBeVisible();

		await openFold();

		await expect.element(doulaField()).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Order', { exact: true })).toBeVisible();
	});

	it.each([375, 320])('holds the opened fold inside the window at %ipx', async (width) => {
		await setup(undefined, { doulas });
		await testPage.viewport(width, 900);
		await openFold();

		const [doula, order] = [rectOf(doulaField()), rectOf(testPage.getByLabelText('Order', { exact: true }))];
		expect(doula.right).toBeLessThanOrEqual(width);
		expect(order.right).toBeLessThanOrEqual(width);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(document.documentElement.clientWidth);
	});

	it('draws the Name alone where there is no Doula select', async () => {
		await setup();
		await testPage.viewport(414, 900);

		await expect.element(doulaField()).not.toBeInTheDocument();
		await expect.element(nameField()).toBeVisible();
	});

	it('shows the Doula select beside the Name in a wide column too', async () => {
		await setup(undefined, { doulas });

		await expect.element(nameField()).toBeVisible();
		await expect.element(doulaField()).toBeVisible();
		const [name, doula] = [rectOf(nameField()), rectOf(doulaField())];
		expect(Math.abs(name.top - doula.top)).toBeLessThanOrEqual(1);
	});
});

// The result line is the table's description, so a screen reader reaching
// the table hears the count and the order.
describe('the table and its result line', () => {
	it('describes the table by the filter bar\'s result line', async () => {
		await setup();
		await waitForRows();

		await expect
			.element(testPage.getByRole('table', { name: 'Clients' }))
			.toHaveAccessibleDescription(/^Showing \d+ Clients?, soonest due first, then most recently ended\.$/);
	});

	it('lets a keyboard user skip the filters to the result line', async () => {
		await setup();
		await waitForRows();
		testPage.getByRole('link', { name: 'Find or add a Client' }).element().focus();
		await userEvent.keyboard('{Tab}');

		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).toHaveFocus();
		await userEvent.keyboard('{Tab}');

		await expect.element(testPage.getByRole('link', { name: 'Skip to results' })).toHaveFocus();
		await userEvent.keyboard('{Enter}');

		await expect.element(testPage.getByRole('status')).toHaveFocus();
	});
});

describe('the "Name" field', () => {
	it('is a search field that holds up to a hundred characters, empty with nothing on the wire', async () => {
		await setup();

		await expect.element(nameField()).toHaveValue('');
		await expect.element(nameField()).toHaveAttribute('maxlength', '100');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('does not move the list while it is typed in, only when applied', async () => {
		await setup();

		await nameField().fill('Camille');
		expect(goto).not.toHaveBeenCalled();
		await applyFilters();

		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients`, { state: { clientsName: 'Camille' } });
	});

	// #1829: the name rides in the history entry, so no address the page
	// navigates to or asks the BFF for holds it -- the one BFF request that
	// carries it is a POST with it in the body.
	it('is in no address the page navigates to or asks the BFF for', async () => {
		historyState.clientsName = 'Anne & Marie 100%';
		await setup();
		await waitForRows();

		await owing().click();
		await applyFilters();

		expectNoNameInAnyAddress('Anne');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients/list`);
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients?owes=true`, {
			state: { clientsName: 'Anne & Marie 100%' }
		});
	});

	it('is applied from the keyboard with Enter, as the form submits', async () => {
		await setup();

		await nameField().fill('Camille');
		await testPage.getByRole('searchbox', { name: 'Name' }).click();
		await userEvent.keyboard('{Enter}');

		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients`, { state: { clientsName: 'Camille' } });
	});

	it('trims the text, and a blank one clears it', async () => {
		await setup();

		await nameField().fill('  Anne & Marie 100%  ');
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients`, {
			state: { clientsName: 'Anne & Marie 100%' }
		});

		await nameField().fill(' '.repeat(3));
		await applyFilters();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/clients`, noName);
	});

	it('combines with every other filter, `all` included', async () => {
		pageState.url.searchParams.set('all', 'true');
		await setup(undefined, { doulas });

		await nameField().fill('Cam');
		await doulaField().selectOptions(lena.label);
		await owing().click();
		await noContract().click();
		await applyFilters();

		expect(goto).toHaveBeenLastCalledWith(
			`/practices/${practiceId}/clients?owes=true&noContract=true&doula=${lena.value}&all=true`,
			{ state: { clientsName: 'Cam' } }
		);
	});

	it('reads the history entry: filled in, sent to the BFF in a body, and worded, with "Also show anyone not yet in care" still offered', async () => {
		historyState.clientsName = ' Camille ';

		await setup();

		await expect.element(nameField()).toHaveValue('Camille');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients/list`);
		expect(requestInit().method).toBe('POST');
		expect(JSON.parse(requestInit().body as string)).toMatchObject({ name: 'Camille' });
		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing \d+ Clients? named "Camille", soonest due first, then most recently ended\.$/);
		await expect.element(alsoShow()).toBeVisible();
	});

	it('says no Client is named so, and offers the switch for the rest', async () => {
		historyState.clientsName = 'Zelda';

		await setup(nobody);

		await expect
			.element(
				testPage
					.getByText('No Client is named "Zelda". Turn on "Also show anyone not yet in care" to look among them.')
					.first()
			)
			.toBeVisible();
	});

	it('says no Client is named so, and how to widen the list, once the rest are in the list', async () => {
		historyState.clientsName = 'Zelda';
		pageState.url.searchParams.set('all', 'true');

		await setup(nobody);

		await expect
			.element(testPage.getByText('No Client is named "Zelda". Clear the filters to widen the list.').first())
			.toBeVisible();
	});

	it('words a name beside "Who owe money"', async () => {
		historyState.clientsName = 'Cam';
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing \d+ Clients? named "Cam" who owes? money, soonest due first, then most recently ended\.$/);
	});
});

type Located = { element: () => Element };
const top = (locator: Located) => locator.element().getBoundingClientRect().top;
const isFollowing = (before: Located, after: Located) =>
	Boolean(before.element().compareDocumentPosition(after.element()) & Node.DOCUMENT_POSITION_FOLLOWING);

describe('the filter bar\'s layout', () => {
	// Two rows that do not depend on each other: the switches, then the
	// fields (Name, Doula, "Mine"), Apply and "Clear filters". Order, not pixels, is asserted.
	it('puts the switches above the fields, Name before Doula, Apply then "Clear filters" last', async () => {
		historyState.clientsName = 'Camille';
		pageState.url.searchParams.set('all', 'true');
		await setup(undefined, { isDoula: true, doulas });

		const switchBoxes = [
			testPage.getByRole('checkbox', { name: 'Who owe money' }),
			noContract(),
			testPage.getByRole('checkbox', { name: 'Mine' }),
			testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' })
		];
		for (const box of switchBoxes) {
			expect(top(box)).toBeLessThan(top(nameField()));
		}
		const apply = testPage.getByRole('button', { name: 'Apply' });
		const clear = testPage.getByRole('link', { name: 'Clear filters' });
		expect(isFollowing(nameField(), doulaField())).toBe(true);
		expect(isFollowing(doulaField(), apply)).toBe(true);
		expect(isFollowing(apply, clear)).toBe(true);
	});

	it('keeps "Also show anyone not yet in care" last of the switches', async () => {
		await setup(undefined, { isDoula: true });

		const names = testPage
			.getByRole('checkbox')
			.elements()
			.map((box) => box.getAttribute('name'));
		expect(names).toEqual(['owes', 'noContract', 'mine', 'all']);
	});

	it('groups the narrowing boxes under the visible legend "Show only", and the box that widens outside it', async () => {
		await setup(undefined, { isDoula: true });

		const group = testPage.getByRole('group', { name: 'Show only' });
		await expect.element(group).toBeVisible();
		await expect.element(group.getByRole('checkbox', { name: 'Who owe money' })).toBeVisible();
		await expect.element(group.getByRole('checkbox', { name: 'Active, with no Contract' })).toBeVisible();
		await expect.element(group.getByRole('checkbox', { name: 'Mine' })).toBeVisible();
		await expect.element(group.getByRole('checkbox', { name: 'Also show anyone not yet in care' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' })).toBeVisible();
		await expect.element(testPage.getByText('Show only', { exact: true })).toBeVisible();
	});

	it('says "Clients" in none of the filter controls', async () => {
		await setup(undefined, { isDoula: true, doulas });

		const bar = testPage.getByRole('group', { name: 'Choose which Clients to show' });
		for (const box of bar.getByRole('checkbox').elements()) {
			const label = (box as HTMLInputElement).labels![0]!;
			expect(label.textContent).not.toMatch(/Clients?/);
		}
	});

	it('draws the legend on the boxes’ own row, so the group takes no row of its own', async () => {
		await setup(undefined, { isDoula: true });

		const legend = testPage.getByText('Show only', { exact: true }).element().getBoundingClientRect();
		const owes = testPage.getByRole('checkbox', { name: 'Who owe money' }).element().getBoundingClientRect();
		expect(Math.abs(legend.top + legend.height / 2 - (owes.top + owes.height / 2))).toBeLessThanOrEqual(8);
		expect(legend.right).toBeLessThanOrEqual(owes.left);
	});
});

// GOV.UK's "selected filters": each applied filter is a tag above the count
// line, a link to the list minus that one filter.
const tags = () => testPage.getByRole('list', { name: 'Applied filters' });

describe('the applied filters', () => {
	const base = `/practices/${practiceId}/clients`;

	it('draws no list of tags while the address names no filter and no name is applied', async () => {
		await setup(undefined, { isDoula: true, doulas });

		await expect.element(tags()).not.toBeInTheDocument();
	});

	it.each([
		['owes', 'Remove filter: Who owe money', 'Who owe money'],
		['noContract', 'Remove filter: Active, with no Contract', 'Active, with no Contract'],
		['mine', 'Remove filter: Mine', 'Mine'],
		['all', 'Remove filter: Including anyone not yet in care', 'Including anyone not yet in care'],
		['doula', 'Remove filter: Doula Kimiko Nakashima', 'Doula: Kimiko Nakashima']
	])('draws a tag for `%s`, named for what pressing it does, leading to the list without it', async (key, name, shown) => {
		pageState.url.searchParams.set(key, key === 'doula' ? kimiko.value : 'true');

		await setup(undefined, { isDoula: true, doulas });

		const tag = tags().getByRole('link', { name });
		await expect.element(tag).toBeVisible();
		await expect.element(tag).toHaveTextContent(shown);
		await expect.element(tag).toHaveAttribute('href', base);
	});

	it('draws one tag for the name, which is in no address', async () => {
		historyState.clientsName = 'Camille';

		await setup();

		const tag = tags().getByRole('link', { name: 'Remove filter: Name Camille' });
		await expect.element(tag).toHaveTextContent('Name: Camille');
		await expect.element(tag).toHaveAttribute('href', base);
	});

	it('leads each tag to the list with the other filters still on', async () => {
		pageState.url.searchParams.set('owes', 'true');
		pageState.url.searchParams.set('noContract', 'true');
		pageState.url.searchParams.set('doula', kimiko.value);

		await setup(undefined, { isDoula: true, doulas });

		await expect.element(tags().getByRole('link', { name: 'Remove filter: Who owe money' })).toHaveAttribute('href', `${base}?noContract=true&doula=${kimiko.value}`);
		await expect.element(tags().getByRole('link', { name: 'Remove filter: Active, with no Contract' })).toHaveAttribute('href', `${base}?owes=true&doula=${kimiko.value}`);
		await expect.element(tags().getByRole('link', { name: 'Remove filter: Doula Kimiko Nakashima' })).toHaveAttribute('href', `${base}?owes=true&noContract=true`);
		expect(tags().getByRole('link').elements()).toHaveLength(3);
	});

	it('removes one filter with one press, handing the typed name on in the history entry', async () => {
		pageState.url.searchParams.set('owes', 'true');
		pageState.url.searchParams.set('noContract', 'true');
		historyState.clientsName = 'Camille';

		await setup();
		await tags().getByRole('link', { name: 'Remove filter: Who owe money' }).click();

		expect(goto).toHaveBeenCalledTimes(1);
		expect(goto).toHaveBeenCalledWith(`${base}?noContract=true`, { state: { clientsName: 'Camille' } });
		expectNoNameInAnyAddress('Camille');
	});

	it('removes the name alone, clearing it from the history entry and keeping every filter in the address', async () => {
		pageState.url.searchParams.set('owes', 'true');
		historyState.clientsName = 'Camille';

		await setup();
		await tags().getByRole('link', { name: 'Remove filter: Name Camille' }).click();

		expect(goto).toHaveBeenCalledTimes(1);
		expect(goto).toHaveBeenCalledWith(`${base}?owes=true`, noName);
		expectNoNameInAnyAddress('Camille');
	});

	it('leaves "Clear filters" beside Apply, which removes every filter at once', async () => {
		pageState.url.searchParams.set('owes', 'true');
		pageState.url.searchParams.set('doula', kimiko.value);

		await setup(undefined, { isDoula: true, doulas });

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', base);
	});

	it('draws the tags above the count line and below the controls', async () => {
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		const tag = tags().getByRole('link', { name: 'Remove filter: Who owe money' });
		expect(isFollowing(testPage.getByRole('button', { name: 'Apply' }), tag)).toBe(true);
		expect(isFollowing(tag, testPage.getByRole('status'))).toBe(true);
	});

	describe('once the list for the smaller address has loaded', () => {
		const plainUrl = pageState.url;
		beforeEach(() => {
			pageState.url = new SvelteURL(plainUrl);
		});
		afterEach(() => {
			pageState.url = plainUrl;
		});

		it('puts focus on the count line, as Apply does, and drops the tag', async () => {
			pageState.url.searchParams.set('owes', 'true');
			await setup();
			navigation.onMove!({});
			await waitForRows();
			const tag = tags().getByRole('link', { name: 'Remove filter: Who owe money' });
			await tag.click();

			pageState.url.searchParams.delete('owes');
			navigation.onMove!({ from: { route: listRoute, params: {} } });

			await expect.element(testPage.getByRole('status')).toHaveFocus();
			await expect.element(tags()).not.toBeInTheDocument();
		});
	});
});

describe('"Clear filters"', () => {
	it('is not drawn while the address names no filter', async () => {
		await setup();

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it.each([
		['all', 'true'],
		['mine', 'true'],
		['owes', 'true'],
		['noContract', 'true'],
		['doula', 'staff-kimiko']
	])('is not drawn beside the one tag for `%s`, whose × leads to the bare Clients list', async (name, value) => {
		pageState.url.searchParams.set(name, value);

		await setup(undefined, { isDoula: true, doulas });

		await expect
			.element(testPage.getByRole('link', { name: /^Remove filter: / }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it('is not drawn beside an applied name alone, whose × leads to the bare Clients list', async () => {
		historyState.clientsName = 'Camille';

		await setup();

		await expect
			.element(testPage.getByRole('link', { name: /^Remove filter: Name/ }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it('is drawn beside two tags, and leads to the bare Clients list', async () => {
		historyState.clientsName = 'Camille';
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Clear filters' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients`);
	});

	it('is not drawn for a filter typed but not yet applied', async () => {
		await setup();

		await nameField().fill('Camille');

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});
});

describe('the table\'s caption', () => {
	it('names the table by its purpose and leaves the count to the status line', async () => {
		historyState.clientsName = 'Camille';

		await setup();

		await expect
			.element(testPage.getByRole('table', { name: 'Clients', exact: true }))
			.toBeVisible();
	});
});

// #264 (RA-G6), #2022: the Status column -- each open Engagement as its
// kind's link and a badge per fact, then a pending Request. Scoped to the
// table: the record view draws the same cell, hidden at this width.
function table() {
	return testPage.getByRole('table');
}

describe('the Status column (#264, #2022)', () => {
	it('lists Name and Status, and no longer Email, Portal invite or Pending request', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Status' })).toBeVisible();
		for (const gone of ['Email', 'Engagements', 'Portal invite', 'Pending request']) {
			await expect.element(testPage.getByRole('columnheader', { name: gone })).not.toBeInTheDocument();
		}
	});

	it('shows each open Engagement as its own line, dropping none', async () => {
		// The fixture's first Client carries three open Engagements (#596:
		// one fully populated, one fully absent; #741 a settled Contract).
		await setup();

		await expect.element(table().getByText('Contract sent')).toBeVisible();
		await expect.element(table().getByText('Intake', { exact: true })).toBeVisible();
		await expect.element(table().getByText('No contract yet').first()).toBeVisible();
	});

	// #2232 (RA-G15): who covers each Engagement, on the list, so seeing
	// it takes no Engagement opened.
	it('names the Doula of each open Engagement in a Doula column, by kind where there are several', async () => {
		// The fixture's first Client holds three open Engagements: two
		// Doulas on the first, none on the second.
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Doula' })).toBeVisible();
		await expect.element(table().getByText('Birth: Yolanda Okonkwo-Fitzgerald, Anne-Marie Ochieng-Whitfield', { exact: true }).last()).toBeVisible();
		await expect.element(table().getByText('Postpartum: No Doula yet').last()).toBeVisible();
	});

	it('names her alone where the Client holds one open Engagement, keeping a comma inside her name', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], openEngagements: [{ ...line, doulaNames: ['Priya Raman, Jr.'] }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: 'Priya Raman, Jr.', exact: true })).toBeVisible();
	});

	it('shows the date of each open Engagement in a Date column, by kind where there are several', async () => {
		// The fixture's first Client holds three open Engagements: the first
		// and third dated, the second not.
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Date' })).toBeVisible();
		await expect.element(table().getByText('Birth: Due Jan 10, 2027')).toBeVisible();
		await expect.element(table().getByText('Postpartum: —').first()).toBeVisible();
		await expect.element(table().getByText('Birth: Due Feb 21, 2027')).toBeVisible();
	});

	it('shows the date alone where the Client holds one open Engagement', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], openEngagements: [line] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: 'Due Jan 10, 2027', exact: true })).toBeVisible();
	});

	// Beside a red Overdue badge, the words "No due date" on screen read as
	// being about the invoice, so the cell draws a dash and a screen reader
	// hears the words.
	it('draws a dash and hides "No due date" for a lone Engagement with none on file', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], openEngagements: [{ ...line, dueDate: undefined }] }],
				hasMore: false
			})
		);

		const cell = table().getByRole('cell', { name: 'No due date', exact: true });
		await expect.element(cell).toBeVisible();
		await expect.element(cell).toHaveTextContent('—No due date');
		await expect.element(table().getByText('No due date', { exact: true })).toHaveClass('visually-hidden');
	});

	// The fixture's fourth Client holds one line of each ended state, which
	// the continuum sweep measures.
	it('reads a Client whose pregnancies ended as weeks postpartum for a birth and "Ended" for a loss, by kind', async () => {
		await setup();

		await expect.element(table().getByText('Postpartum: 26 weeks postpartum')).toBeVisible();
		await expect.element(table().getByText('Birth: Ended Nov 30, 2026')).toBeVisible();
	});

	// A postpartum line has no due date; how long ago the birth was stands
	// in its place, and the day the pregnancy ended for a loss, in words that
	// never say "postpartum" (docs/personas/loss-client.md).
	describe('with the day pinned to Nov 20, 2026', () => {
		beforeEach(() => {
			vi.setSystemTime(new Date('2026-11-20T17:00:00Z'));
		});

		afterEach(() => {
			vi.useRealTimers();
		});

		it.each([
			['live_birth', '2026-10-30', '3 weeks postpartum, born Oct 30, 2026'],
			['live_birth', '2026-10-18', '4 weeks postpartum, born Oct 18, 2026'],
			['live_birth', '2026-11-15', '5 days postpartum, born Nov 15, 2026'],
			['live_birth', '2026-11-06', '2 weeks postpartum, born Nov 6, 2026'],
			['loss', '2026-10-30', 'Ended Oct 30, 2026'],
			['loss', '2026-11-15', 'Ended Nov 15, 2026'],
			['unknown', '2026-10-30', 'Ended Oct 30, 2026']
		])('reads a %s outcome on %s as "%s" and not the due date', async (birthOutcome, endedOn, text) => {
			const [line] = clients[0].openEngagements ?? [];
			await setup(
				jsonResponse({
					items: [{ ...clients[0], openEngagements: [{ ...line, birthOutcome, pregnancyEndedOn: endedOn }] }],
					hasMore: false
				})
			);

			// By its words, not its accessible name: a `time` element's own words are not read into a cell's name.
			await expect.element(table().getByRole('cell').filter({ hasText: text })).toBeVisible();
			const row = table().getByRole('row').nth(1);
			if (birthOutcome === 'live_birth') {
				await expect.element(row.getByText('Ended')).not.toBeInTheDocument();
			} else {
				await expect.element(row).not.toHaveTextContent(/postpartum/);
			}
		});

		it('keeps the birth day for a screen reader in a time element, said after the weeks', async () => {
			const [line] = clients[0].openEngagements ?? [];
			await setup(
				jsonResponse({
					items: [{ ...clients[0], openEngagements: [{ ...line, birthOutcome: 'live_birth', pregnancyEndedOn: '2026-10-30' }] }],
					hasMore: false
				})
			);

			const born = table().getByText('3 weeks postpartum').element();
			expect(born.tagName).toBe('TIME');
			expect(born.getAttribute('datetime')).toBe('2026-10-30');
			await expect.element(born.querySelector('.visually-hidden') as HTMLElement).toHaveTextContent(', born Oct 30, 2026');
		});
	});

	it('does not draw a loss’s Client as postpartum, whatever her Engagement’s kind', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], openEngagements: [{ ...line, kind: 'postpartum', birthOutcome: 'loss', pregnancyEndedOn: '2026-10-30' }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: 'Ended Oct 30, 2026', exact: true })).toBeVisible();
		await expect.element(table().getByText(/weeks postpartum|days? postpartum/)).not.toBeInTheDocument();
	});

	it('keeps the due date while the outcome is recorded with no day, and the dash when there is neither', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [
					{ ...clients[0], openEngagements: [{ ...line, birthOutcome: 'unknown' }] },
					{
						...clients[1],
						openEngagements: [{ ...line, engagementId: 'e-none', dueDate: undefined, birthOutcome: 'unknown' }]
					}
				],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: 'Due Jan 10, 2027', exact: true })).toBeVisible();
		await expect.element(table().getByRole('cell', { name: 'No due date', exact: true })).toBeVisible();
	});

	it('reads "overdue" for an Invoice one day late, as for one twelve days late', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], oldestOverdueDays: 1, openEngagements: [{ ...line, daysOverdue: 1 }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: /^\$4,500\.00\s*,\s*1 day overdue\s*,\s*1 Invoice$/ })).toBeVisible();
	});

	it('reads the total in the Money column, whatever the late Invoice carries for an amount', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], openEngagements: [{ ...line, invoiceAmountCents: undefined }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: /^\$4,500\.00\s*,\s*12 days overdue\s*,\s*1 Invoice$/ })).toBeVisible();
	});

	// #2130: the BFF's one count (#2122) decides it, so a late Invoice
	// reads Overdue here as on its Engagement and on Invoices.
	it('reads "Overdue" once the BFF counts the open Invoice late', async () => {
		await setup();

		await expect.element(table().getByRole('cell', { name: /\$4,500\.00\s*,\s*12 days overdue\s*,\s*1 Invoice/ })).toBeVisible();
		await expect.element(table().getByText('Owes', { exact: true })).not.toBeInTheDocument();
	});

	// #2225: she has paid and the money is on its way, so it is neither late
	// nor owed.
	it('reads "payment on its way" in the Money column while a payment settles, even on a late Invoice', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], owedCents: 0, owedInvoiceCount: 0, openEngagements: [{ ...line, paymentProcessing: true }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: /^\$4,500\.00\s*,\s*payment on its way$/ })).toBeVisible();
		await expect.element(table().getByText(/overdue|Owes/i)).not.toBeInTheDocument();
	});

	it('reads "owed" in the Money column for an open Invoice that is not late, and says nothing of it in Status', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[0], oldestOverdueDays: 0, openEngagements: [{ ...line, daysOverdue: 0 }] }],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: /^\$4,500\.00\s*,\s*owed\s*,\s*1 Invoice$/ })).toBeVisible();
		await expect.element(table().getByText('Owes', { exact: true })).not.toBeInTheDocument();
		await expect.element(table().getByText(/overdue/i)).not.toBeInTheDocument();
	});

	// #741: `open` (billed, unpaid) is what "is anything outstanding"
	// means; a settled Contract's `paid` reads as paid, with nothing to do.
	it('reads "Paid" rather than "Owes" once nothing is outstanding', async () => {
		const settled = clients[0].openEngagements?.at(2);
		await setup(jsonResponse({ items: [{ ...clients[0], owedCents: 0, owedInvoiceCount: 0, openEngagements: [settled] }], hasMore: false }));

		await expect.element(table().getByText('Owes', { exact: true })).not.toBeInTheDocument();
		await expect.element(table().getByText(/Paid/)).not.toBeInTheDocument();
		await expect.element(table().getByText(/\$1,000\.00\s*,\s*paid/)).toBeVisible();
	});

	it('names any other Invoice status in its own word', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						...clients[1],
						openEngagements: [
							{
								engagementId: 'engagement-void',
								kind: 'birth',
								engagementStatus: 'active',
								contractStatus: 'voided',
								invoiceStatus: 'void'
							}
						]
					}
				],
				hasMore: false
			})
		);

		await expect.element(table().getByText('Contract voided')).toBeVisible();
		await expect.element(table().getByText('Invoice void')).not.toBeInTheDocument();
		await expect.element(table().getByRole('cell', { name: /void$/ })).toBeVisible();
	});

	// #1511's way to a new Engagement is its link on the Client hub, so a
	// row here holds one link, to the Client, and the kind is a word.
	it("links only the Client's name in a row, not each line's kind", async () => {
		await setup();

		await expect.element(table().getByText('Postpartum', { exact: true }).first()).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Postpartum', exact: true })).not.toBeInTheDocument();
	});

	it('says the care phase and the Contract in Status, and leaves the money to its own column', async () => {
		await setup();

		const lines = table().getByRole('listitem');

		await expect.element(lines.nth(0)).toHaveTextContent(/^\s*Birth\s*,\s*Active,\s*·\s*Contract sent\s*$/);
		await expect.element(table().getByRole('row').nth(1)).toHaveTextContent(/overdue/);
	});

	// The "Active, with no Contract" filter carries the job of
	// finding them, so the line states it as a plain fact in every phase and
	// the warning color is left for owed money and a pending Request.
	it('reads No Contract yet as a plain fact in every care phase, never a badge', async () => {
		await setup();

		const lines = table().getByRole('listitem');

		await expect.element(lines.nth(1)).toHaveTextContent(/^\s*Postpartum\s*,\s*Intake,\s*·\s*No Contract yet\s*$/);
		await expect.element(lines.nth(2)).toHaveTextContent(/^\s*Birth\s*,\s*Active,\s*·\s*No Contract yet\s*$/);
	});

	it('says "Care ended" for a Client whose Engagements have all ended', async () => {
		await setup(jsonResponse({ items: [{ ...clients[1], hasWork: true }], hasMore: false }));

		await expect.element(table().getByText('Care ended')).toBeVisible();
	});

	it('draws nothing for a Client with no open Engagement and no pending Request', async () => {
		// The fixture's second Client carries neither.
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));

		await expect.element(testPage.getByRole('columnheader', { name: 'Status' })).toBeVisible();
		await expect.element(table().getByRole('listitem')).not.toBeInTheDocument();
	});

	it('shows her own fee on a line that carries one, never Invoice money', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						...clients[1],
						openEngagements: [
							{ engagementId: 'engagement-fee', kind: 'birth', engagementStatus: 'active', feeCents: 120_000 }
						]
					}
				],
				hasMore: false
			})
		);

		await expect.element(table().getByRole('cell', { name: 'Your fee $1,200.00', exact: true })).toBeVisible();
		await expect.element(table().getByText(/Owes|owed|paid/)).not.toBeInTheDocument();
	});

	// #499: a pending Engagement Request shows on its Client's row, folded
	// into Status (#2022).
	it('shows a pending Request, joining both kinds when she holds one of each', async () => {
		// The fixture's third Client holds a Request of each kind.
		await setup();

		await expect.element(table().getByText('Birth request pending')).toBeVisible();
		await expect.element(table().getByText('Birth & Postpartum request pending')).toBeVisible();
	});
});

// The Money column: the amount of each open Engagement's line and its state, or a
// contractor's own fee, wherever the BFF let the field through (ADR-0006/0008).
const isAfter = (first: Element, second: Element) => Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING);

describe('the Money column', () => {
	const [first] = clients[0].openEngagements ?? [];
	// The BFF sums the open Invoices with no payment on their way for every row (#2874), so a settled line owes nothing.
	const only = (line: object) => {
		const merged = { ...first, ...line };
		const isOwed = merged.invoiceStatus === 'open' && !merged.paymentProcessing;
		return jsonResponse({
			items: [{ ...clients[0], owedCents: isOwed ? 450_000 : 0, owedInvoiceCount: isOwed ? 1 : 0, oldestOverdueDays: isOwed ? (merged.daysOverdue ?? 0) : 0, openEngagements: [merged] }],
			hasMore: false
		});
	};

	it('follows Status, and is right-aligned like the Date', async () => {
		await setup();

		const headers = testPage.getByRole('columnheader').elements().map((header) => header.textContent);
		expect(headers).toEqual(['Name', 'Status', 'Money', 'Date', 'Doula']);
		await expect.element(testPage.getByRole('columnheader', { name: 'Money' })).toHaveClass('numeric');
	});

	it.each([
		['an open Invoice', { daysOverdue: 0 }, String.raw`owed\s*,\s*1 Invoice`],
		['a late Invoice', { daysOverdue: 12 }, String.raw`12 days overdue\s*,\s*1 Invoice`],
		['a payment on its way', { paymentProcessing: true }, 'payment on its way'],
		['a paid Invoice', { invoiceStatus: 'paid', daysOverdue: 0 }, 'paid'],
		// #2849: as the Invoices list says it, through the one `invoiceMoneyState`.
		['a partly refunded Invoice', { invoiceStatus: 'paid', daysOverdue: 0, refundedCents: 30_000 }, String.raw`paid, \$300\.00 refunded`],
		['a void Invoice', { invoiceStatus: 'void', daysOverdue: 0 }, 'void'],
		['a written-off Invoice', { invoiceStatus: 'uncollectible', daysOverdue: 0 }, 'written off']
	])('reads %s as "%s"', async (_name, line, text) => {
		await setup(only(line));

		await expect.element(table().getByRole('cell', { name: new RegExp(String.raw`^\$4,500\.00\s*,\s*${text}$`) })).toBeVisible();
	});

	it('reads a contractor Doula’s own fee, and no Invoice money', async () => {
		await setup(only({ invoiceStatus: undefined, invoiceAmountCents: undefined, feeCents: 120_000 }));

		await expect.element(table().getByRole('cell', { name: 'Your fee $1,200.00', exact: true })).toBeVisible();
	});

	it('is left out where no Client on the page has any money, as for a Client with no Engagement', async () => {
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));

		await expect.element(testPage.getByRole('columnheader', { name: 'Status' })).toBeVisible();
		await expect.element(testPage.getByRole('columnheader', { name: 'Money' })).not.toBeInTheDocument();
		await expect.element(table().getByText(/\$/)).not.toBeInTheDocument();
	});

	it('is left out of the record card too, and the card keeps its Status line', async () => {
		await testPage.viewport(375, 740);
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));
		await testPage.viewport(375, 740);

		const record = testPage.getByRole('group', { name: 'Clients', exact: true }).element().firstElementChild as HTMLElement;
		expect(record.textContent).not.toMatch(/Money|\$/);
		expect(record.textContent).toContain('Not yet in care');
	});

	it('names the kind on each line where a Client holds two and owes nothing in all', async () => {
		await setup(
			jsonResponse({
				items: [{ ...clients[0], owedCents: 0, owedInvoiceCount: 0, openEngagements: [{ ...first, paymentProcessing: true }, ...clients[0].openEngagements!.slice(1)] }],
				hasMore: false
			})
		);

		await expect.element(table().getByText('Birth: $4,500.00, payment on its way')).toBeVisible();
		await expect.element(table().getByText('Postpartum: Your fee $1,200.00')).toBeVisible();
		await expect.element(table().getByText('Birth: $1,000.00, paid')).toBeVisible();
	});

	it('says "No amount" to a screen reader for a line with no money beside one with some', async () => {
		await setup(
			jsonResponse({
				items: [
					{
						...clients[0],
						owedCents: 0,
						owedInvoiceCount: 0,
						openEngagements: [{ ...first, paymentProcessing: true }, { ...clients[0].openEngagements![1], feeCents: undefined }]
					}
				],
				hasMore: false
			})
		);

		await expect.element(table().getByText('No amount', { exact: true })).toHaveClass('visually-hidden');
		await expect.element(table().getByText('Postpartum: —').first()).toBeVisible();
	});

	it('lines the amounts up on their right edge whatever the state beside them says', async () => {
		await renderListScreen({ width: 1440, height: 900 });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const amounts = table()
			.getByText(/^\$[\d,.]+$/)
			.elements()
			.map((amount) => Math.round(amount.getBoundingClientRect().right));
		expect(amounts.length).toBeGreaterThan(3);
		expect(Math.max(...amounts) - Math.min(...amounts)).toBeLessThanOrEqual(1);
	});

	// A card's fields come in one order, and a field it has nothing in is not drawn.
	it('reads on the record card on a line of its own under the Status, labeled "Money"', async () => {
		await renderListScreen({ width: 375, height: 740 });

		const record = testPage.getByRole('group', { name: 'Clients', exact: true }).element().firstElementChild as HTMLElement;
		expect(record.textContent).toMatch(/Birth\s*,\s*Active.*Contract signed\s*Money:\s*\$1,500\.00\s*,\s*owed/s);
		expect(record.querySelector('.record-money')!.textContent).toMatch(/Money:\s*\$1,500\.00\s*,\s*owed/);
	});

	it('draws a card’s fields in one order: name, Status, Money when it has some, Date, Doula', async () => {
		await renderListScreen({ width: 375, height: 740 });

		const cards = [...testPage.getByRole('group', { name: 'Clients', exact: true }).element().querySelectorAll('.record')];
		const withMoney = cards.filter((card) => card.querySelector('.record-money'));
		expect(withMoney.length).toBeGreaterThan(0);
		expect(withMoney.length).toBeLessThan(cards.length);
		for (const card of cards) {
			const title = card.querySelector('.record-title')!;
			const lead = card.querySelector('.compact-lead')!;
			expect(isAfter(title, lead), 'the Status after the name').toBe(true);
			const money = card.querySelector('.record-money');
			const meta = card.querySelector('.compact-meta');
			if (money) expect(isAfter(lead, money), 'Money after the Status').toBe(true);
			if (money && meta) expect(isAfter(money, meta), 'Date and Doula after the Money').toBe(true);
			const date = meta?.querySelector('.date');
			const doula = meta?.querySelector('.doula-names');
			if (date && doula) expect(isAfter(date, doula), 'the Doula after the Date').toBe(true);
		}
	});

	it('draws no Money line on a card whose Client has none, and no card with an empty line', async () => {
		await renderListScreen({ width: 375, height: 740 });

		const cards = [...testPage.getByRole('group', { name: 'Clients', exact: true }).element().querySelectorAll('.record')];
		const notInCare = cards.find((card) => card.textContent?.includes('Not yet in care'))!;
		expect(notInCare.querySelector('.record-money')).toBeNull();
		for (const card of cards) {
			for (const line of card.querySelectorAll('.record-title, .compact-lead, .compact-meta, .record-money')) {
				expect(line.textContent!.trim(), 'an empty line in a card').not.toBe('');
				expect(line.getBoundingClientRect().height, 'an empty line box').toBeGreaterThan(10);
			}
		}
	});

	it('draws no Money on a record card for a Client with none', async () => {
		await testPage.viewport(375, 740);
		await setup(jsonResponse({ items: [clients[1]], hasMore: false }));
		await testPage.viewport(375, 740);

		await expect.element(testPage.getByText(/Money:/)).not.toBeInTheDocument();
	});
});

// #458: "Also show anyone not yet in care" mixes Clients who have work with
// Clients who don't, so the row has to say which is which. It says so in
// Status, the cell that already describes the work, not in a column of
// its own that read "Has work" beside a Status that showed the work.
describe('a Client with no work (#458)', () => {
	it('reads "Not yet in care" in Status once "Also show anyone not yet in care" is on', async () => {
		pageState.url.searchParams.set('all', 'true');

		await setup();

		await expect.element(testPage.getByRole('table')).toBeVisible();
		expect(testPage.getByRole('columnheader', { name: 'Work' }).elements()).toHaveLength(0);
		await expect.element(testPage.getByRole('cell', { name: 'Not yet in care' })).toBeVisible();
	});
});

// Bare `render(Page, {})`, not the module's own `setup()`: these two tests
// drive the exact sequence of `apiFetchWithSession` responses across two
// fetches (initial load, then "Load more"), which `setup()`'s single
// `response` parameter answers every call with alike -- there is nothing
// this block could hand it that would still let the second call differ
// from the first.
async function setupLoadMoreRace() {
	await render(Page, {});
}

describe('a "Load more" already in flight when the filter changes', () => {
	/*
	 * "Load more" is the only fetch that merges into what is on screen, so
	 * it is the only one a stale response can corrupt -- the old filter's
	 * Clients appended onto the new filter's list, with nothing afterwards
	 * to take them back out. Flipping the toggle has to abandon it.
	 */
	it('drops its response rather than appending the old filter\'s Clients', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();

		// Page two never settles until the test says so.
		const pageTwo = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(pageTwo.promise);
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' }).click();
		await applyFilters();
		pageTwo.resolve(
			jsonResponse({
				items: [
					{
						clientId: 'client-9',
						name: 'Stale Filter Client',
						email: 'stale@example.com',
						hasWork: true
					}
				],
				hasMore: false
			})
		);

		await expect
			.element(testPage.getByRole('rowheader', { name: 'Stale Filter Client' }))
			.not.toBeInTheDocument();
	});

	it('surfaces a "Load more" failure only while the filter has not moved', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(textResponse('the practice is gone'));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect
			.element(testPage.getByText('More Clients did not load. Try again.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Try again' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('the practice is gone')).not.toBeInTheDocument();
		// #506: a failed "Load more" used to route through the same `error`
		// state as the initial load, replacing the whole table with a bare
		// notice -- it must leave the rows already on screen in place.
		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();
	});

	it('swallows a "Load more" failure whose filter has since moved', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();
		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();

		const pageTwo = Promise.withResolvers<Response>();
		apiFetchWithSession.mockReturnValueOnce(pageTwo.promise);
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' }).click();
		await applyFilters();
		pageTwo.resolve(textResponse('the practice is gone'));

		await expect.element(testPage.getByText(/More Clients did not load/)).not.toBeInTheDocument();
	});
});

const status = () => testPage.getByRole('status');

describe('the count line says how many there are in all (#2867)', () => {
	it('says the total while more exist, from the first page only, and the plain line once Load more has shown every Client', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1', clientCount: 2 })
		);
		await setupLoadMoreRace();
		await expect.element(status()).toHaveTextContent('Showing the first 1 of 2 Clients, soonest due first');

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ items: [clients[1]], hasMore: false }));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(status()).toHaveTextContent('Showing 2 Clients, soonest due first');
	});

	it('says no total when the page carries none', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [clients[0]], hasMore: true, nextCursor: 'cursor-1' })
		);
		await setupLoadMoreRace();

		await expect.element(status()).toHaveTextContent('Showing the first 1 Client, soonest due first');
	});
});

describe('the Name column', () => {
	it('is the row header, so a cell is heard against its Client', async () => {
		await setup();

		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();
	});
});

// A keyboard user who applies a filter, clears it, goes back or forward, or
// returns from a Client's hub must land where the result is, not at the top of
// the page. The `page` mock is not reactive, so these swap in a reactive URL
// and play SvelteKit's `afterNavigate` by hand.
const listRoute = { id: '/practices/[practiceId]/clients' };
const hubRoute = { id: '/practices/[practiceId]/clients/[clientId]' };

async function waitForRows() {
	await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();
}

describe('where focus goes after the list changes', () => {
	const plainUrl = pageState.url;
	beforeEach(() => {
		pageState.url = new SvelteURL(plainUrl);
	});
	afterEach(() => {
		pageState.url = plainUrl;
	});

	it('stays where it is on first arrival', async () => {
		await setup();
		navigation.onMove!({});
		await waitForRows();

		await expect.element(testPage.getByRole('status')).not.toHaveFocus();
	});

	it('moves to the result line once a changed filter has loaded', async () => {
		await setup();
		navigation.onMove!({});
		await waitForRows();

		historyState.clientsName = 'Cam';
		navigation.onMove!({ from: { route: listRoute, params: {} } });

		await expect.element(testPage.getByRole('status')).toHaveFocus();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/named "Cam"/);
	});

	// A box ticked right after arriving was lost when the address was copied
	// into the controls again on a re-run; the copy now happens once per move.
	it('keeps a ticked box when the page re-renders at the same address', async () => {
		await setup();
		navigation.onMove!({});
		await waitForRows();
		const owes = testPage.getByRole('checkbox', { name: 'Who owe money' });

		await owes.click();
		pageState.url = new SvelteURL(pageState.url.href);
		await new Promise((resolve) => setTimeout(resolve, 0));

		await expect.element(owes).toBeChecked();
	});

	it('puts the controls back in step with the address after a move', async () => {
		await setup();
		navigation.onMove!({});
		await waitForRows();
		await testPage.getByRole('checkbox', { name: 'Who owe money' }).click();

		navigation.onBefore();
		historyState.clientsName = 'Cam';
		navigation.onMove!({ from: { route: listRoute, params: {} } });

		await expect.element(testPage.getByRole('checkbox', { name: 'Who owe money' })).not.toBeChecked();
		await expect.element(testPage.getByRole('searchbox', { name: 'Name' })).toHaveValue('Cam');
	});

	it('stays put when a move leaves the filter as it was', async () => {
		await setup();
		navigation.onMove!({});
		await waitForRows();

		navigation.onMove!({ from: { route: listRoute, params: {} } });
		await new Promise((resolve) => setTimeout(resolve, 0));

		await expect.element(testPage.getByRole('status')).not.toHaveFocus();
	});

	it('returns to the Client’s row link from her hub', async () => {
		await setup();
		navigation.onMove!({ from: { route: hubRoute, params: { clientId: clients[1].clientId } } });

		await expect.element(testPage.getByRole('link', { name: clients[1].name })).toHaveFocus();
	});

	it('falls back to the result line when her row is not on the loaded page', async () => {
		await setup();
		navigation.onMove!({ from: { route: hubRoute, params: { clientId: 'client-not-loaded' } } });

		await expect.element(testPage.getByRole('status')).toHaveFocus();
	});

	it('has nowhere to land for a contractor Doula whose row is not loaded, and leaves focus alone', async () => {
		await setup(undefined, { isContractor: true });
		navigation.onMove!({ from: { route: hubRoute, params: { clientId: 'client-not-loaded' } } });
		await waitForRows();
		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(document.activeElement).toBe(document.body);
	});
});

describe('a control changed before the page has seeded its controls', () => {
	it('is never overwritten by the seed', async () => {
		await setup();
		const owes = testPage.getByRole('checkbox', { name: 'Who owe money' });

		await owes.click();
		navigation.onMove!({});

		await expect.element(owes).toBeChecked();
	});

	it('is seeded again by the next move', async () => {
		await setup();
		const owes = testPage.getByRole('checkbox', { name: 'Who owe money' });
		await owes.click();

		navigation.onBefore();
		navigation.onMove!({});

		await expect.element(owes).not.toBeChecked();
	});

	it('holds for a typed Name and a chosen Doula too', async () => {
		await setup(undefined, { doulas: [{ value: 'staff-kimiko', label: 'Kimiko Nakashima' }] });

		await testPage.getByRole('searchbox', { name: 'Name' }).fill('Cam');
		await testPage.getByRole('combobox', { name: 'Doula' }).selectOptions('Kimiko Nakashima');
		navigation.onMove!({});

		await expect.element(testPage.getByRole('searchbox', { name: 'Name' })).toHaveValue('Cam');
		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toHaveValue('staff-kimiko');
	});
});

describe('the list while a newer filter loads', () => {
	const plainUrl = pageState.url;
	beforeEach(() => {
		pageState.url = new SvelteURL(plainUrl);
	});
	afterEach(() => {
		pageState.url = plainUrl;
	});

	it('keeps the rows on screen, marked busy, until the new page lands', async () => {
		await setup();
		await waitForRows();
		const landings: ((response: Response) => void)[] = [];
		apiFetchWithSession.mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					landings.push(resolve);
				})
		);

		pageState.url.searchParams.set('owes', 'true');

		await expect.element(testPage.getByRole('table')).toHaveAttribute('aria-busy', 'true');
		await expect.element(testPage.getByRole('rowheader', { name: clients[0].name })).toBeVisible();
		await expect.element(testPage.getByRole('status')).not.toHaveTextContent('who owe money');

		landings[0](jsonResponse({ items: [], hasMore: false }));

		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('status')).toHaveTextContent('No Client owes money.');
	});

	it('drops a page that a newer filter has already replaced', async () => {
		await setup();
		await waitForRows();
		const landings: ((response: Response) => void)[] = [];
		apiFetchWithSession.mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					landings.push(resolve);
				})
		);

		pageState.url.searchParams.set('owes', 'true');
		await expect.poll(() => landings.length).toBe(1);
		pageState.url.searchParams.set('noContract', 'true');
		await expect.poll(() => landings.length).toBe(2);
		landings[1](jsonResponse({ items: [], hasMore: false }));
		landings[0](jsonResponse({ items: [clients[1]], hasMore: false }));

		await expect.element(testPage.getByRole('status')).toHaveTextContent('No active Client owes money and is missing a Contract.');
	});
});

describe('the filters that could never match', () => {
	it('leaves "Mine" out for a Doula who holds no live Client', async () => {
		await setup(undefined, { isDoula: true, hasMine: false });

		await expect.element(testPage.getByRole('checkbox', { name: 'Who owe money' })).toBeVisible();
		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).not.toBeInTheDocument();
	});

	it('keeps "Mine" while it is on in the address', async () => {
		pageState.url.searchParams.set('mine', 'true');

		await setup(nobody, { isDoula: true, hasMine: false });

		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).toBeChecked();
	});

	it('names the Doula in a sentence', async () => {
		pageState.url.searchParams.set('doula', 'staff-lena');

		await setup(nobody, { doulas: [{ value: 'staff-lena', label: 'Lena Vasquez' }] });

		await expect.element(testPage.getByText('Lena Vasquez has no Clients.').first()).toBeVisible();
	});

	it('says nothing of the no-care switch beside a narrowing, which already implies care', async () => {
		pageState.url.searchParams.set('all', 'true');
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing \d+ Clients? who owes? money, soonest due first, then most recent/);
	});
});

describe('an empty result', () => {
	// The bar's own link is in view at every width, so the message does not
	// offer it a second time; it says what was looked for and what widens it.
	it('offers the bar\'s one way out, not a second beside the message', async () => {
		pageState.url.searchParams.set('owes', 'true');

		await setup(nobody);

		const out = testPage.getByRole('link', { name: 'Remove filter: Who owe money' });
		await expect.element(out).toBeVisible();
		await expect.element(out).toHaveAttribute('href', `/practices/${practiceId}/clients`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText('No Client owes money. Clear the filters to widen the list.').first())
			.toBeVisible();
	});

	it('offers no "Clear filters" where nothing is applied', async () => {
		await setup(nobody);

		await expect.element(testPage.getByText('No Client is in care yet.', { exact: false }).first()).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});
});

describe('a pending Request', () => {
	it('is a blue badge, so amber stays for what a Client owes', async () => {
		await setup();

		await expect.element(testPage.getByText('Birth request pending').first()).toHaveClass('info');
	});
});

describe('the Due column', () => {
	// Left-aligned with the words around it: right-aligned, a date ran into
	// the Doula's name in the next column.
	it('mutes "Due" and "Ended" before the day, and starts the column at the left like its words', async () => {
		await setup();

		await expect.element(table().getByText('Due', { exact: true }).first()).toHaveClass('lead');
		await expect.element(testPage.getByText('Ended', { exact: true }).first()).toHaveClass('lead');
		await expect.element(testPage.getByRole('columnheader', { name: 'Date' })).not.toHaveClass('numeric');
	});

	it('reads a live birth as one phrase of weeks, with no leading word', async () => {
		await setup();

		await expect.element(testPage.getByRole('row', { name: /Wilhelmina/ })).toHaveTextContent('26 weeks postpartum');
	});
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
	rosterDoulas?: typeof doulas;
}

async function renderListScreen({
	width,
	height,
	respond = typical.respond,
	hasAnyClient = Promise.resolve(true),
	isDoula = true,
	rosterDoulas = doulas
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
			doulas: Promise.resolve(rosterDoulas),
			everyDoula: Promise.resolve(rosterDoulas),
			canChooseDoula: rosterDoulas.length > 0,
			opensOnMine: false,
			session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor: false, timezone: 'America/New_York' }
		}
	});
}

const rectOf = (locator: Located) => locator.element().getBoundingClientRect();


const rowHeights = () =>
	testPage
		.getByRole('row')
		.elements()
		.slice(1)
		.map((row) => row.getBoundingClientRect().height);

// The row of applied-filter tags (a 44px target and the step under it), drawn only while a filter
// is on. The phone screens below all have one on, so the fold they hold is this far lower than it
// was before the tags (decision 4 of 2026-11-08): the first card is still reached within it.
const TAG_ROW = 60;

// The vertical center of an element's words, apart from the box that holds them.
const centerOfWords = (element: Element) => {
	const range = document.createRange();
	range.selectNodeContents(element);
	const box = range.getBoundingClientRect();
	return box.top + box.height / 2;
};

describe('the list at a phone’s width', () => {
	// Rubric T4: with one filter on, the first Client's card sits fully on the
	// first screen at 375px, and at 320px its name and status line show.
	it('puts the first Client on the first screen with one filter on', async () => {
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width: 375, height: 740 });

		const group = testPage.getByRole('group', { name: 'Clients', exact: true });
		await expect.element(group).toBeVisible();
		const first = group.element().firstElementChild!.getBoundingClientRect();
		expect(first.bottom).toBeLessThanOrEqual(740 + TAG_ROW);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(document.documentElement.clientWidth);
	});

	it('shows the first Client’s name and status line at 320px', async () => {
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width: 320, height: 740 });

		const group = testPage.getByRole('group', { name: 'Clients', exact: true });
		await expect.element(group).toBeVisible();
		const [name, status] = [...group.element().firstElementChild!.children].map((part) => part.getBoundingClientRect());
		// The name whole, and the status line begun on the screen; the scrollbar's
		// stable track takes 10px of a 320px window, which wraps more of the bar
		// above it, so its first line may be cut by the fold.
		expect(name.bottom).toBeLessThanOrEqual(740 + TAG_ROW);
		expect(status.top).toBeLessThan(740 + TAG_ROW);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(document.documentElement.clientWidth);
	});

	it('keeps the checkboxes and the Name in view, with the Doula select behind "More filters and order"', async () => {
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width: 375, height: 740 });

		await expect.element(owing()).toBeVisible();
		await expect.element(noContract()).toBeVisible();
		await expect.element(nameField()).toBeVisible();
		await expect.element(doulaField()).not.toBeVisible();
		await expect.element(testPage.getByText('More filters and order')).toBeVisible();
	});

	it.each([375, 320])('draws the Pilot banner on one line at %ipx', async (width) => {
		await renderListScreen({ width, height: 740 });

		const pilot = rectOf(testPage.getByText('Pilot', { exact: true }));
		const control = rectOf(testPage.getByRole('button', { name: 'Tell us what you think.' }));
		expect(Math.abs(pilot.top + pilot.height / 2 - (control.top + control.height / 2))).toBeLessThan(8);
		await expect.element(testPage.getByRole('button', { name: 'Tell us what you think.' })).toBeVisible();
	});

	it('puts the summary and Apply on one row, with Apply no wider than its words, and the one tag under them', async () => {
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width: 375, height: 740 });

		const summary = rectOf(testPage.getByText('More filters and order'));
		const apply = rectOf(testPage.getByRole('button', { name: 'Apply' }));
		const tag = rectOf(testPage.getByRole('link', { name: 'Remove filter: Who owe money' }));
		expect(apply.width).toBeLessThan(375 / 2);
		expect(apply.left).toBeGreaterThan(summary.right);
		expect(Math.abs(apply.top + apply.height / 2 - (summary.top + summary.height / 2))).toBeLessThanOrEqual(2);
		expect(tag.top).toBeGreaterThanOrEqual(apply.bottom - 1);
	});

	it('puts the one tag on Apply’s row, centered on its words, where the bar is wide', async () => {
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width: 1280, height: 900 });

		const applyButton = testPage.getByRole('button', { name: 'Apply' });
		const tagLink = testPage.getByRole('link', { name: 'Remove filter: Who owe money' });
		const [apply, tag] = [rectOf(applyButton), rectOf(tagLink)];
		expect(Math.abs(apply.top + apply.height / 2 - (tag.top + tag.height / 2))).toBeLessThanOrEqual(1);
		expect(tag.left).toBeGreaterThan(apply.right);
		const [applyCenter, tagCenter] = [centerOfWords(applyButton.element()), centerOfWords(tagLink.element())];
		expect(Math.abs(applyCenter - tagCenter)).toBeLessThanOrEqual(1);
	});

	it('reads a record as the name, the status, then the due date and the Doula with their labels', async () => {
		await testPage.viewport(375, 740);
		await setup();
		await testPage.viewport(375, 740);

		const record = testPage.getByRole('group', { name: 'Clients', exact: true }).element().firstElementChild as HTMLElement;
		expect(record.textContent).toMatch(/Date: .*Doula: /s);
	});

	it('reads a record’s date as one phrase, with no "Due:" label before it', async () => {
		await renderListScreen({ width: 375, height: 740 });

		const record = testPage.getByRole('group', { name: 'Clients', exact: true }).element().firstElementChild as HTMLElement;
		expect(record.textContent).toMatch(/Date:\s+Due\sJan 14, 2027/);
		expect(record.textContent).not.toMatch(/Due: /);
	});
});

describe('the rows of a typical page', () => {
	// A Status of the kind, one badge and two facts stays on one line, so the
	// rows are one height. A Client with a Request and no Engagement yet holds
	// the one badge; a Client with none says so in one line.
	it.each([1366, 1440])('keeps every row within 4px of the next at %ipx', async (width) => {
		await renderListScreen({ width, height: 900 });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		await document.fonts.ready;

		const heights = rowHeights();
		expect(heights.length).toBe(8);
		expect(Math.max(...heights) - Math.min(...heights)).toBeLessThanOrEqual(4);
		expect(heights.filter((height) => Math.abs(height - 40) > 4), `row heights ${heights.map((height) => Math.round(height))}`).toStrictEqual([]);
	});

	// The demo Practice's busy rows: two Doulas, a loss's "Ended" date, an overdue
	// badge and all three at once. None may wrap at a desktop width (names are
	// never truncated, so the Doula column has to hold two).
	it.each([1366, 1440])('keeps two Doulas, an Ended date and an overdue badge on one line at %ipx', async (width) => {
		await renderListScreen({ width, height: 900, respond: () => jsonResponse({ items: seedLikeClients, hasMore: false }) });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		await document.fonts.ready;

		await expect.element(testPage.getByRole('cell', { name: 'Joss Adeyemi, Lena Vasquez', exact: true }).first()).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: /Ended Nov 2, 2026/ })).toBeVisible();
		await expect.element(testPage.getByText('62 days overdue', { exact: true }).first()).toBeVisible();
		const heights = rowHeights();
		expect(heights.length).toBe(8);
		expect(heights.filter((height) => Math.abs(height - 40) > 4), `row heights ${heights.map((height) => Math.round(height))}`).toStrictEqual([]);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(document.documentElement.clientWidth);
	});

	// The longest Status line a Client holds: Postpartum, a "12 days overdue" badge,
	// Active and Contract signed. It once wrapped to 78px at 1280px.
	it.each([1366, 1440])('keeps a Postpartum line with an overdue badge, Active and Contract signed on one line at %ipx', async (width) => {
		await renderListScreen({ width, height: 900, respond: () => jsonResponse({ items: seedLikeClients, hasMore: false }) });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		await document.fonts.ready;

		const row = testPage.getByRole('row', { name: new RegExp(TYPICAL_CLIENTS[6].name) });
		await expect.element(row).toHaveTextContent('Postpartum');
		await expect.element(row).toHaveTextContent('overdue');
		await expect.element(row).toHaveTextContent('Active');
		await expect.element(row).toHaveTextContent('Contract signed');
		const height = row.element().getBoundingClientRect().height;
		expect(Math.abs(height - 40), `row height ${height}`).toBeLessThanOrEqual(4);
	});

	it('stops where its columns are satisfied at 1440px, short of the container', async () => {
		await renderListScreen({ width: 1440, height: 900 });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const table = rectOf(testPage.getByRole('table'));
		const container = testPage.getByRole('table').element().parentElement!.getBoundingClientRect();
		expect(table.width).toBeLessThan(container.width);
	});

	// Rubric H8/C4: the screen's job fills the first screen. The count line now
	// has a row of its own on the frame's left edge, which costs the first screen
	// two rows at 1366×800 (eight became six, measured); the 1280×800 window holds its
	// own check in clients-list.layout.svelte.spec.ts.
	it('puts six rows on the first screen at 1366×800', async () => {
		await renderListScreen({ width: 1366, height: 800, isDoula: false });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		await document.fonts.ready;

		const table = rectOf(testPage.getByRole('table'));
		const onScreen = testPage
			.getByRole('row')
			.elements()
			.slice(1)
			.filter((row) => row.getBoundingClientRect().bottom <= 800);
		// Eight rows of the brief's 40px, under the header, end by the fold. A row
		// that holds a Badge is a pixel or two taller, so the count of rows
		// whole on screen is read beside it, not instead of it.
		const header = rectOf(testPage.getByRole('columnheader').first()).height;
		const detail = `table top ${table.top.toFixed(1)}, header ${header}, rows ${rowHeights().map((height) => height.toFixed(1))}`;
		expect(table.top + header + 6 * 40, detail).toBeLessThanOrEqual(800);
		expect(onScreen.length, detail).toBeGreaterThanOrEqual(5);
	});

	it('gives the Date column about 10rem at 1280px', async () => {
		await renderListScreen({ width: 1280, height: 900 });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const width = rectOf(testPage.getByRole('columnheader', { name: 'Date' })).width;
		// "Ended Nov 30, 2026" holds one line here, and no more room than that is spent on it.
		expect(width).toBeLessThanOrEqual(12.5 * 16);
		expect(width).toBeGreaterThanOrEqual(9.5 * 16);
	});

	it('runs two Doulas on one Engagement together on one line, comma-joined', async () => {
		const [line] = clients[0].openEngagements ?? [];
		await setup(
			jsonResponse({
				items: [{ ...clients[1], name: TYPICAL_CLIENTS[0].name, hasWork: true, openEngagements: [{ ...line, doulaNames: ['Joss Adeyemi', 'Lena Vasquez'] }] }],
				hasMore: false
			})
		);
		// Two names are the busiest a Doula cell gets; they share a line where the
		// column has the room, and wrap at the comma, never inside a name, where it
		// has not (about 2000px of window).
		await testPage.viewport(2560, 900);

		const cell = table().getByRole('cell', { name: 'Joss Adeyemi, Lena Vasquez', exact: true });
		await expect.element(cell).toBeVisible();
		expect(table().getByRole('row').nth(1).element().getBoundingClientRect().height).toBeLessThan(48);
	});
});

// A person reaching for Apply finds it where it was: the bar is the same
// arrangement whatever is applied, so Apply never moves. The tags follow Apply
// in the room beside it, where one filter's fit and the table holds still; two
// filters' take a line under Apply, and the table moves by that line.
const states: [string, () => void][] = [
	['default', () => {}],
	['owes', () => pageState.url.searchParams.set('owes', 'true')],
	['a Doula', () => pageState.url.searchParams.set('doula', kimiko.value)],
	[
		'a Name and a Doula',
		() => {
			pageState.url.searchParams.set('doula', kimiko.value);
			historyState.clientsName = 'Camille';
		}
	]
];

async function measureState(width: number, apply: () => void) {
	pageState.url.searchParams.delete('owes');
	pageState.url.searchParams.delete('doula');
	pageState.url.searchParams.delete('order');
	historyState.clientsName = undefined;
	apply();
	await renderListScreen({ width, height: 800 });
	await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
	await document.fonts.ready;
	const button = rectOf(testPage.getByRole('button', { name: 'Apply' }));
	const result = {
		applyLeft: button.left,
		applyTop: button.top,
		tableTop: rectOf(testPage.getByRole('table')).top
	};
	cleanup();
	return result;
}

describe('the filter bar between filter states', () => {
	it.each([1280, 1440, 1920])('keeps Apply, the count and the table where they were in every filter state at %ipx', async (width) => {
		const measured = [];
		for (const state of states) measured.push(await measureState(width, state[1]));
		const [baseline] = measured;
		for (const [index, [name]] of states.entries()) {
			const here = measured[index]!;
			expect(Math.abs(here.applyLeft - baseline!.applyLeft), `${name}: Apply x`).toBeLessThanOrEqual(0.5);
			expect(Math.abs(here.applyTop - baseline!.applyTop), `${name}: Apply y`).toBeLessThanOrEqual(0.5);
			const tagLines = name === 'a Name and a Doula' ? 70 : 0.5;
			expect(Math.abs(here.tableTop - baseline!.tableTop), `${name}: table top`).toBeLessThanOrEqual(tagLines);
		}
	});

	it('holds the space of "Also show…" while hiding it from the tab order and the accessibility tree', async () => {
		pageState.url.searchParams.set('doula', kimiko.value);
		await renderListScreen({ width: 1440, height: 900 });

		await expect.element(testPage.getByRole('link', { name: /^Remove filter: Doula/ })).toBeVisible();
		await expect
			.element(testPage.getByRole('checkbox', { name: 'Also show anyone not yet in care' }))
			.not.toBeInTheDocument();
		const form = testPage.getByRole('button', { name: 'Apply' }).element().closest('form')!;
		const held = form.querySelector<HTMLElement>('[data-holds-space]')!;
		// Not in the accessibility tree: no role query reaches it, and `visibility: hidden` is why.
		expect(getComputedStyle(held).visibility).toBe('hidden');
		expect(held.getBoundingClientRect().width).toBeGreaterThan(100);
		expect(held.hasAttribute('inert')).toBe(true);
	});

	it('holds nothing empty after Apply while no filter is on: the count sits the related step under it', async () => {
		await renderListScreen({ width: 1440, height: 900 });

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
		const apply = rectOf(testPage.getByRole('button', { name: 'Apply' }));
		expect(rectOf(testPage.getByRole('status')).top - apply.bottom).toBeLessThanOrEqual(12);
	});
});

// The typical page with each listed Client's open line changed.
function changed(changes: Readonly<Partial<Record<number, object>>>) {
	return seedLikeClients.map((client, index) => {
		const [line] = client.openEngagements ?? [];
		const change = changes[index];
		if (!change || !line) return client;
		const merged = { ...line, ...change };
		// What the BFF sums for the row (#2874): an open Invoice with no payment on its way is what a Client owes.
		const isOwed = merged.invoiceStatus === 'open' && !merged.paymentProcessing;
		return {
			...client,
			owedCents: isOwed ? (merged.invoiceAmountCents ?? 0) : 0,
			owedInvoiceCount: isOwed ? 1 : 0,
			openEngagements: [merged]
		};
	});
}

// A state is whole, and the Date column begins after it.
describe('the Money column’s state', () => {
	// The typical page, with one payment on its way, the longest plain state words, a
	// written-off Invoice and one 120 days overdue, the widest red badges, and a
	// four-figure refund, the longest plain state (#2849).
	const refunded = { invoiceStatus: 'paid', invoiceAmountCents: 150_000, daysOverdue: 0, refundedCents: 150_000 } as const;
	const latest = {
		3: { invoiceStatus: 'open', invoiceAmountCents: 150_000, paymentProcessing: true },
		4: { invoiceStatus: 'uncollectible', daysOverdue: 0 },
		5: { invoiceStatus: 'open', daysOverdue: 120 },
		7: refunded
	} as const;
	const withClearing = changed(latest);

	// #2739's criterion, kept by #2849: a refund's longer words move nothing.
	it.each([1024, 1280, 1440])('measures the same with a four-figure refund as without at %ipx', async (width) => {
		const measure = async (items: typeof seedLikeClients) => {
			await renderListScreen({ width, height: 900, respond: () => jsonResponse({ items, hasMore: false }) });
			await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
			await document.fonts.ready;
			const table = testPage.getByRole('table').element().getBoundingClientRect();
			const rows = testPage.getByRole('row').elements().map((row) => Math.round(row.getBoundingClientRect().height));
			cleanup();
			return { width: Math.round(table.width), rows };
		};
		const without = await measure(changed({ 7: { invoiceStatus: 'paid', invoiceAmountCents: 150_000, daysOverdue: 0 } }));
		const withRefund = await measure(changed({ 7: refunded }));
		expect(withRefund).toStrictEqual(without);
	});

	it.each([1280, 1440, 1920])('fits its track and ends before the Date column’s text at %ipx', async (width) => {
		await renderListScreen({ width, height: 900, respond: () => jsonResponse({ items: withClearing, hasMore: false }) });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		await document.fonts.ready;

		const states = [...document.querySelectorAll<HTMLElement>('table.table-view .money .state')].filter((state) => state.getClientRects().length > 0);
		expect(states.length).toBeGreaterThan(0);
		for (const state of states) {
			// #2849: a four-figure refund may reach into its own cell's inset, never past the cell.
			const textEnd = state.getBoundingClientRect().left + state.scrollWidth;
			expect(textEnd, `"${state.textContent}" overflows its cell`).toBeLessThanOrEqual(state.closest('td')!.getBoundingClientRect().right);
			const day = state.closest('tr')!.querySelector<HTMLElement>(':scope .date .lead');
			if (!day) continue;
			const gap = day.getBoundingClientRect().left - state.getBoundingClientRect().right;
			expect(gap, `"${state.textContent}" to the Date column`).toBeGreaterThanOrEqual(7);
		}
	});

	it('draws overdue in the red badge; owed, not yet due, and a settled state stay plain words (#2891)', async () => {
		await renderListScreen({ width: 1280, height: 900, respond: () => jsonResponse({ items: seedLikeClients, hasMore: false }) });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const badges = [...document.querySelectorAll<HTMLElement>('table.table-view .money .state span[class]')].filter((element) => /overdue/.test(element.textContent ?? ''));
		expect(badges.length).toBeGreaterThan(0);
		for (const badge of badges) {
			expect(getComputedStyle(badge).getPropertyValue('--badge-hue').trim()).toBe(getComputedStyle(document.documentElement).getPropertyValue('--color-error').trim());
		}
		const states = [...document.querySelectorAll<HTMLElement>('table.table-view .money .state')];
		const owed = states.filter((state) => state.textContent?.trim().startsWith('owed'));
		expect(owed.length).toBeGreaterThan(0);
		for (const state of owed) expect(state.querySelector('svg')).toBeNull();
	});

	it('says the fact once to a screen reader: in the Money cell, never in Status', async () => {
		await renderListScreen({ width: 1280, height: 900, respond: () => jsonResponse({ items: seedLikeClients, hasMore: false }) });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const row = testPage.getByRole('row', { name: new RegExp(TYPICAL_CLIENTS[6].name) }).element();
		expect(row.textContent!.match(/overdue/g)).toHaveLength(1);
		expect(row.querySelector('td.grows')!.textContent).not.toMatch(/overdue|Owes/i);
		expect(row.querySelector('td.numeric')!.textContent).toMatch(/\$[\d,.]+\s*,\s*12 days overdue\s*,\s*1 Invoice/);
	});

	it('gives the widest badge its track', async () => {
		await renderListScreen({ width: 1280, height: 900, respond: () => jsonResponse({ items: seedLikeClients, hasMore: false }) });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const badge = [...document.querySelectorAll<HTMLElement>('table.table-view .money .state span')].find((element) => element.textContent?.trim() === '62 days overdue')!;
		expect(badge.getBoundingClientRect().width).toBeLessThanOrEqual(badge.closest('.state')!.getBoundingClientRect().width);
	});
});

describe('the Date column', () => {
	it('starts every day at one place whichever word leads it', async () => {
		const [line] = clients[0].openEngagements ?? [];
		const one = (id: string, lead: object) => ({
			...clients[1],
			clientId: id,
			name: id,
			hasWork: true,
			openEngagements: [{ ...line, engagementId: id, invoiceStatus: undefined, invoiceAmountCents: undefined, ...lead }]
		});
		await setup(
			jsonResponse({
				items: [
					one('Dueclient', { dueDate: '2027-01-10' }),
					one('Endedclient', { dueDate: undefined, birthOutcome: 'loss', pregnancyEndedOn: '2026-11-30' })
				],
				hasMore: false
			})
		);
		await document.fonts.ready;

		for (const width of [1280, 1440, 1920]) {
			await testPage.viewport(width, 900);
			const starts = [
				['Due', 'Jan 10, 2027'],
				['Ended', 'Nov 30, 2026']
			].map(([word, day]) => {
				const lead = table().getByText(word!, { exact: true }).element();
				expect(lead.scrollWidth, `"${word}" overflows its track`).toBeLessThanOrEqual(lead.clientWidth);
				return table().getByText(day!, { exact: true }).element().getBoundingClientRect().left;
			});
			expect(Math.max(...starts) - Math.min(...starts), `day starts ${starts} at ${width}px`).toBeLessThanOrEqual(1);
		}
	});

	it('still reads "Due Jan 14, 2027" as one phrase to a screen reader', async () => {
		await renderListScreen({ width: 1440, height: 900 });

		await expect.element(table().getByRole('cell', { name: 'Due Jan 14, 2027', exact: true })).toBeVisible();
	});

	it('says in each value which date it is, with the word muted', async () => {
		await setup();

		const row = testPage.getByRole('row', { name: /Wilhelmina/ });
		await expect.element(row).toHaveTextContent('26 weeks postpartum');
		await expect.element(row).toHaveTextContent('Ended Nov 30, 2026');
		await expect.element(table().getByText('Due', { exact: true }).first()).toHaveClass('lead');
		await expect.element(testPage.getByRole('columnheader', { name: 'Date' })).toBeVisible();
		await expect.element(testPage.getByRole('columnheader', { name: 'Due' })).not.toBeInTheDocument();
	});

	it('reads "Due" before a date not yet reached', async () => {
		await setup();

		await expect.element(testPage.getByRole('row').nth(1)).toHaveTextContent('Due Jan 10, 2027');
	});
});

describe('while a new filter loads', () => {
	const plainUrl = pageState.url;
	beforeEach(() => {
		pageState.url = new SvelteURL(plainUrl);
	});
	afterEach(() => {
		pageState.url = plainUrl;
	});

	// Acknowledged within 100ms: Apply shows its loading state the moment it
	// is pressed, before the address has changed, and the rows dim.
	it('shows Apply busy and dims the rows within 100ms of pressing it', async () => {
		await setup();
		await waitForRows();
		goto.mockImplementation(() => new Promise(() => {}));
		await owing().click();

		// The address never changes here (the navigation is held), so what
		// answers the press is the press itself, not the list's own request.
		await applyFilters();
		await expect
			.poll(() => testPage.getByRole('button', { name: 'Apply' }).element().getAttribute('aria-busy'), { timeout: 100 })
			.toBe('true');
		await expect
			.poll(() => getComputedStyle(testPage.getByRole('table').element()).opacity, { timeout: 100 })
			.not.toBe('1');
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	it('keeps Apply busy while the new page loads, and ends it when the page lands', async () => {
		await setup();
		await waitForRows();
		const landings: ((response: Response) => void)[] = [];
		apiFetchWithSession.mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					landings.push(resolve);
				})
		);

		pageState.url.searchParams.set('owes', 'true');

		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toHaveAttribute('aria-busy', 'true');
		landings[0](jsonResponse({ items: [clients[0]], hasMore: false }));
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toHaveAttribute('aria-busy', 'false');
		await expect.element(testPage.getByRole('table')).toHaveStyle({ opacity: '1' });
	});
});

describe('the "Also show anyone not yet in care" switch beside a filter', () => {
	it.each([['mine'], ['owes'], ['noContract'], ['doula']])('is not drawn while `%s` is applied', async (name) => {
		pageState.url.searchParams.set(name, name === 'doula' ? kimiko.value : 'true');

		await setup(undefined, { isDoula: true, doulas });

		await expect.element(owing()).toBeVisible();
		await expect.element(alsoShow()).not.toBeInTheDocument();
	});

	// Its space is kept while it is out, so ticking a narrowing switch and applying
	// it moves nothing: Apply stays where it was.
	it.each([1280, 1440, 1920])('keeps Apply where it was when it goes, at %ipx', async (width) => {
		await renderListScreen({ width, height: 900 });
		await expect.element(alsoShow()).toBeVisible();
		const before = rectOf(testPage.getByRole('button', { name: 'Apply' }));

		// The list as the address leaves it once `owes` is applied.
		cleanup();
		pageState.url.searchParams.set('owes', 'true');
		await renderListScreen({ width, height: 900 });
		await expect.element(owing()).toBeChecked();
		await expect.element(alsoShow()).not.toBeInTheDocument();

		const after = rectOf(testPage.getByRole('button', { name: 'Apply' }));
		expect(Math.abs(after.left - before.left), `Apply at ${before.left}, then ${after.left}`).toBeLessThanOrEqual(0.5);
		expect(Math.abs(after.top - before.top)).toBeLessThanOrEqual(0.5);
	});


	it('stays drawn beside a Name, which implies no care', async () => {
		historyState.clientsName = 'Camille';

		await setup();

		await expect.element(alsoShow()).toBeVisible();
	});
});

describe('the default list’s words', () => {
	it('says what the default list holds, in the result line and on the switch', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent('Showing 4 Clients, soonest due first, then most recently ended.');
		await expect.element(alsoShow()).toBeVisible();
	});
});

describe('a Doula who holds no Client', () => {
	it('is not an option of the Doula select', async () => {
		await setup(undefined, { doulas: [kimiko] });

		await expect.element(doulaField().getByRole('option', { name: lena.label })).not.toBeInTheDocument();
		await expect.element(doulaField().getByRole('option', { name: kimiko.label })).toBeInTheDocument();
	});
});

describe('a Doula in the address that the BFF refuses', () => {
	it('says the Doula is not in this Practice, with a way out, and no Try again', async () => {
		pageState.url.searchParams.set('doula', 'not-an-id');

		await setup(jsonResponse({ error: 'doula must be a staff id' }, 400), { doulas });

		await expect.element(testPage.getByText('That Doula is not in this Practice.').first()).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: /^Remove filter: Doula/ })).toHaveAttribute('href', `/practices/${practiceId}/clients`);
		await expect.element(testPage.getByText(/Your Clients did not load/)).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Try again' })).not.toBeInTheDocument();
	});

	it('still says the Clients did not load for a refusal that is not about a Doula', async () => {
		await setup(jsonResponse({ error: 'bad' }, 400));

		await expect.element(testPage.getByText(/Your Clients did not load/)).toBeVisible();
	});
});

describe('the action while the Practice’s Client read is on its way', () => {
	it.each([1440, 375])('does not move the page when the link lands, at %ipx', async (width) => {
		const { promise: pending, resolve: land } = Promise.withResolvers<boolean>();
		await renderListScreen({ width, height: 900, hasAnyClient: pending });
		const bar = testPage.getByRole('group', { name: 'Choose which Clients to show' });
		await expect.element(bar).toBeVisible();
		const before = top(bar);

		land(true);
		await expect.element(testPage.getByRole('link', { name: 'Find or add a Client' })).toBeVisible();

		expect(Math.abs(top(bar) - before)).toBeLessThanOrEqual(0.5);
	});
});

describe('Back to the list after Load more', () => {
	const pages = [
		{ items: [clients[0]], nextCursor: 'cursor-2', hasMore: true },
		{ items: [clients[1]], nextCursor: 'cursor-3', hasMore: true },
		{ items: [clients[2]], hasMore: false }
	];
	const answerPages = (path: string) => {
		const index = path.includes('cursor=cursor-3') ? 2 : (path.includes('cursor=cursor-2') ? 1 : 0);
		return Promise.resolve(jsonResponse(pages[index]));
	};

	it('keeps how many pages are open in the history entry', async () => {
		await setup(undefined, { responder: answerPages });
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).toBeVisible();

		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('rowheader', { name: clients[1].name })).toBeVisible();

		expect(replaceState).toHaveBeenLastCalledWith('', { clientsName: undefined, clientsPages: 2 });
	});

	// T3 (#2746): replaceState resolves the route before it writes, so a page
	// that lands once she has left would stamp the entry she is opening.
	it('stamps nothing when the page lands after she has left for a Client', async () => {
		await setup(undefined, { responder: answerPages });
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).toBeVisible();
		replaceState.mockClear();

		navigation.onBefore({ to: { url: new URL(`https://example.test/practices/${practiceId}/clients/${clients[1].clientId}`) } });
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('rowheader', { name: clients[1].name })).toBeVisible();

		expect(replaceState).not.toHaveBeenCalled();
	});

	it('loads that many pages again, then lands on the row she came from', async () => {
		historyState.clientsPages = 3;
		await setup(undefined, { responder: answerPages });

		await expect.element(testPage.getByRole('rowheader', { name: clients[2].name })).toBeVisible();
		expect(apiFetchWithSession.mock.calls.map((call) => String(call[0]))).toEqual([
			`/api/practices/${practiceId}/clients`,
			`/api/practices/${practiceId}/clients?cursor=cursor-2`,
			`/api/practices/${practiceId}/clients?cursor=cursor-3`
		]);
		navigation.onMove!({ from: { route: hubRoute, params: { clientId: clients[2].clientId } } });

		await expect.element(testPage.getByRole('link', { name: clients[2].name })).toHaveFocus();
	});

	it('caps the restore at five pages', async () => {
		historyState.clientsPages = 40;
		await setup(undefined, {
			responder: (path) => {
				const number = Number(/cursor=cursor-(\d+)/.exec(path)?.[1] ?? 1);
				return Promise.resolve(jsonResponse({ items: [clients[number % 4]], nextCursor: `cursor-${number + 1}`, hasMore: true }));
			}
		});

		await expect.element(testPage.getByRole('button', { name: 'Load more' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledTimes(5);
	});
});

const kept = () => vi.mocked(snapshot).mock.calls.at(-1)![0] as {
	capture: () => ListReturn;
	restore: (held: ListReturn) => void;
};

// #2840: Back from a Client puts the list where it was, as the Invoices list
// does (#2746): the same rows, how far down she was, focus on the row.
describe('coming back from a Client', () => {
	const pages = [
		{ items: [clients[0]], nextCursor: 'cursor-2', hasMore: true },
		{ items: [clients[1]], nextCursor: 'cursor-3', hasMore: true },
		{ items: [clients[2]], hasMore: false }
	];
	const answerPages = (path: string) => {
		const index = path.includes('cursor=cursor-3') ? 2 : (path.includes('cursor=cursor-2') ? 1 : 0);
		return Promise.resolve(jsonResponse(pages[index]));
	};
	const hubAddress = (index: number) =>
		new URL(`https://example.test/practices/${practiceId}/clients/${clients[index].clientId}`);
	const place = { pages: 3, scrollY: 120, rowId: clients[2].clientId };

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('keeps the pages open, how far down she was and the Client she left for', async () => {
		await setup(undefined, { responder: answerPages });
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).toBeVisible();
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('rowheader', { name: clients[1].name })).toBeVisible();

		navigation.onBefore({ to: { url: hubAddress(1) } });

		expect(kept().capture()).toEqual({ pages: 2, scrollY: window.scrollY, rowId: clients[1].clientId });
	});

	it('scrolls back to where she was once the pages have landed', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		historyState.clientsPages = 3;
		await setup(undefined, { responder: answerPages });
		kept().restore(place);

		await expect.element(testPage.getByRole('rowheader', { name: clients[2].name })).toBeVisible();
		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
	});

	it('scrolls when the place arrives after the pages have landed', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		historyState.clientsPages = 3;
		await setup(undefined, { responder: answerPages });
		await expect.element(testPage.getByRole('rowheader', { name: clients[2].name })).toBeVisible();

		kept().restore(place);

		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
		await expect.element(testPage.getByRole('link', { name: clients[2].name })).toHaveFocus();
	});

	it('drops the place when the read fails', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		await setup(textResponse('no'));
		kept().restore(place);
		await expect.element(testPage.getByRole('button', { name: 'Try again' })).toBeVisible();

		expect(scrollTo).not.toHaveBeenCalled();
	});

	it('drops the place when she moves to another view before the pages land', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		historyState.clientsPages = 3;
		await setup(undefined, { responder: answerPages });
		kept().restore(place);
		navigation.onBefore({ to: { url: new URL(`https://example.test/practices/${practiceId}/clients?all=true`) } });

		await expect.element(testPage.getByRole('rowheader', { name: clients[2].name })).toBeVisible();
		expect(scrollTo).not.toHaveBeenCalled();
	});
});

// What a filter has already said is not said again down the page: one Doula
// is named in the count line, and "no Contract" is the filter's own words.
const columnNames = () =>
	testPage
		.getByRole('columnheader')
		.elements()
		.map((header) => header.textContent);
const typicalAnswer = (path: string) => Promise.resolve(typical.respond!(path));

describe('what the applied filter has already said', () => {
	const [line] = clients[0].openEngagements ?? [];
	const bareLine = {
		...line,
		engagementId: 'engagement-bare',
		kind: 'birth' as const,
		contractStatus: undefined,
		invoiceStatus: undefined,
		invoiceAmountCents: undefined,
		daysOverdue: undefined,
		feeCents: undefined
	};
	const withNoContract = () => jsonResponse({ items: [{ ...clients[1], hasWork: true, openEngagements: [bareLine] }], hasMore: false });

	it('leaves the Doula column out under one Doula, and the count line names her', async () => {
		pageState.url.searchParams.set('doula', kimiko.value);
		await setup(undefined, { doulas, responder: typicalAnswer });

		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		expect(columnNames()).toEqual(['Name', 'Status', 'Money', 'Date']);
		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent("Showing Kimiko Nakashima's 8 Clients, soonest due first, then most recently ended.");
	});

	it('keeps the Doula column with no Doula chosen', async () => {
		await setup(undefined, { doulas, responder: typicalAnswer });

		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		expect(columnNames()).toEqual(['Name', 'Status', 'Money', 'Date', 'Doula']);
	});

	it('leaves "No Contract yet" out of the Status lines under the no-Contract filter', async () => {
		pageState.url.searchParams.set('noContract', 'true');
		await setup(withNoContract());

		await expect.element(testPage.getByRole('rowheader', { name: clients[1].name })).toBeVisible();
		await expect.element(table().getByText('No Contract yet')).not.toBeInTheDocument();
		await expect.element(table().getByRole('row').nth(1)).not.toHaveTextContent(/Contract/);
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing 1 active Client with no Contract,/);
	});

	it('says "No Contract yet" on a line without one when the filter is off', async () => {
		await setup(withNoContract());

		await expect.element(table().getByText('No Contract yet')).toBeVisible();
	});

	it('leaves the Money column out under the no-Contract filter, as no such line holds any', async () => {
		pageState.url.searchParams.set('noContract', 'true');
		await setup(withNoContract());

		await expect.element(testPage.getByRole('rowheader', { name: clients[1].name })).toBeVisible();
		expect(columnNames()).toEqual(['Name', 'Status', 'Date', 'Doula']);
	});

	it('draws the same set on the record card at 375px', async () => {
		pageState.url.searchParams.set('noContract', 'true');
		await testPage.viewport(375, 740);
		await setup(withNoContract(), { doulas });
		await testPage.viewport(375, 740);
		pageState.url.searchParams.delete('noContract');

		const record = testPage.getByRole('group', { name: 'Clients', exact: true }).element().firstElementChild as HTMLElement;
		expect(record.textContent).not.toContain('No Contract yet');
		expect(record.textContent).not.toContain('Money');
		expect(record.textContent).toContain('Doula: ');
	});

	it('leaves the Doula out of the record card under one Doula', async () => {
		pageState.url.searchParams.set('doula', kimiko.value);
		await renderListScreen({ width: 375, height: 740 });

		const record = testPage.getByRole('group', { name: 'Clients', exact: true }).element().firstElementChild as HTMLElement;
		expect(record.textContent).toContain('Date: ');
		expect(record.textContent).not.toContain('Doula: ');
	});

	it.each([
		['one Doula', 'doula', kimiko.value],
		['the no-Contract filter', 'noContract', 'true']
	])('keeps the rows of one height under %s', async (_name, key, value) => {
		pageState.url.searchParams.set(key, value);
		await renderListScreen({ width: 1366, height: 900 });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
		await document.fonts.ready;

		const heights = rowHeights();
		expect(heights.length).toBe(8);
		expect(heights.filter((height) => Math.abs(height - 40) > 4), `row heights ${heights.map((height) => Math.round(height))}`).toStrictEqual([]);
	});

	it('puts the first card whole on a 375px first screen for a Doula on her own Clients, with no allowance for the tag', async () => {
		pageState.url.searchParams.set('mine', 'true');
		await renderListScreen({ width: 375, height: 740, rosterDoulas: [] });

		const group = testPage.getByRole('group', { name: 'Clients', exact: true });
		await expect.element(group).toBeVisible();
		const first = group.element().firstElementChild!.getBoundingClientRect();
		expect(first.bottom, `first card ${Math.round(first.top)}-${Math.round(first.bottom)}`).toBeLessThanOrEqual(740);
	});

	it.each([
		['one Doula', 'doula', kimiko.value],
		['the no-Contract filter', 'noContract', 'true']
	])('puts the first card whole on a 375px first screen under %s', async (_name, key, value) => {
		pageState.url.searchParams.set(key, value);
		await renderListScreen({ width: 375, height: 740 });

		const group = testPage.getByRole('group', { name: 'Clients', exact: true });
		await expect.element(group).toBeVisible();
		const first = group.element().firstElementChild!.getBoundingClientRect();
		expect(first.bottom, `first card ${Math.round(first.top)}-${Math.round(first.bottom)}`).toBeLessThanOrEqual(740 + TAG_ROW);
	});
});

// Where an element's words start, apart from the box that holds them.
const wordsLeft = (element: Element) => {
	const range = document.createRange();
	range.selectNodeContents(element);
	return range.getBoundingClientRect().left;
};

describe('the table’s first column', () => {
	// The page's edge: the title, the filter bar and the names start at one x.
	it('starts its names where the title and the filter bar start', async () => {
		await renderListScreen({ width: 1280, height: 900 });
		await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();

		const title = wordsLeft(testPage.getByRole('heading', { level: 1 }).element());
		const heading = wordsLeft(testPage.getByRole('columnheader', { name: 'Name' }).element());
		const name = wordsLeft(testPage.getByRole('link', { name: TYPICAL_CLIENTS[0].name }).element());
		const bar = rectOf(testPage.getByRole('group', { name: 'Choose which Clients to show' })).left;
		const grid = rectOf(testPage.getByRole('table')).left;
		expect(Math.abs(name - title), `title ${title}, name ${name}`).toBeLessThanOrEqual(1);
		expect(Math.abs(heading - title)).toBeLessThanOrEqual(1);
		expect(Math.abs(grid - bar)).toBeLessThanOrEqual(1);
	});
});

describe('the amount on a record card', () => {
	it('keeps "$1,500.00" and its "owed" each whole at 375px', async () => {
		await renderListScreen({ width: 375, height: 740 });

		const group = testPage.getByRole('group', { name: 'Clients', exact: true });
		await expect.element(group).toBeVisible();
		const amount = group.getByText('$1,500.00').first().element();
		expect(amount.getClientRects().length).toBe(1);
		const state = [...document.querySelectorAll<HTMLElement>('.record-money .state')].find((element) => element.checkVisibility() && element.textContent?.trim().startsWith('owed'))!;
		const word = [...state.childNodes].find((node) => node.nodeType === Node.TEXT_NODE && node.textContent === 'owed')!;
		const range = document.createRange();
		range.selectNodeContents(word);
		expect(range.getClientRects().length).toBe(1);
	});

	// A dot between two facts only ever follows a fact on its line: where the
	// money line wraps, the dot never opens the new line. Nothing in the
	// accessible tree says where a line breaks (the dot is aria-hidden), so
	// the separators are read from the DOM.
	it.each([320, 340, 360, 375, 390, 420])('never starts a line with the separator dot at %ipx', async (width) => {
		await renderListScreen({ width, height: 740, respond: () => jsonResponse({ items: seedLikeClients, hasMore: false }) });
		const group = testPage.getByRole('group', { name: 'Clients', exact: true });
		await expect.element(group).toBeVisible();
		await document.fonts.ready;

		const dots = [...group.element().querySelectorAll('.separator')];
		expect(dots.length).toBeGreaterThan(0);
		const starting = dots.filter((dot) => {
			const dotBox = dot.getBoundingClientRect();
			const line = dot.closest('.facts')!;
			const lineStart = line.getBoundingClientRect().left;
			return dotBox.left - lineStart < dotBox.width;
		});
		expect(starting.map((dot) => dot.closest('li')?.textContent)).toStrictEqual([]);
	});
});

describe('the Name held for the Client hub’s Back link', () => {
	it('is the Name the list was left with', async () => {
		historyState.clientsName = 'Camille';
		await setup();

		await expect.element(nameField()).toHaveValue('Camille');
		expect(heldClientsName()).toBe('Camille');
	});
});

// The page's pixels, not its tokens': a token that holds a container unit
// (`--space-2`) resolves where it is used, so it is read inside the cell it
// spaces.
function tokenPixels(token: string, inside: Element) {
	const probe = document.createElement('div');
	probe.style.position = 'absolute';
	probe.style.inlineSize = `var(${token})`;
	inside.append(probe);
	const { width } = probe.getBoundingClientRect();
	probe.remove();
	return width;
}

const seedLikePage = () => jsonResponse({ items: seedLikeClients, hasMore: false });

async function renderSeedLikeTable(width: number, height = 900, isDoula = true) {
	await renderListScreen({ width, height, respond: seedLikePage, isDoula });
	await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
	await document.fonts.ready;
}

describe('the columns of a busy page at 1366px and up', () => {
	const DAY = /^[A-Z][a-z]{2} \d{1,2}, \d{4}$/;

	it.each([1366, 1440, 1920])('keeps every date a gutter clear of the Doula’s name, on rows of one height, at %ipx', async (width) => {
		await renderSeedLikeTable(width);

		const days = table().getByText(DAY).elements();
		expect(days.length).toBeGreaterThan(0);
		for (const day of days) {
			const cell = day.closest('td')!;
			const words = cell.nextElementSibling!.querySelector('.doula-names')!;
			const gutter = wordsLeft(words) - day.getBoundingClientRect().right;
			expect(gutter, `"${day.textContent}" is ${gutter}px from the Doula’s name`).toBeGreaterThanOrEqual(tokenPixels('--space-1', cell) - 0.5);
		}
		const heights = rowHeights();
		expect(heights.length).toBe(8);
		expect(heights.filter((height) => Math.abs(height - 40) > 4), `row heights ${heights.map((height) => Math.round(height))}`).toStrictEqual([]);
	});

	it('still puts six whole rows on the first screen at 1366×800', async () => {
		await renderSeedLikeTable(1366, 800, false);

		const header = rectOf(testPage.getByRole('columnheader').first());
		expect(header.bottom + 6 * 40, `table top ${header.top}`).toBeLessThanOrEqual(800);
		expect(rowHeights().length).toBe(8);
	});

	it.each([1280, 1920])('ends the Money header where the amounts end, at %ipx', async (width) => {
		await renderSeedLikeTable(width);

		const header = testPage.getByRole('columnheader', { name: 'Money' }).element();
		const words = document.createRange();
		words.selectNodeContents(header);
		const amounts = table()
			.getByText(/^\$[\d,]+\.\d{2}$/)
			.elements();
		expect(amounts.length).toBeGreaterThan(0);
		for (const amount of amounts) {
			expect(Math.abs(words.getBoundingClientRect().right - amount.getBoundingClientRect().right)).toBeLessThanOrEqual(1);
		}
	});

	// A filter that leaves no money, and one that names a Doula, draw a column
	// fewer: the columns before it start where they started.
	it('starts Status at one x whether or not Money and the Doula are drawn', async () => {
		const withoutMoney = seedLikeClients.map((client) => ({
			...client,
			owedCents: undefined,
			owedInvoiceCount: undefined,
			openEngagements: client.openEngagements?.map((line) => ({
				...line,
				invoiceStatus: undefined,
				invoiceAmountCents: undefined,
				daysOverdue: undefined
			}))
		}));
		const starts: [string, number, boolean][] = [];
		for (const [name, queries, items] of [
			['default', {}, seedLikeClients],
			['no Contract', { noContract: 'true' }, withoutMoney],
			['a Doula', { doula: kimiko.value }, seedLikeClients]
		] as const) {
			for (const [key, value] of Object.entries(queries)) pageState.url.searchParams.set(key, value);
			await renderListScreen({ width: 1440, height: 900, respond: () => jsonResponse({ items, hasMore: false }) });
			await expect.element(testPage.getByRole('rowheader', { name: TYPICAL_CLIENTS[0].name })).toBeVisible();
			await document.fonts.ready;
			const isMoneyDrawn = testPage.getByRole('columnheader', { name: 'Money' }).elements().length > 0;
			const isDoulaDrawn = testPage.getByRole('columnheader', { name: 'Doula' }).elements().length > 0;
			starts.push([name, rectOf(testPage.getByRole('columnheader', { name: 'Status' })).left, isMoneyDrawn]);
			expect({ name, isMoneyDrawn, isDoulaDrawn }).toStrictEqual({
				name,
				isMoneyDrawn: name !== 'no Contract',
				isDoulaDrawn: name !== 'a Doula'
			});
			cleanup();
			for (const key of Object.keys(queries)) pageState.url.searchParams.delete(key);
		}
		const [, first] = starts[0]!;
		for (const [name, left] of starts) expect(Math.abs(left - first), `Status starts at ${left} under ${name}, ${first} by default`).toBeLessThanOrEqual(1);
	});
});

describe('the table under the filter bar', () => {
	it('sits further from the controls than the bar’s own rows do', async () => {
		await renderSeedLikeTable(1280, 800);

		// Case 1 of svelte-tests.md: two rows of one bar, told apart by what they hold, not a role.
		const form = testPage.getByRole('button', { name: 'Apply' }).element().closest('form')!;
		const switches = form.querySelector('.switches')!.getBoundingClientRect();
		const fields = form.querySelector('.fields')!.getBoundingClientRect();
		const head = rectOf(testPage.getByRole('columnheader').first());
		const within = fields.top - switches.bottom;
		const apart = head.top - form.getBoundingClientRect().bottom;
		expect(apart, `bar rows ${within}px apart, table ${apart}px from the bar`).toBeGreaterThan(within);
		expect(apart).toBeGreaterThanOrEqual(tokenPixels('--space-related', form) - 0.5);
	});
});

const countAndApply = () => ({
	count: rectOf(testPage.getByRole('status')),
	apply: rectOf(testPage.getByRole('button', { name: 'Apply' })),
	form: testPage.getByRole('button', { name: 'Apply' }).element().closest('form')!.getBoundingClientRect()
});

describe('the count line in a narrow and a wide bar', () => {
	it('takes a row of its own under the controls at 768px, the width of the bar', async () => {
		await renderListScreen({ width: 768, height: 900 });
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing/);

		const { count, apply, form } = countAndApply();
		expect(count.top).toBeGreaterThanOrEqual(apply.bottom);
		expect(count.left).toBeLessThanOrEqual(form.left + 1);
		expect(count.width).toBeGreaterThan(form.width / 2);
	});

	it('takes a row of its own on the frame’s left edge at 1280px, where the bar is wide', async () => {
		await renderListScreen({ width: 1280, height: 900 });
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing/);

		const { count, apply, form } = countAndApply();
		expect(count.top).toBeGreaterThanOrEqual(apply.bottom);
		expect(count.left).toBeLessThanOrEqual(form.left + 1);
	});
});



// #2888: a box, a select and the order apply when changed; the Name waits for Apply.
const settledChange = () => new Promise((resolve) => setTimeout(resolve, 450));

describe('the Clients list applies untyped controls when changed (#2888)', () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it('clears a held Name emptied without an input event when a box changes', async () => {
		goto.mockResolvedValue(undefined);
		historyState.clientsName = 'Camille';
		await setup();
		await expect.element(nameField()).toHaveValue('Camille');

		(nameField().element() as HTMLInputElement).value = '';
		await testPage.getByRole('checkbox', { name: 'Who owe money' }).click();
		await settledChange();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?owes=true`, expect.objectContaining({ state: { clientsName: '' } }));
	});

	it('reads once for a ticked box, keeps focus on it, and sends the Name as drawn', async () => {
		goto.mockResolvedValue(undefined);
		await setup();
		await nameField().fill('Zelda');

		await testPage.getByRole('checkbox', { name: 'Who owe money' }).click();
		expect(goto).not.toHaveBeenCalled();
		await settledChange();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?owes=true`, {
			state: { clientsName: 'Zelda' },
			replaceState: false,
			reset: false
		});
		expect(document.activeElement).toBe(testPage.getByRole('checkbox', { name: 'Who owe money' }).element());
	});

	it('replaces the entry for a second change on the same box, and adds one for another control', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await testPage.getByRole('checkbox', { name: 'Who owe money' }).click();
		await settledChange();
		await testPage.getByRole('checkbox', { name: 'Who owe money' }).click();
		await settledChange();
		await testPage.getByRole('checkbox', { name: 'Active, with no Contract' }).click();
		await settledChange();

		expect(goto.mock.calls.map((call) => (call[1] as { replaceState: boolean }).replaceState)).toEqual([false, true, false]);
	});

	it('applies the Order select when picked', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await testPage.getByRole('combobox', { name: 'Order' }).selectOptions('Name, A to Z');
		await settledChange();

		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?order=name`, expect.objectContaining({ reset: false }));
	});

	it('does not read for the Name until Apply', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await nameField().fill('Zelda');
		await settledChange();

		expect(goto).not.toHaveBeenCalled();
	});

	it('still lets Apply send the bar once', async () => {
		goto.mockResolvedValue(undefined);
		await setup();
		vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });

		await testPage.getByRole('checkbox', { name: 'Who owe money' }).click();
		await applyFilters();
		await vi.advanceTimersByTimeAsync(450);

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients?owes=true`, noName);
	});
});

const nextStepLink = (name: string) => testPage.getByRole('link', { name, exact: true });

describe('a shared link to a Doula, before the roster lands (#2864)', () => {
	const base = `/practices/${practiceId}/clients`;

	it('draws the select disabled from the first paint, holding her id, with no tag and no result line', async () => {
		pageState.url.searchParams.set('doula', lena.value);
		const roster = Promise.withResolvers<typeof doulas>();
		await setup(undefined, { roster: roster.promise });

		await expect.element(doulaField()).toBeDisabled();
		await expect.element(doulaField()).toHaveValue(lena.value);
		await expect.element(testPage.getByRole('option', { name: 'Any Doula' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: /Remove filter: Doula/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('status')).toHaveTextContent('');

		roster.resolve(doulas);

		await expect.element(doulaField()).toBeEnabled();
		await expect.element(doulaField()).toHaveValue(lena.value);
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Doula Lena Vasquez' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing Lena Vasquez's \d+ Clients?/);
	});

	it('says "More filters and order" before and after the roster lands', async () => {
		pageState.url.searchParams.set('doula', lena.value);
		const roster = Promise.withResolvers<typeof doulas>();
		await setup(undefined, { roster: roster.promise });
		await testPage.viewport(375, 900);

		await expect.element(testPage.getByText('More filters and order')).toBeVisible();
		roster.resolve(doulas);
		await expect.element(doulaField()).toBeEnabled();
		await expect.element(testPage.getByText('More filters and order')).toBeVisible();
	});

	it('draws no sentence for an empty list until the roster names her', async () => {
		pageState.url.searchParams.set('doula', lena.value);
		const roster = Promise.withResolvers<typeof doulas>();
		await setup(nobody, { roster: roster.promise });

		await expect.element(testPage.getByRole('status', { name: 'Loading Clients' })).toBeVisible();
		await expect.element(testPage.getByText(/has no Clients/)).not.toBeInTheDocument();

		roster.resolve(doulas);

		await expect.element(testPage.getByText('Lena Vasquez has no Clients. Clear the filters to widen the list.').first()).toBeVisible();
	});

	it('names a Doula who holds no Client from the whole roster, and keeps her in the select', async () => {
		pageState.url.searchParams.set('doula', lena.value);
		await setup(undefined, { doulas: [kimiko], everyDoula: doulas });

		await expect.element(doulaField()).toHaveValue(lena.value);
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Doula Lena Vasquez' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing Lena Vasquez's/);
	});

	it('keeps the select drawn and enabled with "Any Doula" alone when the roster lands empty', async () => {
		await setup(undefined, { canChooseDoula: true });

		await expect.element(doulaField()).toBeEnabled();
		await expect.element(doulaField().getByRole('option', { name: 'Any Doula' })).toBeInTheDocument();
		expect(doulaField().getByRole('option').elements()).toHaveLength(1);
	});

	it('says "that Doula" when the roster could not be read', async () => {
		pageState.url.searchParams.set('doula', lena.value);
		await setup(undefined, { canChooseDoula: true });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Doula that Doula' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing that Doula's/);
	});

	it('is not drawn at all for a reader who may not choose a Doula', async () => {
		await setup(undefined, { canChooseDoula: false });

		await expect.element(doulaField()).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Order', { exact: true })).toBeVisible();
	});

	describe('an empty result offers one next step chosen by what emptied it', () => {
		const nextStep = nextStepLink;

		it('offers every Client for a Name beside other filters, and sends the Name with it', async () => {
			pageState.url.searchParams.set('owes', 'true');
			pageState.url.searchParams.set('order', 'name');
			historyState.clientsName = 'Cam';
			await setup(nobody);

			await testPage.getByRole('button', { name: 'Search every Client for "Cam"' }).click();

			expect(goto).toHaveBeenLastCalledWith(`${base}?all=true&order=name`, { state: { clientsName: 'Cam' } });
		});

		it('offers every Client for a Name alone among those in care', async () => {
			historyState.clientsName = 'Cam';
			await setup(nobody);

			await testPage.getByRole('button', { name: 'Search every Client for "Cam"' }).click();

			expect(goto).toHaveBeenLastCalledWith(`${base}?all=true`, { state: { clientsName: 'Cam' } });
		});

		it('sends a Name over every Client to the search that finds or adds one', async () => {
			pageState.url.searchParams.set('all', 'true');
			historyState.clientsName = 'Cam';
			await setup(nobody);

			await expect.element(nextStep('Find or add a Client').nth(1)).toHaveAttribute('href', `${base}/search`);
			await expect.element(testPage.getByRole('button', { name: /Search every Client/ })).not.toBeInTheDocument();
		});

		it('sends a Name over every Client to adding one while the Practice holds none', async () => {
			pageState.url.searchParams.set('all', 'true');
			historyState.clientsName = 'Cam';
			await setup(nobody, { hasAnyClient: false });

			await expect.element(nextStep('Add a Client').nth(1)).toHaveAttribute('href', `${base}/new?from=clients`);
		});

		it('shows every Client in care without the filters, keeping the order', async () => {
			pageState.url.searchParams.set('owes', 'true');
			pageState.url.searchParams.set('doula', lena.value);
			pageState.url.searchParams.set('order', 'name');
			await setup(nobody, { doulas });

			await expect.element(nextStep('Show every Client in care')).toHaveAttribute('href', `${base}?order=name`);
		});

		it('offers a Doula who opens on Mine her own Clients when other filters emptied it', async () => {
			pageState.url.searchParams.set('mine', 'true');
			pageState.url.searchParams.set('owes', 'true');
			await setup(nobody, { isDoula: true, opensOnMine: true });

			await expect.element(nextStep('Show all your Clients')).toHaveAttribute('href', `${base}?mine=true`);
		});

		it('offers a Doula who opens on Mine every Client in care once Mine is off', async () => {
			pageState.url.searchParams.set('owes', 'true');
			await setup(nobody, { isDoula: true, opensOnMine: true });

			await expect.element(nextStep('Show every Client in care')).toHaveAttribute('href', base);
		});

		it('offers nothing beyond the sentence when Mine alone is empty', async () => {
			pageState.url.searchParams.set('mine', 'true');
			await setup(nobody, { isDoula: true, opensOnMine: true });

			await expect.element(testPage.getByText('You are not the Doula for any Client yet.').first()).toBeVisible();
			await expect.element(testPage.getByRole('link', { name: /^Show / })).not.toBeInTheDocument();
		});

		it('offers nothing beyond the sentence for the default list with nobody in care', async () => {
			await setup(nobody);

			await expect.element(testPage.getByRole('link', { name: /^Show / })).not.toBeInTheDocument();
			await expect.element(testPage.getByRole('button', { name: /^Search every/ })).not.toBeInTheDocument();
		});
	});
});

const owingClient = (owedCents: number, owedInvoiceCount: number, daysOverdue = 0) => ({
	...clients[1]!,
	clientId: 'client-owing',
	name: 'Quinn Owing',
	owedCents,
	owedInvoiceCount,
	oldestOverdueDays: daysOverdue,
	openEngagements: [
		{
			engagementId: 'engagement-owing',
			kind: 'birth',
			engagementStatus: 'active',
			dueDate: '2027-03-14',
			doulaNames: ['Lena Vasquez'],
			invoiceStatus: 'open',
			invoiceAmountCents: 30_000,
			daysOverdue
		}
	]
});
const owingRow = () => testPage.getByRole('row', { name: /Quinn Owing/ });

describe('the Money cell says what a Client owes in all (#2874)', () => {
	const row = owingRow;

	it('says the total and the count of Invoices, owed as a plain word', async () => {
		await setup(jsonResponse({ items: [owingClient(50_000, 2)], hasMore: false }));

		await expect.element(row()).toHaveTextContent(/\$500\.00, owed,\s*2 Invoices/);
	});

	it('says one Invoice in the singular', async () => {
		await setup(jsonResponse({ items: [owingClient(30_000, 1)], hasMore: false }));

		await expect.element(row()).toHaveTextContent(/\$300\.00, owed,\s*1 Invoice(?!s)/);
	});

	it('marks the total overdue, in the red badge with the oldest Invoice’s days, when an open Invoice is late', async () => {
		await setup(jsonResponse({ items: [owingClient(50_000, 2, 12)], hasMore: false }));

		await expect.element(row()).toHaveTextContent(/\$500\.00, 12 days overdue,\s*2 Invoices/);
		await expect.element(row().getByText('12 days overdue', { exact: true })).toBeVisible();
	});

	it('says one day in the singular', async () => {
		await setup(jsonResponse({ items: [owingClient(50_000, 1, 1)], hasMore: false }));

		await expect.element(row()).toHaveTextContent(/\$500\.00, 1 day overdue,\s*1 Invoice/);
	});

	it('keeps the count for a screen reader in the table, where the badge fills the state track, and shows it on a card', async () => {
		await setup(jsonResponse({ items: [owingClient(50_000, 2, 62)], hasMore: false }));

		const counts = [...document.querySelectorAll<HTMLElement>('.owed-total .count')];
		expect(counts.find((element) => element.closest('table'))!.classList.contains('visually-hidden')).toBe(true);
		expect(counts.find((element) => !element.closest('table'))!.classList.contains('visually-hidden')).toBe(false);
	});

	it('says the total once on a card, with its count', async () => {
		await setup(jsonResponse({ items: [owingClient(50_000, 2)], hasMore: false }));
		await testPage.viewport(375, 740);

		// Both trees are in the DOM and the page draws one: only a fact about which one CSS shows (case 1).
		const shown = [...document.querySelectorAll<HTMLElement>('.record-money')].filter((element) => element.checkVisibility());
		expect(shown).toHaveLength(1);
		expect(shown[0]!.textContent).toMatch(/\$500\.00, owed,\s*2 Invoices/);
	});

	it('draws no total for a Client who owes nothing, and keeps her Engagement\'s own words', async () => {
		await setup(jsonResponse({ items: [{ ...owingClient(0, 0), openEngagements: [{ ...owingClient(0, 0).openEngagements[0]!, invoiceStatus: 'paid' }] }], hasMore: false }));

		await expect.element(row()).toHaveTextContent(/\$300\.00, paid/);
		await expect.element(row()).not.toHaveTextContent(/Invoices?\b/);
	});
});

const dueField = () => testPage.getByLabelText('Due', { exact: true });
const days = (name: 'From' | 'To') => testPage.getByLabelText(name, { exact: true });

describe('the "Due" select (#2871)', () => {
	const base = `/practices/${practiceId}/clients`;
	const months = dueMonthOptions(dayIn(new Date(), 'America/New_York'));
	const march = '2027-03';

	it('offers "Any time", this month and the next nine by name, then "Choose days", with nothing on the wire', async () => {
		await setup();

		await expect.element(dueField()).toHaveValue('');
		await expect.element(dueField()).toHaveAttribute('name', 'due');
		const options = dueField().getByRole('option').elements().map((option) => [(option as HTMLOptionElement).value, option.textContent]);
		expect(options).toEqual([['', 'Any time'], ...months.map((month) => [month.value, month.label]), ['choose', 'Choose days']]);
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('applies a month when changed, so "Priya has room in March" is two actions', async () => {
		goto.mockResolvedValue(undefined);
		await setup(undefined, { doulas });

		await doulaField().selectOptions(lena.label);
		await settledChange();
		await dueField().selectOptions(months[4]!.label);
		await settledChange();

		expect(goto).toHaveBeenCalledTimes(2);
		expect(goto).toHaveBeenLastCalledWith(`${base}?doula=${lena.value}&due=${months[4]!.value}`, expect.objectContaining({ reset: false }));
	});

	it('opens From and To on "Choose days" and sends nothing, starting them on this month\'s days', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await expect.element(days('From')).not.toBeInTheDocument();
		await dueField().selectOptions('Choose days');
		await settledChange();

		expect(goto).not.toHaveBeenCalled();
		await expect.element(days('From')).toHaveValue(dueMonthDays(months[0]!.value).from);
		await expect.element(days('To')).toHaveValue(dueMonthDays(months[0]!.value).to);
		await expect.element(testPage.getByText('Both days are included.')).toBeVisible();
	});

	it('starts chosen days on the month that was applied', async () => {
		pageState.url.searchParams.set('due', march);
		await setup();

		await dueField().selectOptions('Choose days');

		await expect.element(days('From')).toHaveValue('2027-03-01');
		await expect.element(days('To')).toHaveValue('2027-03-31');
	});

	it('waits for Apply to send typed days, in the address as `dueFrom` and `dueTo`', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await dueField().selectOptions('Choose days');
		await days('From').fill('2027-03-01');
		await days('To').fill('2027-03-15');
		await settledChange();
		expect(goto).not.toHaveBeenCalled();
		await applyFilters();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${base}?dueFrom=2027-03-01&dueTo=2027-03-15`, noName);
	});

	it('says what is wrong with typed days beside them, sends nothing, and goes to the field', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await dueField().selectOptions('Choose days');
		await days('To').fill('');
		await applyFilters();

		await expect.element(testPage.getByText('Enter a To day')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
		await vi.waitFor(() => expect(document.activeElement).toBe(days('To').element()));
	});

	it('does not send another control\'s change while the days are not a range', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await dueField().selectOptions('Choose days');
		await days('From').fill('');
		await owing().click();
		await settledChange();

		expect(goto).not.toHaveBeenCalled();
		await expect.element(testPage.getByText('Enter a From day')).toBeVisible();
	});

	it('sends another control\'s change with the typed days once they are a range', async () => {
		goto.mockResolvedValue(undefined);
		await setup();

		await dueField().selectOptions('Choose days');
		await owing().click();
		await settledChange();

		expect(goto).toHaveBeenCalledWith(
			`${base}?owes=true&dueFrom=${dueMonthDays(months[0]!.value).from}&dueTo=${dueMonthDays(months[0]!.value).to}`,
			expect.objectContaining({ reset: false })
		);
	});

	it('reads `?due=`: selected, sent to the BFF, in a tag, in the count line, with the no-care switch left out', async () => {
		pageState.url.searchParams.set('due', march);

		await setup();

		await expect.element(dueField()).toHaveValue(march);
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?due=${march}`);
		await expect.element(tags().getByRole('link', { name: 'Remove filter: Due in March 2027' })).toHaveAttribute('href', base);
		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing \d+ Clients due in March 2027, soonest due first, then most recently ended\.$/);
		await expect.element(testPage.getByRole('checkbox', { name: /Also show/ })).not.toBeInTheDocument();
	});

	it('draws a month the address names beyond the ten, so the select never claims "Any time" for it', async () => {
		pageState.url.searchParams.set('due', '2031-05');

		await setup();

		await expect.element(dueField()).toHaveValue('2031-05');
		await expect.element(dueField().getByRole('option', { name: 'May 2031' })).toBeInTheDocument();
	});

	it('reads `?dueFrom=&dueTo=`: "Choose days" with both days, a tag in fixed words and the days in the request', async () => {
		pageState.url.searchParams.set('dueFrom', '2027-03-01');
		pageState.url.searchParams.set('dueTo', '2027-03-15');

		await setup();

		await expect.element(dueField()).toHaveValue('choose');
		await expect.element(days('From')).toHaveValue('2027-03-01');
		await expect.element(days('To')).toHaveValue('2027-03-15');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients?dueFrom=2027-03-01&dueTo=2027-03-15`);
		await expect.element(tags().getByRole('link', { name: 'Remove filter: Due Mar 1 to Mar 15, 2027' })).toBeVisible();
	});

	it.each([['due', 'March'], ['dueFrom', '2027-03-01']])('reads `?%s=%s` as no narrowing, which the BFF would refuse', async (key, value) => {
		pageState.url.searchParams.set(key, value);

		await setup();

		await expect.element(dueField()).toHaveValue('');
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('is not read for a contractor Doula, whose whole bar is hidden', async () => {
		pageState.url.searchParams.set('due', march);

		await setup(undefined, { isContractor: true, isDoula: true });

		await expect.element(dueField()).not.toBeInTheDocument();
		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients`);
	});

	it('is sent in the body of a Name search, which keeps it out of the address', async () => {
		pageState.url.searchParams.set('due', march);
		historyState.clientsName = 'Camille';

		await setup();

		expect(requestUrl()).toBe(`/api/practices/${practiceId}/clients/list`);
		expect(JSON.parse(requestInit().body as string)).toMatchObject({ name: 'Camille', due: march, dueFrom: '', dueTo: '' });
	});

	it('is removed by its tag, which keeps every other filter', async () => {
		pageState.url.searchParams.set('due', march);
		pageState.url.searchParams.set('owes', 'true');

		await setup();

		await expect.element(tags().getByRole('link', { name: 'Remove filter: Due in March 2027' })).toHaveAttribute('href', `${base}?owes=true`);
		await expect.element(tags().getByRole('link', { name: 'Remove filter: Who owe money' })).toHaveAttribute('href', `${base}?due=${march}`);
	});

	it('puts the Doula in the count line before the month', async () => {
		pageState.url.searchParams.set('due', march);
		pageState.url.searchParams.set('doula', lena.value);

		await setup(undefined, { doulas });

		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing Lena Vasquez's \d+ Clients due in March 2027, soonest due first/);
	});

	describe('an empty result says what was looked for and offers one next step', () => {
		it('offers every Client in care for a month alone', async () => {
			pageState.url.searchParams.set('due', march);
			await setup(nobody);

			await expect.element(testPage.getByText('No Client is due in March 2027. Clear the filters to widen the list.').first()).toBeVisible();
			await expect.element(nextStepLink('Show every Client in care')).toHaveAttribute('href', base);
		});

		it('offers the list with every other filter kept and the month dropped', async () => {
			pageState.url.searchParams.set('due', march);
			pageState.url.searchParams.set('doula', lena.value);
			pageState.url.searchParams.set('order', 'name');
			await setup(nobody, { doulas });

			await expect.element(nextStepLink('Show Clients due any time')).toHaveAttribute('href', `${base}?doula=${lena.value}&order=name`);
		});

		it('offers a Doula who opens on Mine her own Clients, the month dropped', async () => {
			pageState.url.searchParams.set('mine', 'true');
			pageState.url.searchParams.set('due', march);
			await setup(nobody, { isDoula: true, opensOnMine: true });

			await expect.element(nextStepLink('Show all your Clients')).toHaveAttribute('href', `${base}?mine=true`);
		});

		it('keeps Mine for an Owner who ticked it, and drops only the month', async () => {
			pageState.url.searchParams.set('mine', 'true');
			pageState.url.searchParams.set('due', march);
			await setup(nobody, { isDoula: true });

			await expect.element(nextStepLink('Show Clients due any time')).toHaveAttribute('href', `${base}?mine=true`);
		});
	});

	describe('its fold summary', () => {
		it('says "Due and order" for a Doula on Mine, who sees no Doula select and no "Also show" box', async () => {
			pageState.url.searchParams.set('mine', 'true');
			await setup(undefined, { isDoula: true, opensOnMine: true, canChooseDoula: false });
			await testPage.viewport(414, 900);

			await expect.element(testPage.getByText('Due and order', { exact: true })).toBeVisible();
		});

		it('says "More filters and order" otherwise', async () => {
			await setup(undefined, { isDoula: true, canChooseDoula: false });
			await testPage.viewport(414, 900);

			await expect.element(testPage.getByText('More filters and order', { exact: true })).toBeVisible();
		});
	});
});
