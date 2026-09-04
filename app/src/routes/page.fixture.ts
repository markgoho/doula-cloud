/*
 * The root landing screen, as the continuum check sees it (#595).
 *
 * `+page.ts`'s own `load` decides which of three shapes `data` takes;
 * the fixture skips straight to the one with a Practice's own free text
 * in it -- the staff picker, where each membership names a Practice.
 * `practiceName` carries #530's own URL rather than this ticket's own
 * invention, since a Practice's registered name is exactly the value
 * that broke a grid track there.
 */
import type { RootLanding } from './+page.js';
import type { RouteFixture } from './routeFixture.js';
import Page from './+page.svelte';

// Narrowed to the one shape this fixture describes, rather than the full
// `RootLanding` union -- the spec spreads this object to build its own
// "several Practices" variant, and a union type would let that spread
// merge in a shape (`portal-picker`) that has no `memberships` at all.
export const data: Extract<RootLanding, { type: 'staff-picker' }> = {
	type: 'staff-picker',
	memberships: [
		{
			practiceId: 'practice-1',
			practiceName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			roles: ['owner']
		}
	]
};

export const fixture: RouteFixture = {
	name: 'The root landing screen (staff picker)',
	component: Page,
	params: {},
	url: 'https://example.test/',
	props: { data },
	readyText: 'Choose a Practice'
};
