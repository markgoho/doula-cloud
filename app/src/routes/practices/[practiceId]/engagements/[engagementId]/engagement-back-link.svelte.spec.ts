import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import '#lib/styles/app.css';
import { toPageState } from '../../../../routeFixture.js';
import { detail as fixtureDetail, fixture, session } from './page.fixture.js';
import { holdListSearch, ListBack } from '#lib/listOrigin.svelte.js';
import { pageState } from '#testing/routePage.js';
import Page from './EngagementScreen.svelte';
if (!customElements.get('center-l')) registerLayoutPrimitives();

vi.mock('$app/state', () => import('#testing/routePage.js'));
Object.assign(pageState, toPageState(fixture));

const navigation = vi.hoisted(() => ({
	goto: vi.fn(),
	afterNavigate: undefined as ((navigation: { from: { url: URL } | undefined }) => void) | undefined
}));
vi.mock('$app/navigation', () => ({
	goto: navigation.goto,
	afterNavigate: (callback: (navigation: { from: { url: URL } | undefined }) => void) => {
		navigation.afterNavigate = callback;
	}
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));
vi.mock('#lib/pushRefresh.js', () => ({ subscribeToThreadPushMessages: vi.fn(() => vi.fn()) }));

const practice = '/practices/practice-1';
const arrivedFrom = (path: string) => ({ from: { url: new URL(`https://example.test${practice}${path}`) } });

async function setup() {
	await testPage.viewport(1280, 900);
	apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
	const back = new ListBack(
		() => ({ schedule: `${practice}/schedule`, invoices: `${practice}/invoices` }),
		() => pageState.url.pathname
	);
	await render(Page, { data: { ...fixtureDetail, session }, back });
}

const back = (name: string) => testPage.getByRole('link', { name, exact: true });

describe('the Engagement page back link (#2864)', () => {
	let historyBack: ReturnType<typeof vi.spyOn>;

	beforeEach(() => {
		apiFetchWithSession.mockReset();
		navigation.goto.mockReset();
		history.replaceState(undefined, '', location.pathname + location.search);
		historyBack = vi.spyOn(history, 'back').mockImplementation(() => {});
	});

	afterEach(() => {
		historyBack.mockRestore();
	});

	it('names the Clients list on a cold load', async () => {
		await setup();
		navigation.afterNavigate?.({ from: undefined });

		await expect.element(back('Clients')).toHaveAttribute('href', `${practice}/clients`);
	});

	it('names the Schedule when opened from it, keeping the filters in its address', async () => {
		await setup();
		navigation.afterNavigate?.(arrivedFrom('/schedule?window=next30&mine=true'));

		await expect
			.element(back('Back to the Schedule'))
			.toHaveAttribute('href', `${practice}/schedule?window=next30&mine=true`);
	});

	it('names Invoices when opened from the Invoices list', async () => {
		await setup();
		navigation.afterNavigate?.(arrivedFrom('/invoices?view=unpaid'));

		await expect.element(back('Back to Invoices')).toHaveAttribute('href', `${practice}/invoices?view=unpaid`);
	});

	it('names the Clients list when opened from anywhere else', async () => {
		await setup();
		navigation.afterNavigate?.(arrivedFrom('/schedule?window=next30'));
		navigation.afterNavigate?.(arrivedFrom('/clients/client-1'));

		await expect.element(back('Clients')).toHaveAttribute('href', `${practice}/clients`);
	});

	it('goes back through history when it was opened from the list', async () => {
		await setup();
		navigation.afterNavigate?.(arrivedFrom('/schedule?window=next30'));

		await back('Back to the Schedule').click();

		expect(historyBack).toHaveBeenCalledOnce();
		expect(navigation.goto).not.toHaveBeenCalled();
	});

	it('goes to the list with its held search once the page has moved since', async () => {
		await setup();
		holdListSearch('schedule', 'Quinn');
		navigation.afterNavigate?.(arrivedFrom('/schedule?window=next30'));
		history.replaceState(undefined, '', '#messages');

		await back('Back to the Schedule').click();

		expect(historyBack).not.toHaveBeenCalled();
		expect(navigation.goto).toHaveBeenCalledWith(`${practice}/schedule?window=next30`, {
			state: { scheduleSearch: 'Quinn' }
		});
		expect(String(navigation.goto.mock.calls[0][0])).not.toContain('Quinn');
	});

	it('hands the Invoices list its search back the same way', async () => {
		await setup();
		holdListSearch('invoices', 'INV-0019');
		navigation.afterNavigate?.(arrivedFrom('/invoices'));
		history.replaceState(undefined, '', '#messages');

		await back('Back to Invoices').click();

		expect(navigation.goto).toHaveBeenCalledWith(`${practice}/invoices`, { state: { invoicesSearch: 'INV-0019' } });
	});
});
