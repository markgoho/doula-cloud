import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { toPageState } from '../../../routeFixture.js';
import type { ListReturn } from '#lib/listReturn.js';
import { contractorItems, data, doulas, fixture, items, queue, voidRequests } from './page.fixture.js';
import { pageState } from '#testing/routePage.js';
import { beforeNavigate, goto, snapshot } from '$app/navigation';
import Page from './ContractsScreen.svelte';

vi.mock('$app/state', () => import('#testing/routePage.js'));
if (!customElements.get('center-l')) registerLayoutPrimitives();

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

vi.mock('$app/navigation', () => ({
	invalidate: vi.fn(),
	goto: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	replaceState: vi.fn(),
	snapshot: vi.fn()
}));

const practiceUrl = 'https://example.test/practices/practice-1/contracts';
const sessionOf = (roles: string[], extra: Record<string, unknown> = {}) => ({
	session: { ...(fixture.pageData as { session: object }).session, roles, ...extra }
});

type Properties = Record<string, unknown>;
const ownerProperties = { data, isQueueExpected: true, queue, doulas, isRosterLanded: true };

async function setup({
	query = '?show=all',
	props: properties = ownerProperties,
	session = sessionOf(['owner']),
	width = 1440,
	state = {}
}: { query?: string; props?: Properties; session?: object; width?: number; state?: object } = {}) {
	Object.assign(pageState, toPageState(fixture));
	pageState.url = new URL(`${practiceUrl}${query}`);
	pageState.data = session as Record<string, unknown>;
	pageState.state = state;
	await testPage.viewport(width, 900);
	return render(Page, properties);
}

// Chromium puts a space either side of a visually hidden comma, so words are matched with any spacing between them.
const heard = (text: string) =>
	new RegExp(`^${text.replaceAll(/[$.+]/g, String.raw`\$&`).split(/,? /).join(String.raw`\s*,?\s*`)}$`);
const cell = (name: string | RegExp) => testPage.getByRole('cell', { name });
const radio = (name: string) => testPage.getByRole('radio', { name, exact: true });

const empty = (view: string, extra: Record<string, unknown> = {}) => ({
	...ownerProperties,
	data: { ...data, items: [], total: 0, view, ...extra }
});

async function expectSearchAcrossAll(search: string) {
	await vi.waitFor(() => expect(goto).toHaveBeenCalled());
	const [href, options] = vi.mocked(goto).mock.calls.at(-1)!;
	expect(href).toBe('/practices/practice-1/contracts?show=all');
	expect(href).not.toContain(search);
	expect(options).toMatchObject({ state: { contractsSearch: search } });
}

const kept = () =>
	vi.mocked(snapshot).mock.calls.at(-1)![0] as { capture: () => ListReturn; restore: (held: ListReturn) => void };

const leaveFor = (hash: string) => {
	for (const [leave] of vi.mocked(beforeNavigate).mock.calls) {
		leave({ type: 'link', to: { url: new URL(`https://example.test/practices/practice-1/engagements/eng-1${hash}`) } } as never);
	}
};

const downloads: { click?: ReturnType<typeof vi.spyOn> } = {};

beforeEach(() => {
	apiFetchWithSession.mockReset();
	downloads.click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
	vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:contracts');
	vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
	vi.mocked(goto).mockReset();
});

afterEach(() => {
	vi.restoreAllMocks();
	Object.assign(pageState, toPageState(fixture));
	pageState.state = {};
});

