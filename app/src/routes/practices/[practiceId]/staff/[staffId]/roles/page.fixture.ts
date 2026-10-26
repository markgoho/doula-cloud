/*
 * The roles question, as the continuum check sees it (#2041).
 *
 * The Owner's own row, because it holds three roles: every box ticked,
 * each with its hint under it, under a question that embeds the
 * hyphenated double-barreled name (#537).
 *
 * An Admin's page is another tree, not a subset -- the question gives way
 * to a notice saying who can change roles, and nothing is fetched -- so
 * it is declared.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture, RouteVariant } from '../../../../../routeFixture.js';
import { fixture as rosterFixture, roster } from '../../page.fixture.js';
import Page from './+page.svelte';

const { session } = rosterFixture.pageData as { session: Record<string, unknown> };
const [owner] = roster.members;

export const asAdmin: RouteVariant = {
	name: 'The roles question, as an Admin who cannot change roles',
	pageData: { session: { ...session, roles: ['admin'] } },
	readyText: 'Change roles'
};

export const fixture: RouteFixture = {
	name: 'The roles question',
	component: Page,
	params: { practiceId: 'practice-1', staffId: owner!.staffId },
	url: `https://example.test/practices/practice-1/staff/${owner!.staffId}/roles`,
	pageData: rosterFixture.pageData,
	variants: [asAdmin],
	respond: () => jsonResponse(roster),
	readyText: `Which roles does ${owner!.name} have?`
};
