/*
 * #2888: a box, a radio and a select apply when changed, and the typed fields
 * wait for Apply. Its own file because the clock here is real after the first
 * await (the bar settles for 250ms).
 */
// jscpd:ignore-start -- the route-spec imports and mock preamble every list's spec repeats
import { page as testPage, userEvent } from 'vitest/browser';
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

async function setupAt(query = '') {
	pageState.url = new SvelteURL(`https://example.test${schedulePath}${query}`);
	pageState.state = {};
	await testPage.viewport(1440, 900);
	const parameters = new URLSearchParams(query);
	const [from, to] = [parameters.get('from'), parameters.get('to')];
	const chosen = from && to ? { from, to } : undefined;
	const view = { ...data.view, preset: 30 as const, chosen };
	return render(Page, { data: { ...data, view, filters: { ...data.filters, ...chosen } } });
}

const box = (name: string) => testPage.getByRole('checkbox', { name });
const keep = { reset: false };

describe('the Schedule applies untyped controls when changed (#2888)', () => {
	it('reads once for a ticked box, keeps focus on it and adds a history entry', async () => {
		await setupAt();

		await box('Mine').click();
		expect(goto).not.toHaveBeenCalled();
		await settled();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${schedulePath}?mine=true`, {
			state: { scheduleSearch: '' },
			replaceState: false,
			...keep
		});
		expect(document.activeElement).toBe(box('Mine').element());
	});

	it('applies the days radios, the Doula and the order the same way', async () => {
		await setupAt();

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await settled();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7`, expect.objectContaining(keep));

		await testPage.getByLabelText('Order').selectOptions('Latest first');
		await settled();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7&order=latest`, expect.objectContaining(keep));
	});

	it('sends the whole bar as drawn, a typed Client included', async () => {
		await setupAt();

		await testPage.getByLabelText('Client').fill('Cam');
		await box('Mine').click();
		await settled();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?mine=true`, expect.objectContaining({ state: { scheduleSearch: 'Cam' } }));
	});

	it('does not read for a typed Client until Apply', async () => {
		await setupAt();

		await testPage.getByLabelText('Client').fill('Cam');
		await userEvent.keyboard('{Tab}');
		await settled();

		expect(goto).not.toHaveBeenCalled();
	});

	it('sends one read for arrow presses inside the settle, and the draft is the last pick', async () => {
		await setupAt();
		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await settled();
		goto.mockClear();

		await userEvent.keyboard('{ArrowDown}{ArrowUp}');
		await settled();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${schedulePath}?days=7`, expect.objectContaining(keep));
		await expect.element(testPage.getByRole('radio', { name: 'Next 7 days' })).toBeChecked();
	});

	it('applies a picked Doula', async () => {
		await setupAt();
		const doula = testPage.getByRole('combobox', { name: 'Doula' });
		await doula.selectOptions(doula.element().querySelectorAll('option')[1]!.textContent!);
		await settled();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(expect.stringContaining('doula='), expect.objectContaining(keep));
	});

	it('replaces the entry for a second change on the same control', async () => {
		await setupAt();

		await box('Mine').click();
		await settled();
		await box('Mine').click();
		await settled();

		expect(goto.mock.calls.map((call) => (call[1] as { replaceState: boolean }).replaceState)).toEqual([false, true]);
	});

	it('only opens From and To for "Choose days", filled with the days on screen, and sends nothing', async () => {
		await setupAt();

		await testPage.getByRole('radio', { name: 'Choose days' }).click();
		await settled();

		expect(goto).not.toHaveBeenCalled();
		await expect.element(testPage.getByLabelText('From')).toBeVisible();
	});

	it('applies "Next 7 days" while From and To are open, and sends the open range with another change', async () => {
		await setupAt('?from=2026-09-10&to=2026-09-19');

		await box('Mine').click();
		await settled();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?from=2026-09-10&to=2026-09-19&mine=true`, expect.objectContaining(keep));

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await settled();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7&mine=true`, expect.objectContaining(keep));
	});

	it('sends nothing for a range that cannot match, says so beside To and keeps focus on the box', async () => {
		await setupAt('?from=2026-09-10&to=2026-09-19');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-09-01');

		await box('Mine').click();
		await settled();

		expect(goto).not.toHaveBeenCalled();
		await expect.element(testPage.getByText('Choose a To day that is the same as the From day or later.')).toBeVisible();
		expect(document.activeElement).toBe(box('Mine').element());
	});

	it('still lets Apply send the bar', async () => {
		await setupAt();
		vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
		vi.setSystemTime(new Date('2026-09-07T13:00:00Z'));

		await box('Mine').click();
		await testPage.getByRole('button', { name: 'Apply' }).click();
		await vi.advanceTimersByTimeAsync(450);

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${schedulePath}?mine=true`, { state: { scheduleSearch: '' } });
	});
});

describe('the Schedule for a Doula', () => {
	it('applies Mine when changed too', async () => {
		pageState.data = { session: { ...practiceSession(['doula']).session, hasOtherDoulas: true } };
		await setupAt();

		await box('Mine').click();
		await settled();

		expect(goto).toHaveBeenCalledOnce();
	});
});