describe('the Practice-wide Contracts list rows', () => {
	it('says each state with its day, once, and a wait as a count of days', async () => {
		await setup();

		await expect.element(cell(heard('Draft made Nov 8'))).toBeVisible();
		await expect.element(cell(heard('Sent Nov 2, waiting 12 days Opened Nov 12'))).toBeVisible();
		await expect.element(cell(heard('Sent today Not opened'))).toBeVisible();
		await expect.element(cell(heard('Signed Dec 30, 2025 Void requested'))).toBeVisible();
		await expect.element(cell(heard('Voided Nov 9'))).toBeVisible();
	});

	it('says who took each Contract’s step: who made a Draft, sent it, voided it', async () => {
		await setup();

		await expect.element(cell('Made by Dee Whitlock')).toBeVisible();
		await expect.element(cell('Sent by Dee Whitlock').first()).toBeVisible();
		await expect.element(cell('Voided by Dee Whitlock')).toBeVisible();
	});

	it('says no one is on record where a backfilled Contract has no name for its step', async () => {
		await setup({
			props: { ...ownerProperties, data: { ...data, items: [{ ...items[1]!, sentByName: undefined }] } }
		});

		await expect.element(cell('No one on record')).toBeVisible();
	});

	it('calls the reader You in the Doula cell, and keeps the other names for a screen reader', async () => {
		await setup({ session: sessionOf(['doula'], { staffId: 'staff-7' }) });

		await expect.element(cell(heard('You, Kimiko Nakashima'))).toBeVisible();
		await expect.element(testPage.getByText('You +1')).toBeVisible();
	});

	it('names every Doula on the Engagement, the first and a count in view, and says none', async () => {
		await setup();

		await expect.element(cell('Marisol Terrazas')).toBeVisible();
		await expect.element(cell('Kimiko Nakashima, Joss Adeyemi')).toBeVisible();
		await expect.element(testPage.getByText('Kimiko Nakashima +1')).toBeVisible();
		await expect.element(cell('No Doula yet').first()).toBeVisible();
	});

	it('draws the Doula word on a narrow card only before a name, and reads it either way', async () => {
		await setup({ width: 375 });

		await expect.element(testPage.getByLabelText('Contracts', { exact: true })).toBeVisible();
		// querySelector case 1: the table and the cards hold the same words; only CSS tells them apart
		const cards = [...document.querySelectorAll('.record')];
		const named = cards.find((card) => card.textContent?.includes('Marisol Terrazas'));
		const none = cards.find((card) => card.textContent?.includes('No Doula yet'));
		expect(named!.querySelectorAll('[aria-hidden="true"]')).toContainEqual(expect.objectContaining({ textContent: 'Doula ' }));
		expect(none!.querySelectorAll('[aria-hidden="true"]')).not.toContainEqual(expect.objectContaining({ textContent: 'Doula ' }));
		expect(none!.textContent).toContain('Doula: No Doula yet');
	});

	it('says the care with its due or ended day, the year only when it is not this one', async () => {
		await setup();

		await expect.element(cell(heard('Birth, Due Jan 13, 2027'))).toBeVisible();
		await expect.element(cell(heard('Postpartum, no due date'))).toBeVisible();
		await expect.element(cell(heard('Birth, Ended Oct 30'))).not.toBeInTheDocument();
		await expect.element(cell(/Birth\s*,?\s*Ended Oct 1/)).toBeVisible();
		await expect.element(cell(/Birth\s*,?\s*Born Oct 30/).first()).toBeVisible();
	});

	it('shows the fee and the Action column (Signed PDF or reminder) to a reader the PDF read serves', async () => {
		await setup();

		await expect.element(testPage.getByRole('columnheader', { name: 'Fee', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('columnheader', { name: 'Action', exact: true })).toBeVisible();
		await expect.element(cell('$1,500.00').first()).toBeVisible();
	});

	it('names each row link by Client, and tells a voided Contract from its replacement and from a queued request', async () => {
		await setup();

		await expect.element(testPage.getByRole('link', { name: 'Quinn Bishop, Birth Contract, signed' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Quinn Bishop, Birth Contract, voided' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Priya Okonkwo-Fitzgerald, void request' })).toBeVisible();
	});

	it('opens the Contract on its Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Rowan Okafor' }))
			.toHaveAttribute('href', '/practices/practice-1/engagements/eng-5#contract-contract-5');
	});

	it('names the table, describes it by the result line and sorts it by Status', async () => {
		await setup();

		const table = testPage.getByRole('table', { name: 'Contracts' });
		await expect.element(table).toHaveAccessibleDescription('Showing 5 Contracts, newest first.');
		await expect
			.element(testPage.getByRole('columnheader', { name: 'Status', exact: true }))
			.toHaveAttribute('aria-sort', 'descending');
	});

	it.each([
		['?show=all&order=oldest', 'all', 'oldest', 'Status', 'ascending'],
		['?show=all&order=client', 'all', 'client', 'Client', 'ascending'],
		['?show=sent', 'sent', '', 'Status', 'ascending'],
		['?show=signed', 'signed', '', 'Status', 'descending']
	])('sorts %s by %s, %s', async (query, view, order, column, direction) => {
		await setup({ query, props: { ...ownerProperties, data: { ...data, view, order } } });

		await expect
			.element(testPage.getByRole('columnheader', { name: column, exact: true }))
			.toHaveAttribute('aria-sort', direction);
	});
});

