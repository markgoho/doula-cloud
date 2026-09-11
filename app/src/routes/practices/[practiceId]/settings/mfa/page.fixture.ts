/*
 * The MFA settings screen, as the continuum check sees it (#595, #606).
 *
 * An Owner viewing the switch before she has thrown it: `required` is
 * false and a realistic slice of a 14-doula Practice's roster --
 * `withoutSecondFactor` -- has not enrolled a second factor yet, which is
 * the number this screen exists to show her before she can bar them.
 *
 * Two sessions (#928). `isOwner` picks between two whole bodies here, not
 * between a body and a shorter one: an Owner reads the Badge, the roster
 * count and a button, and everybody else reads a single Notice instead.
 * Neither tree is a subset of the other.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import { practiceSession } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const impact = { required: false, withoutSecondFactor: 6 };

/*
 * Everyone who is not an Owner, Admin included. She is not asked for the
 * impact at all -- the route fetches it only when `isOwner` -- so the
 * inherited `respond` is never reached on this branch, and its own
 * longest line is the Notice, which is this repo's copy.
 */
export const nonOwner: RouteVariant = {
	name: 'The MFA settings screen, as a non-Owner',
	pageData: practiceSession(['admin'])
};

export const fixture: RouteFixture = {
	name: 'The MFA settings screen, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/mfa',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(impact),
	readyText: 'Multi-factor authentication',
	variants: [nonOwner]
};
