/*
 * The screen that asks an Owner to agree to a material change (#1549),
 * as the continuum check sees it. Fetches nothing on arrival: what is
 * due rides on the layout's session read.
 *
 * Its longest Practice-typed value is the Practice name, inside the
 * declaration sentence; its longest line of our own is a version's
 * Change. Both changed is the variant: two lines of what changed and the
 * longer heading.
 */
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../routeFixture.js';
import type { DueVersion } from '../+layout.js';
import Page from './+page.svelte';

export const termsDue: DueVersion = {
	document: 'terms',
	name: 'Terms of Service',
	version: '2026-11-01',
	change: 'Raises the price of a Credit from $5 to $6, for every Credit bought from that day.'
};

export const privacyDue: DueVersion = {
	document: 'privacy',
	name: 'Privacy Policy',
	version: '2026-12-01',
	change: "Keeps a Visit's notes for seven years after the Engagement ends."
};

function ownerWith(due: DueVersion[], hasSecondFactor?: boolean) {
	const { session } = practiceSession(['owner'], {
		practiceName: 'The Finger Lakes Birth and Postpartum Support Collective of Rochester',
		secondFactor: hasSecondFactor
	});
	return { session: { ...session, agreementDue: due } };
}

export const bothDue: RouteVariant = {
	name: 'The agreement screen, with both documents changed',
	pageData: ownerWith([termsDue, privacyDue]),
	readyText: 'The Terms of Service and the Privacy Policy have changed'
};

export const privacyOnly: RouteVariant = {
	name: 'The agreement screen, with the Privacy Policy changed',
	pageData: ownerWith([privacyDue]),
	readyText: 'The Privacy Policy has changed'
};

export const withoutSecondFactor: RouteVariant = {
	name: 'The agreement screen, as an Owner with no second factor',
	pageData: ownerWith([termsDue], false)
};

export const fixture: RouteFixture = {
	name: 'The agreement screen',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/agreement',
	pageData: ownerWith([termsDue]),
	readyText: 'The Terms of Service have changed',
	variants: [bothDue, privacyOnly, withoutSecondFactor]
};