describe('the Signed PDF', () => {
	it('is offered on a signed row, named as the Engagement names it, and downloads', async () => {
		apiFetchWithSession.mockResolvedValue(new Response(new Blob(['pdf']), { status: 200 }));
		await setup();

		const download = testPage.getByRole('button', { name: 'Download signed Contract (PDF), Quinn Bishop' });
		await expect.element(download).toBeVisible();
		await download.click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-3/contract/pdf');
		await vi.waitFor(() => expect(downloads.click).toHaveBeenCalledOnce());
		await expect.element(testPage.getByRole('status').filter({ hasText: 'Downloaded the Signed PDF for Quinn Bishop.' })).toBeVisible();
		expect(downloads.click!.mock.contexts[0]).toHaveProperty('download', 'signed-contract-quinn-bishop.pdf');
	});

	it('says in place when it did not download', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('refused', 500));
		await setup();

		await testPage.getByRole('button', { name: 'Download signed Contract (PDF), Quinn Bishop' }).click();

		await expect.element(testPage.getByText('The Signed PDF did not download.')).toBeVisible();
	});

	it('is not offered to a contractor Doula, nor is a fee or an amount', async () => {
		const session = sessionOf(['doula'], { isContractor: true });
		await setup({
			session,
			props: { data: { ...data, items: contractorItems }, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByRole('columnheader', { name: 'Client', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('columnheader', { name: 'Fee' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('columnheader', { name: 'Signed PDF' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: /Download/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByText('$1,500.00')).not.toBeInTheDocument();
	});
});

const sentName = items[1]!.clientName;
const remindButton = () => testPage.getByRole('button', { name: `Remind the Client, ${sentName}` });

describe('the reminder (#2951)', () => {

	it('is offered on a sent row that can be reminded, and says when the Client was last reminded', async () => {
		await setup();

		await expect.element(remindButton()).toBeVisible();
		await expect.element(testPage.getByText('Reminded Nov 13', { exact: false }).first()).toBeInTheDocument();
	});

	it('sends the reminder, then says so and takes the control away until tomorrow', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ status: 'sent', reminded: { at: '2026-11-15T15:00:00Z', by: 'Renata Alvarez' } }));
		await setup();

		await remindButton().click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-2/contract/remind', { method: 'POST' });
		await expect.element(testPage.getByRole('status').filter({ hasText: `Reminder on its way to ${sentName}.` })).toBeVisible();
		await expect.element(remindButton()).not.toBeInTheDocument();
	});

	it('says why in place when the reminder was refused', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('This Client was already emailed about a Contract today. You can send a reminder tomorrow.', 409));
		await setup();

		await remindButton().click();

		await expect.element(testPage.getByText('You can send a reminder tomorrow.', { exact: false })).toBeVisible();
	});

	it('is offered as a button on the card of a narrow list', async () => {
		await setup({ width: 600 });

		await expect.element(testPage.getByRole('button', { name: 'Remind the Client' })).toBeVisible();
	});

	it('carries the last-reminded day in the control name, not in a title (#2978)', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: `Remind the Client, ${sentName}, last reminded Nov 13` })).toBeVisible();
		expect(document.querySelector('[title^="Reminded"]')).toBeNull();
	});

	it('offers no control where the BFF would refuse, and says when the Client can next be reminded (#2978)', async () => {
		await setup();

		const refused = items.find((item) => item.canRemindOn)!;
		expect(testPage.getByRole('button', { name: `Remind the Client, ${refused.clientName}` }).elements()).toHaveLength(0);
		await expect.element(testPage.getByText(`Remind the Client Nov 15`, { exact: false })).toBeInTheDocument();
	});

	it('says it in words on the card of a narrow list (#2978)', async () => {
		await setup({ width: 600 });

		await expect.element(testPage.getByText('Remind again Nov 15')).toBeVisible();
	});

	it('is not offered on the Signed view, where nothing waits on the Client', async () => {
		await setup({ query: '?show=signed' });

		await expect.element(testPage.getByRole('columnheader', { name: 'Reminder' })).not.toBeInTheDocument();
	});
});

