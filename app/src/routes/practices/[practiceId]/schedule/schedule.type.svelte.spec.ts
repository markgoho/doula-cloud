/*
 * #2888: the Schedule narrows to the Visit's type. Its own file because the
 * clock here is real after the first await (the bar settles for 250ms).
 */
// jscpd:ignore-start -- the route-spec imports and mock preamble every list's spec repeats
import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteURL } from 'svelte/reactivity';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './ScheduleScreen.svelte';
import { practiceSession, toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';
import { apiFetchWithSession, goto, pageState } from '#testing/routePage.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
vi.mock('#lib/api.js', () => import('#testing/routePage.js'));
vi.mock('$app/navigation', async () => ({
	...(await import('#testing/routePage.js')),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	snapshot: vi.fn()
}));
// jscpd:ignore-end
if (!customElements.get('center-l')) registerLayoutPrimitives();

const { practiceId } = fixture.params;
const schedulePath = `/practices/${practiceId}/schedule`;
const settled = () => new Promise((resolve) => setTimeout(resolve, 450));

// jscpd:ignore-start -- the per-test clock and mock reset every list's spec repeats
beforeEach(() => {
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2026-09-07T13:00:00Z'));
	apiFetchWithSession.mockReset();
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	Object.assign(pageState, toPageState(fixture));
});

afterEach(() => {
	vi.useRealTimers();
});
// jscpd:ignore-end

async function setupAt(query = '', read: Partial<typeof data> = {}) {
	pageState.url = new SvelteURL(`https://example.test${schedulePath}${query}`);
	pageState.state = {};
	await testPage.viewport(1440, 900);
	return render(Page, { data: { ...data, ...read } });
}

const typeSelect = () => testPage.getByRole('combobox', { name: 'Type' });

describe('the Type select (#2888)', () => {
	it('is offered to an Owner after the Doula, with every type and "Any type"', async () => {
		await setupAt();

		await expect.element(typeSelect()).toBeVisible();
		await expect.element(typeSelect()).toHaveAttribute('name', 'type');
		await expect.element(typeSelect()).toHaveValue('');
		const options = [...typeSelect().element().querySelectorAll('option')].map((option) => [option.value, option.textContent?.trim()]);
		expect(options).toEqual([
			['', 'Any type'],
			['prenatal', 'Prenatal'],
			['birth', 'Birth'],
			['postpartum', 'Postpartum']
		]);
		const doula = testPage.getByRole('combobox', { name: 'Doula' }).element();
		expect(doula.compareDocumentPosition(typeSelect().element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('is offered to a Doula, who has no Doula select', async () => {
		pageState.data = { session: practiceSession(['doula']).session };
		await setupAt('', { doulas: [] });

		await expect.element(typeSelect()).toBeVisible();
		expect(testPage.getByRole('combobox', { name: 'Doula' }).elements()).toHaveLength(0);
	});

	it('is offered to a contractor, who has nothing else to fold with Order', async () => {
		pageState.data = { session: { ...practiceSession(['doula']).session, isContractor: true } };
		await setupAt('', { doulas: [] });

		await expect.element(typeSelect()).toBeVisible();
		expect(testPage.getByRole('checkbox', { name: 'Mine' }).elements()).toHaveLength(0);
	});

	it('applies when changed, with no Apply', async () => {
		await setupAt();

		await typeSelect().selectOptions('Postpartum');
		expect(goto).not.toHaveBeenCalled();
		await settled();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${schedulePath}?type=postpartum`, {
			state: { scheduleSearch: '' },
			replaceState: false,
			reset: false
		});
	});

	it('is read from the address, drawn as a tag, and said in the result line', async () => {
		await setupAt('?type=postpartum', {
			view: { ...data.view, type: 'postpartum' },
			page: { items: [data.page.items[0]], hasMore: false, visitCount: 1 }
		});

		await expect.element(typeSelect()).toHaveValue('postpartum');
		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Type Postpartum' }))
			.toHaveAttribute('href', schedulePath);
		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing 1 postpartum Visit from Mon\sSep\s7 to Tue\sOct\s6, soonest\sfirst\.$/);
	});

	it('keeps the type in another tag’s ×', async () => {
		await setupAt('?days=7&type=birth', { view: { ...data.view, preset: 7, type: 'birth' } });

		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Next 7 days' }))
			.toHaveAttribute('href', `${schedulePath}?type=birth`);
	});

	it('says what was looked for when none matched, and offers every type of Visit', async () => {
		await setupAt('?type=birth', {
			view: { ...data.view, type: 'birth' },
			page: { items: [], hasMore: false, visitCount: 0 }
		});

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^No birth Visits from Mon\sSep\s7 to Tue\sOct\s6\.$/);
		await expect
			.element(testPage.getByRole('link', { name: 'Show every type of Visit' }))
			.toHaveAttribute('href', schedulePath);
	});
});
