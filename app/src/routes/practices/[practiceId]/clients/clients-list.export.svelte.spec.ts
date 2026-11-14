import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { goto, pageState } from '#testing/routePage.js';
import { practiceSession, toPageState } from '../../../routeFixture.js';
import { clients, fixture } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

vi.mock('$app/state', () => import('#testing/routePage.js'));
vi.mock('$app/navigation', async () => ({
	...(await import('#testing/routePage.js')),
	replaceState: vi.fn(),
	snapshot: vi.fn(),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn()
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId } = fixture.params;
const csv = (count: string, name = 'clients-2026-11-14.csv') =>
	new Response('Client\n', { headers: { 'Content-Disposition': `attachment; filename="${name}"`, 'Client-Count': count } });

const button = () => testPage.getByRole('button', { name: 'Download CSV' });

beforeEach(() => {
	cleanup();
	vi.restoreAllMocks();
	apiFetchWithSession.mockReset();
	goto.mockReset();
	Object.assign(pageState, toPageState(fixture));
	pageState.url = new URL(fixture.url!);
	pageState.state = {};
});

interface Caller {
	list?: { items?: unknown[]; hasMore?: boolean; clientCount?: number };
	isContractor?: boolean;
}

async function setup({ list = {}, isContractor = false }: Caller = {}) {
	apiFetchWithSession.mockImplementation(async (path: string) => {
		if (path.endsWith('/export')) return reply.response;
		return jsonResponse({ items: clients, hasMore: false, ...list });
	});
	await testPage.viewport(1440, 900);
	await render(Page, {
		data: {
			isContractor,
			isOwner: true,
			isDoula: false,
			hasOtherDoulas: true,
			hasMine: Promise.resolve(true),
			hasAnyClient: Promise.resolve(true),
			doulas: Promise.resolve([]),
			everyDoula: Promise.resolve([]),
			canChooseDoula: false,
			opensOnMine: false,
			session: { practiceId, staffId: 'staff-1', practiceName: 'Riverside Doula Collective', roles: [], isContractor, timezone: 'America/New_York' }
		}
	});
}

const reply: { response?: Response } = {};

async function download(response: Response) {
	reply.response = response;
	const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:x');
	vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
	const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
	await button().click();
	return { create, click };
}

describe('the Download CSV button (#2891)', () => {
	it('says in place what it handed over, with the server’s file name and count', async () => {
		await setup();
		const { create, click } = await download(csv('16'));

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Downloaded clients-2026-11-14.csv, 16 Clients.' })).toBeVisible();
		expect(create).toHaveBeenCalledOnce();
		expect(click).toHaveBeenCalledOnce();
	});

	it('says one Client in the singular and nothing after a refusal', async () => {
		await setup();
		await download(csv('1'));
		await expect.element(testPage.getByText('Downloaded clients-2026-11-14.csv, 1 Client.')).toBeVisible();

		await download(new Response('', { status: 500 }));
		await expect.element(testPage.getByText('Downloaded clients', { exact: false })).not.toBeInTheDocument();
	});

	it('posts the applied narrowing, the Name from history state included, never in the address', async () => {
		pageState.url = new URL(`${fixture.url}?owes=true&mine=true&doula=staff-2&dueFrom=2027-03-01&dueTo=2027-03-15&all=true&order=overdue`);
		pageState.state = { clientsName: 'Anne' };
		await setup({ list: { clientCount: 16 } });

		await download(csv('16'));

		const [path, init] = apiFetchWithSession.mock.calls.findLast((call) => String(call[0]).endsWith('/export'))!;
		expect(path).toBe(`/api/practices/${practiceId}/clients/export`);
		expect((init as RequestInit).method).toBe('POST');
		expect(JSON.parse((init as RequestInit).body as string)).toEqual({
			all: true,
			mine: false,
			owes: true,
			noContract: false,
			doula: 'staff-2',
			name: 'Anne',
			due: '',
			dueFrom: '2027-03-01',
			dueTo: '2027-03-15',
			order: 'overdue'
		});
		expect(String(path)).not.toContain('Anne');
	});

	it('describes what it downloads, using the list’s total while more are to come', async () => {
		await setup({ list: { hasMore: true, clientCount: 35 } });
		await expect.element(button()).toHaveAccessibleDescription('A spreadsheet of the 35 Clients this list shows.');
	});

	it('counts the rows on screen when no more are to come', async () => {
		await setup({ list: { items: [clients[0]] } });
		await expect.element(button()).toHaveAccessibleDescription('A spreadsheet of the 1 Client this list shows.');
	});

	it('says no count where more are to come and the total is not known', async () => {
		await setup({ list: { hasMore: true } });
		await expect.element(button()).toHaveAccessibleDescription('A spreadsheet of every Client this list shows.');
	});

	it.each([
		[429, 'Try again in 10 minutes.'],
		[403, 'Your role cannot download Clients.'],
		[500, 'Your Clients did not download. Nothing you saved is lost.']
	])('answers a %i in place, saves nothing and leaves the button for another try', async (status, words) => {
		await setup();

		const { click } = await download(new Response('', { status, headers: { 'Retry-After': '600' } }));

		await expect.element(testPage.getByRole('alert')).toHaveTextContent(words);
		expect(click).not.toHaveBeenCalled();
		await expect.element(button()).not.toHaveAttribute('aria-disabled', 'true');
	});

	it('says an absent connection in place', async () => {
		await setup();
		apiFetchWithSession.mockRejectedValue(new TypeError('offline'));

		await button().click();

		await expect.element(testPage.getByText('Your Clients did not download. Nothing you saved is lost.')).toBeVisible();
	});

	it('is drawn for an Admin and not for a Doula', async () => {
		Object.assign(pageState, toPageState({ ...fixture, pageData: practiceSession(['admin']) }));
		await setup();
		await expect.element(button()).toBeVisible();
		cleanup();

		Object.assign(pageState, toPageState({ ...fixture, pageData: practiceSession(['doula']) }));
		await setup();
		await expect.element(button()).not.toBeInTheDocument();
	});

	it('is not drawn for a contractor Doula', async () => {
		await setup({ isContractor: true });
		await expect.element(testPage.getByRole('heading', { name: 'Clients' })).toBeVisible();
		await expect.element(button()).not.toBeInTheDocument();
	});

	it('is drawn but unavailable while the list is empty, so the title row never moves', async () => {
		await setup({ list: { items: [] } });
		await expect.element(button()).toHaveAttribute('aria-disabled', 'true');
	});
});