describe('the void request queue', () => {
	it('counts the requests, then names the Client, who asked, how long ago and why', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: '2 void requests to decide', level: 2 })).toBeVisible();
		const [first, second] = voidRequests;
		await expect
			.element(testPage.getByRole('link', { name: `${first!.clientName}, void request` }))
			.toHaveAttribute('href', `/practices/practice-1/engagements/${first!.engagementId}#void-request-${first!.requestId}`);
		await expect.element(testPage.getByText('Jamie Doula asked 11 days ago', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText(first!.reason)).toBeVisible();
		await expect.element(testPage.getByText('Joss Adeyemi asked today', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText(second!.reason)).toBeVisible();
		expect(testPage.getByRole('listitem').elements().filter((each) => each.id.startsWith('void-request-row-'))).toHaveLength(2);
		await expect
			.element(testPage.getByText('Open one to void its Contract or decline it, both on the Engagement.'))
			.toBeVisible();
	});

	it('is a click target as a whole, through its one link', async () => {
		await setup();
		const [first] = voidRequests;
		const link = testPage.getByRole('link', { name: `${first!.clientName}, void request` }).element() as HTMLAnchorElement;
		const followed = vi.spyOn(link, 'click').mockImplementation(() => {});

		await testPage.getByText(first!.reason).click();
		expect(followed).toHaveBeenCalledTimes(1);

		await testPage.getByText('Jamie Doula asked 11 days ago', { exact: true }).click();
		expect(followed).toHaveBeenCalledTimes(2);
	});

	it('says one request in the singular', async () => {
		await setup({ props: { ...ownerProperties, queue: { ...queue, items: [voidRequests[0]!], total: 1 } } });

		await expect.element(testPage.getByRole('heading', { name: '1 void request to decide' })).toBeVisible();
	});

	it('comes before the filter bar', async () => {
		await setup();

		const heading = testPage.getByRole('heading', { name: '2 void requests to decide' }).element();
		const bar = testPage.getByRole('radio', { name: 'Not signed' }).element();
		expect(heading.compareDocumentPosition(bar) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('says so when none is waiting, with no heading or list', async () => {
		await setup({ props: { ...ownerProperties, queue: { items: [], hasMore: false, total: 0 } } });

		await expect.element(testPage.getByText('No void request is waiting for a decision.')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 2 })).not.toBeInTheDocument();
	});

	it('is not drawn for a reader who cannot decide one', async () => {
		await setup({
			session: sessionOf(['doula']),
			props: { data, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByRole('table', { name: 'Contracts' })).toBeVisible();
		await expect.element(testPage.getByText(/void request/)).not.toBeInTheDocument();
	});

	it('says in place when it did not load, keeps the Contracts, and tries only itself again', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse(queue));
		await setup({ props: { ...ownerProperties, queue: undefined, hasQueueFailed: true } });

		await expect.element(testPage.getByText('The void requests did not load.')).toBeVisible();
		await expect.element(testPage.getByRole('table', { name: 'Contracts' })).toBeVisible();
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByRole('heading', { name: '2 void requests to decide' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledExactlyOnceWith('/api/practices/practice-1/contracts/void-requests');
	});

	it('stays failed when trying again fails', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('down', 500));
		await setup({ props: { ...ownerProperties, queue: undefined, hasQueueFailed: true } });

		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByText('The void requests did not load.')).toBeVisible();
	});

	it('reads the next page of requests on "Load more void requests", and says when that fails', async () => {
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ items: [{ ...voidRequests[1]!, requestId: 'request-9', clientName: 'Ada Next' }], hasMore: false, total: 3 })
		);
		await setup({ props: { ...ownerProperties, queue: { ...queue, hasMore: true, nextCursor: 'q1', total: 3 } } });

		await testPage.getByRole('button', { name: 'Load more void requests' }).click();
		await expect.element(testPage.getByRole('link', { name: 'Ada Next, void request' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/contracts/void-requests?cursor=q1');
	});

	it('says More void requests did not load beside the button, keeping the rows', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('down', 500));
		await setup({ props: { ...ownerProperties, queue: { ...queue, hasMore: true, nextCursor: 'q1' } } });

		await testPage.getByRole('button', { name: 'Load more void requests' }).click();

		await expect.element(testPage.getByText('More void requests did not load. Try again.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Jamie Dunmore, void request' })).toBeVisible();
	});

	it('draws the title, the intro and a skeleton and no bar until the void requests have settled', async () => {
		await setup({ props: { data, isQueueExpected: true, doulas, isRosterLanded: true } });

		await expect.element(testPage.getByRole('heading', { name: 'Contracts', level: 1 })).toBeVisible();
		await expect.element(testPage.getByText('Each Contract you can open, and where it stands.')).toBeVisible();
		await expect.element(testPage.getByRole('status', { name: 'Loading your Contracts' })).toBeVisible();
		await expect.element(radio('Not signed')).not.toBeInTheDocument();
	});

	it('draws the bar from the first frame for a reader with no queue', async () => {
		await setup({ session: sessionOf(['doula']), props: { isQueueExpected: false, doulas: [], isRosterLanded: true } });

		await expect.element(radio('Not signed')).toBeVisible();
		await expect.element(testPage.getByRole('status', { name: 'Loading your Contracts' })).toBeVisible();
	});
});

