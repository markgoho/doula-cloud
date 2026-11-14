/*
 * #2888: the Show radios and the Order select apply when changed, and the
 * search waits for Apply. Its own file for the same reason the order's is.
 */
import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './InvoicesScreen.svelte';
import { toPageState } from '../../../routeFixture.js';
// jscpd:ignore-start -- vi.mock is hoisted per file, so each spec of this screen restates the same module mocks
import { data, fixture } from './page.fixture.js';
import { apiFetchWithSession, goto, pageState } from '#testing/routePage.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
vi.mock('#lib/api.js', () => import('#testing/routePage.js'));
vi.mock('$app/navigation', async () => ({
	...(await import('#testing/routePage.js')),
	invalidate: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	replaceState: vi.fn(),
	snapshot: vi.fn()
}));
if (!customElements.get('center-l')) registerLayoutPrimitives();
// jscpd:ignore-end

const path = `/practices/${fixture.params.practiceId}/invoices`;
const settled = () => new Promise((resolve) => setTimeout(resolve, 450));

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	Object.assign(pageState, toPageState(fixture));
});

afterEach(() => {
	vi.useRealTimers();
});

async function setupChange() {
	pageState.url = new URL(fixture.url);
	pageState.state = {};
	await testPage.viewport(1440, 900);
	return render(Page, { data });
}

const radio = (name: string) => testPage.getByRole('radio', { name });

describe('the Invoices list applies untyped controls when changed (#2888)', () => {
	it('reads once for a Show radio, keeps focus on it and adds an entry', async () => {
		await setupChange();

		await radio('Overdue').click();
		expect(goto).not.toHaveBeenCalled();
		await settled();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${path}?show=overdue`, {
			state: { invoicesSearch: '' },
			replaceState: false,
			reset: false
		});
		expect(document.activeElement).toBe(radio('Overdue').element());
	});

	it('sends the typed search with the change, and the order too', async () => {
		await setupChange();

		await testPage.getByRole('searchbox', { name: 'Reference or Client' }).fill('Anne');
		await testPage.getByRole('combobox', { name: 'Order' }).selectOptions('Largest amount first');
		await settled();

		expect(goto).toHaveBeenCalledWith(`${path}?order=amount-largest`, expect.objectContaining({ state: { invoicesSearch: 'Anne' } }));
	});

	it('does not read for the search until Apply', async () => {
		await setupChange();

		await testPage.getByRole('searchbox', { name: 'Reference or Client' }).fill('Anne');
		await settled();

		expect(goto).not.toHaveBeenCalled();
	});

	it('sends one read for two quick changes, and Apply stays a way to send', async () => {
		await setupChange();
		vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });

		await radio('Unpaid').click();
		await radio('Overdue').click();
		await vi.advanceTimersByTimeAsync(450);
		expect(goto).toHaveBeenCalledOnce();

		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenLastCalledWith(`${path}?show=overdue`, { state: { invoicesSearch: '' } });
	});
});
