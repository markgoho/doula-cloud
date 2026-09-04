/*
 * The seam every route reads its environment through (#597).
 *
 * Two properties matter here and neither is about the getters themselves.
 * First, that an unoverridden read is the real `$app/state` and nothing
 * else -- 30 routes changed their import on this ticket, and a shim that
 * quietly answered something other than SvelteKit would have moved every
 * one of them. Second, that an override is total and reversible, because
 * the drag surface installs one per subject and a leak would mean one
 * dragged route rendering the previous one's Practice.
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

/*
 * The real module, mocked -- which is the point: `appState` reads
 * `$app/state` rather than replacing it, so a spec that mocks the source
 * still reaches the shim. That is what keeps the 24 route specs that
 * already mock `$app/state` working unchanged.
 */
const realPage = vi.hoisted(() => ({
	params: { practiceId: 'the-real-practice' } as Record<string, string>,
	url: new URL('https://example.test/practices/the-real-practice'),
	data: { practiceName: 'The real Practice' } as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: realPage }));

const { clearPageOverride, overridePage, page } = await import('./appState.svelte.js');

const fixtureEnvironment = {
	params: { practiceId: 'practice-1' },
	url: new URL('https://example.test/practices/practice-1/invoices'),
	data: { practiceName: 'Highland Midwifery Group' }
};

beforeEach(() => {
	clearPageOverride();
});

describe('the page a route reads', () => {
	it('is $app/state itself when nothing has overridden it', () => {
		expect(page.params).toBe(realPage.params);
		expect(page.url).toBe(realPage.url);
		expect(page.data).toBe(realPage.data);
	});

	it('is the fixture environment once one is installed', () => {
		overridePage(fixtureEnvironment);

		expect(page.params.practiceId).toBe('practice-1');
		expect(page.url.pathname).toBe('/practices/practice-1/invoices');
		expect(page.data.practiceName).toBe('Highland Midwifery Group');
	});

	/*
	 * The drag surface's `$effect.pre` cleanup runs this. A route left
	 * installed after its subject is swapped out is how the surface would
	 * come to show one screen's content under another screen's name.
	 */
	it('is $app/state again once the override is cleared', () => {
		overridePage(fixtureEnvironment);
		clearPageOverride();

		expect(page.params).toBe(realPage.params);
		expect(page.url).toBe(realPage.url);
		expect(page.data).toBe(realPage.data);
	});

	/*
	 * Every property together, never a merge. A partial override would let
	 * a dragged route read one screen's params beside another's data, and
	 * the reader would have no way to see which half was which.
	 */
	it('replaces every property at once rather than merging with the real page', () => {
		overridePage({ params: {}, url: new URL('https://example.test/'), data: {} });

		expect(page.params).toEqual({});
		expect(page.data).toEqual({});
		expect(page.url.pathname).toBe('/');
	});
});