describe('the filter bar', () => {
	it('offers the six views as one radio group, the whole book checked on All', async () => {
		await setup();

		for (const label of ['Not signed', 'Draft', 'Sent', 'Signed', 'Voided', 'All']) {
			await expect.element(radio(label)).toBeVisible();
		}
		await expect.element(radio('All')).toBeChecked();
		await expect.element(testPage.getByRole('group', { name: 'Show', exact: true })).toBeVisible();
	});

	it('offers the Doula select to an Owner at a Practice with two Doulas', async () => {
		await setup();

		const select = testPage.getByRole('combobox', { name: 'Doula' });
		await expect.element(select).toBeVisible();
		await expect.element(testPage.getByRole('option', { name: 'Any Doula' })).toBeInTheDocument();
		await expect.element(testPage.getByRole('option', { name: 'Priya Raman' })).toBeInTheDocument();
	});

	it('does not offer the Doula select where the Practice has one Doula, or to a Doula', async () => {
		await setup({ props: { ...ownerProperties, doulas: [doulas[0]!] } });
		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).not.toBeInTheDocument();
	});

	it('does not offer the Doula select or Mine to a contractor Doula', async () => {
		await setup({
			session: sessionOf(['doula'], { isContractor: true }),
			props: { data, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).not.toBeInTheDocument();
	});

	it('offers Mine under "Show only" to an employed Doula at a Practice with another Doula, and not the Doula select', async () => {
		await setup({
			session: sessionOf(['doula'], { hasOtherDoulas: true }),
			props: { data, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByRole('group', { name: 'Show only' }).getByRole('checkbox', { name: 'Mine' })).toBeVisible();
		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).not.toBeInTheDocument();
	});

	it('does not offer Mine where she is the only Doula', async () => {
		await setup({
			session: sessionOf(['doula'], { hasOtherDoulas: false }),
			props: { data, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).not.toBeInTheDocument();
	});

	it.each([
		['Draft', '?show=draft'],
		['Sent', '?show=sent'],
		['Signed', '?show=signed'],
		['Voided', '?show=voided'],
		['Not signed', '']
	])('applies the %s view when chosen, in the address', async (label, query) => {
		await setup();

		await radio(label).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(vi.mocked(goto).mock.calls.at(-1)![0]).toBe(`/practices/practice-1/contracts${query}`);
	});

	it.each([
		['Status', '?show=sent', 'sent', '', '?show=sent&order=newest'],
		['Client', '?show=sent', 'sent', '', '?show=sent&order=client'],
		['Status', '?show=sent&order=client', 'sent', 'client', '?show=sent']
	])('sorts by the %s heading from %s (#2955)', async (column, query, view, order, expected) => {
		await setup({ query, props: { ...ownerProperties, data: { ...data, view, order } } });

		await testPage.getByRole('columnheader', { name: column, exact: true }).getByRole('button').click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(vi.mocked(goto).mock.calls.at(-1)![0]).toBe(`/practices/practice-1/contracts${expected}`);
	});

	it('carries the order through a change of view, the view own order left out', async () => {
		await setup({ query: '?show=all&order=oldest', props: { ...ownerProperties, data: { ...data, order: 'oldest' } } });

		await radio('Sent').click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(vi.mocked(goto).mock.calls.at(-1)![0]).toBe('/practices/practice-1/contracts?show=sent');
	});

	it('offers the view own order first and applies a pick', async () => {
		await setup({ query: '?show=sent', props: { ...ownerProperties, data: { ...data, view: 'sent' } } });

		await expect.element(testPage.getByRole('option', { name: 'Longest wait first' })).toBeInTheDocument();
		await testPage.getByRole('combobox', { name: 'Order' }).selectOptions('Client, A to Z');

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(vi.mocked(goto).mock.calls.at(-1)![0]).toBe('/practices/practice-1/contracts?show=sent&order=client');
	});

	it('applies a Doula when chosen and the Mine box when ticked', async () => {
		await setup({ session: sessionOf(['owner', 'doula'], { hasOtherDoulas: true }) });

		await testPage.getByRole('combobox', { name: 'Doula' }).selectOptions('Priya Raman');
		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(vi.mocked(goto).mock.calls.at(-1)![0]).toBe('/practices/practice-1/contracts?show=all&doula=staff-2');

		await testPage.getByRole('checkbox', { name: 'Mine' }).click();
		await vi.waitFor(() => expect(vi.mocked(goto).mock.calls.length).toBeGreaterThan(1));
		expect(vi.mocked(goto).mock.calls.at(-1)![0]).toContain('mine=true');
	});

	it('waits for Apply to send a typed name, which is in no address', async () => {
		await setup();

		await testPage.getByRole('searchbox', { name: 'Client' }).fill('quinn');
		expect(goto).not.toHaveBeenCalled();
		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expectSearchAcrossAll('quinn');
	});
});

describe('what is applied', () => {
	it('says each narrowing as a removable tag, and clears one or all', async () => {
		await setup({
			query: '?show=sent&mine=true&doula=staff-2',
			session: sessionOf(['owner', 'doula'], { hasOtherDoulas: true }),
			props: { ...ownerProperties, data: { ...data, view: 'sent', isMine: true, doula: 'staff-2' } },
			state: { contractsSearch: 'quinn' }
		});
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [], hasMore: false, total: 0 }));

		const remove = (name: string) => testPage.getByRole('link', { name: `Remove filter: ${name}` });
		await expect.element(remove('Sent Contracts')).toHaveAttribute('href', '/practices/practice-1/contracts?mine=true&doula=staff-2');
		await expect.element(remove('Mine')).toHaveAttribute('href', '/practices/practice-1/contracts?show=sent&doula=staff-2');
		await expect.element(remove('Doula Priya Raman')).toHaveAttribute('href', '/practices/practice-1/contracts?show=sent&mine=true');
		await expect.element(remove('Matching quinn')).toHaveAttribute('href', '/practices/practice-1/contracts?show=sent&mine=true&doula=staff-2');
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', '/practices/practice-1/contracts');

		await remove('Matching quinn').click();
		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(vi.mocked(goto).mock.calls.at(-1)![1]).toMatchObject({ state: { contractsSearch: '' } });
	});

	it('draws no tag for the default view or All, and no Clear link until something is applied', async () => {
		await setup();

		await expect.element(testPage.getByRole('link', { name: /Remove filter/ })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toBeVisible();
	});

	it('draws no Clear link on the default view', async () => {
		await setup({ query: '', props: { ...ownerProperties, data: { ...data, view: 'not-signed' } } });

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});
});

describe('the result line and what an empty one offers', () => {

	it.each([
		['not-signed', '', 'No Contract is waiting to be signed.'],
		['draft', '?show=draft', 'No Draft Contract is waiting to be sent.'],
		['sent', '?show=sent', 'No Sent Contract is waiting on a Client.'],
		['signed', '?show=signed', 'No Contract is signed.'],
		['voided', '?show=voided', 'No Contract has been voided.']
	])('says nothing in the %s view and offers every Contract', async (view, query, words) => {
		await setup({ query, props: empty(view) });

		await expect.element(testPage.getByText(words).first()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Show all Contracts' }))
			.toHaveAttribute('href', '/practices/practice-1/contracts?show=all');
	});

	it('says the whole book is empty with no bar and no action', async () => {
		await setup({ props: empty('all') });

		await expect
			.element(testPage.getByText('No Contracts yet. One appears here when a Contract is made on an Engagement.'))
			.toBeVisible();
		await expect.element(radio('All')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: 'Show all Contracts' })).not.toBeInTheDocument();
	});

	it('says a contractor Doula has no Contract on her Engagements yet, with no action', async () => {
		await setup({
			session: sessionOf(['doula'], { isContractor: true }),
			props: { data: { ...data, items: [], total: 0 }, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByText('No Contract on your Engagements yet.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Show all Contracts' })).not.toBeInTheDocument();
	});

	it('offers everyone Contracts to a Doula whose own come to nothing', async () => {
		await setup({
			query: '?show=all&mine=true',
			session: sessionOf(['doula'], { hasOtherDoulas: true }),
			props: { data: { ...data, items: [], total: 0, isMine: true }, isQueueExpected: false, doulas: [], isRosterLanded: true }
		});

		await expect.element(testPage.getByText('No Contract of yours.').first()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Show everyone's Contracts" }))
			.toHaveAttribute('href', '/practices/practice-1/contracts?show=all');
	});

	it('runs a search that found nothing in a view again over every Contract, keeping the words typed', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [], hasMore: false, total: 0 }));
		await setup({ query: '?show=sent', props: empty('sent'), state: { contractsSearch: 'zed' } });

		const action = testPage.getByRole('button', { name: 'Search all Contracts for "zed"' });
		await expect.element(action).toBeVisible();
		await action.click();

		await expectSearchAcrossAll('zed');
	});
});

