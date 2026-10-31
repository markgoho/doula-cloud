/*
 * Accepting a Staff invite, as the continuum check sees it (#595).
 *
 * The screen reads its Invitation on mount (#2238), so the fixture answers
 * that read. The base is a person new here, who is asked the most; the
 * variant is a person who already has an account, asked for a password
 * only. Values are long on purpose (#537): a Practice name and an address
 * at the length a real one can reach.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture, RouteVariant } from '../../routeFixture.js';
import Page from './+page.svelte';

export const invitation = {
	practiceName: 'Finger Lakes Birth & Postpartum Doula Collective of Greater Rochester',
	address: 'christina.vanderhoeven-okonkwo@fingerlakesbirthcollective.example.com',
	roles: ['owner', 'admin', 'doula'],
	employmentType: 'employee',
	hasAccount: false
};

export const returningInvitation = { ...invitation, roles: ['doula'], employmentType: 'contractor', hasAccount: true };

/*
 * #2025: the same screen from a link with no token in it -- one plain
 * sentence and the next step, in place of the form.
 */
export const linkNotComplete: RouteVariant = {
	name: 'Accepting a Staff invite, from a link that is not complete',
	url: 'https://example.test/(signed-out)/accept-invite',
	readyText: 'Accept your invite'
};

export const withAnAccount: RouteVariant = {
	name: 'Accepting a Staff invite, on an account she already has',
	respond: () => jsonResponse(returningInvitation)
};

export const fixture: RouteFixture = {
	name: 'Accepting a Staff invite, as a person new here',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/accept-invite?token=invite-token-1',
	readyText: `Join ${invitation.practiceName}`,
	respond: () => jsonResponse(invitation),
	variants: [linkNotComplete, withAnAccount]
};
