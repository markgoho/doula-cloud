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
import { NO_CARE_HEADING } from '#lib/clientRegister.js';
import type { RootLanding } from './+page.js';
import type { RouteFixture, RouteVariant } from '../routeFixture.js';
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

/*
 * #1645: the signed-out state is a different Template from the other
 * three (`LandingPage`, two panels and three doors), so it is its own
 * screen to sweep. Every word on it is fixed product copy and none is a
 * Practice's own, so there is no hostile value to choose; the greeting is
 * whichever the clock gives when the sweep runs, and the four differ in
 * length by less than the space the display step already wraps in.
 */
export const signedOut = {
	name: 'The root landing screen (signed out)',
	props: { data: { type: 'signed-out' } satisfies RootLanding },
	readyText: 'Sign in or set up a Practice'
} as const satisfies RouteVariant;

export const fixture: RouteFixture = {
	name: 'The root landing screen (staff picker)',
	component: Page,
	params: {},
	url: 'https://example.test/',
	props: { data },
	readyText: 'Choose a Practice',
	// #1116: the fourth shape `data` takes is not a picker with nothing in
	// it but a state of its own -- its own heading, two paragraphs and a
	// button, none of which the staff picker above renders. A tree the
	// continuum check never mounts is a tree measured at no width, so it
	// is declared here rather than left to the unit spec alone.
	variants: [
		{
			name: 'The root landing screen (a Portal Account with no care set up)',
			props: { data: { type: 'portal-picker', engagements: [] } satisfies RootLanding },
			readyText: NO_CARE_HEADING
		},
		signedOut
	]
};