describe('the Contracts read failing', () => {
	it('says what happened in place, keeps the bar and the view, and tries again', async () => {
		const onRetry = vi.fn(async () => {});
		await setup({ query: '?show=sent', props: { ...ownerProperties, data: undefined, hasReadFailed: true, onRetry } });

		await expect.element(testPage.getByText('Your Contracts did not load.')).toBeVisible();
		await expect.element(radio('Sent')).toBeChecked();
		await testPage.getByRole('button', { name: 'Try again' }).click();

		expect(onRetry).toHaveBeenCalledOnce();
	});

	it('says when a search did not run, and runs it again', async () => {
		apiFetchWithSession.mockRejectedValueOnce(new Error('down')).mockResolvedValueOnce(jsonResponse({ items: [items[2]], hasMore: false, total: 1 }));
		await setup({ props: { ...ownerProperties, data: undefined }, state: { contractsSearch: 'quinn' } });

		await expect.element(testPage.getByText('Your search did not run.')).toBeVisible();
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Quinn Bishop' })).toBeVisible();
	});

	it('keeps the rows read and says beside the button when more did not load', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('down', 500));
		await setup({ props: { ...ownerProperties, data: { ...data, hasMore: true, nextCursor: 'c1' } } });

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByText('More Contracts did not load. Try again.')).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Rowan Okafor' })).toBeVisible();
	});

	it('reads the next page under the view and order the rows were read in', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [], hasMore: false, total: 5 }));
		await setup({
			query: '?show=signed&order=client',
			props: { ...ownerProperties, data: { ...data, view: 'signed', order: 'client', hasMore: true, nextCursor: 'c1' } }
		});

		await testPage.getByRole('button', { name: 'Load more' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/contracts?view=signed&order=client&cursor=c1');
	});
});

describe('a typed name', () => {
	it('is read by a POST with the name in the body and in no address, and says so in the result line', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [items[2]], hasMore: false, total: 1 }));
		await setup({ props: { ...ownerProperties, data: undefined }, state: { contractsSearch: 'quinn' } });

		await expect.element(testPage.getByRole('link', { name: 'Quinn Bishop' })).toBeVisible();
		const [path, init] = apiFetchWithSession.mock.calls[0]!;
		expect(path).toBe('/api/practices/practice-1/contracts/list');
		expect(JSON.parse(init.body)).toMatchObject({ view: 'all', search: 'quinn' });
		await expect.element(testPage.getByText('Showing 1 Contract matching "quinn", newest first.')).toBeVisible();
	});
});

class HistoryState {
	contractsSearch = $state<string>();
}

describe('a search removed before it lands', () => {
	it('leaves the unsearched rows and the result line, and the table not busy', async () => {
		const history = new HistoryState();
		let release!: (response: Response) => void;
		apiFetchWithSession.mockReturnValue(new Promise<Response>((resolve) => (release = resolve)));
		await setup({ state: history });

		history.contractsSearch = 'alma';
		await vi.waitFor(() => expect(apiFetchWithSession).toHaveBeenCalledOnce());
		history.contractsSearch = '';
		release(jsonResponse({ items: [items[2]], hasMore: false, total: 1 }));

		const table = testPage.getByRole('table', { name: 'Contracts' });
		await expect.element(table).toHaveAttribute('aria-busy', 'false');
		await expect.element(table).toHaveAccessibleDescription('Showing 5 Contracts, newest first.');
		await expect.element(testPage.getByRole('link', { name: 'Rowan Okafor' })).toBeVisible();
	});
});

// T2, T3 (#2749): Back from an Engagement puts the list where it was left, with focus on the row.
describe('coming back from an Engagement', () => {

	it('keeps the Contract she left for, and a void request she left for', async () => {
		await setup();

		leaveFor('#contract-contract-3');
		expect(kept().capture()).toEqual({ pages: 1, scrollY: 0, rowId: 'contract-3' });
		leaveFor('#void-request-request-1');
		expect(kept().capture().rowId).toBe('vr:request-1');
		leaveFor('#messages');
		expect(kept().capture().rowId).toBeUndefined();
	});

	it('scrolls back and focuses the Contract row link', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		await setup();

		kept().restore({ pages: 1, scrollY: 120, rowId: 'contract-3' });

		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
		await expect.element(testPage.getByRole('link', { name: 'Quinn Bishop, Birth Contract, signed' })).toHaveFocus();
		scrollTo.mockRestore();
	});

	it('focuses the void request she left from', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		await setup();

		kept().restore({ pages: 1, scrollY: 0, rowId: 'vr:request-2' });

		await expect.element(testPage.getByRole('link', { name: 'Jamie Dunmore, void request' })).toHaveFocus();
		scrollTo.mockRestore();
	});

	it('only scrolls when the row has left the view', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		await setup();

		kept().restore({ pages: 1, scrollY: 80, rowId: 'gone' });

		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 80));
		expect(document.activeElement).toBe(document.body);
		scrollTo.mockRestore();
	});

	it('does not restore a place over a read that failed', async () => {
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		await setup({ props: { ...ownerProperties, data: undefined, hasReadFailed: true } });

		kept().restore({ pages: 1, scrollY: 80, rowId: 'contract-3' });

		expect(scrollTo).not.toHaveBeenCalled();
		scrollTo.mockRestore();
	});
});

describe('the columns each view draws', () => {
	const HEADERS = ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee', 'Reminder', 'Signed PDF', 'Action'];
	it.each([
		['', 'not-signed', ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee', 'Reminder']],
		['?show=draft', 'draft', ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee']],
		['?show=sent', 'sent', ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee', 'Reminder']],
		['?show=signed', 'signed', ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee', 'Signed PDF']],
		['?show=voided', 'voided', ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee']],
		['?show=all', 'all', ['Client', 'Status', 'Who', 'Doula', 'Care', 'Fee', 'Action']]
	])('draws only the columns that can hold something in %j', async (query, view, drawn) => {
		await setup({ query, props: { ...ownerProperties, data: { ...data, view } } });

		for (const header of HEADERS) {
			const column = testPage.getByRole('columnheader', { name: header, exact: true });
			if (drawn.includes(header)) await expect.element(column).toBeVisible();
			else await expect.element(column).not.toBeInTheDocument();
		}
	});
});
