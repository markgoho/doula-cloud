/*
 * The employment type question, as the continuum check sees it (#2041).
 *
 * The question embeds the hyphenated double-barreled name (#537), and both
 * answers carry a hint. Which one is chosen changes no width, so one
 * Member is enough.
 *
 * An Admin's page is another tree, not a subset -- the question gives way
 * to a notice saying who can change it, and nothing is fetched -- so it
 * is declared.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture, RouteVariant } from '../../../../../routeFixture.js';
import { fixture as rosterFixture, roster } from '../../page.fixture.js';
import Page from './+page.svelte';

const { session } = rosterFixture.pageData as { session: Record<string, unknown> };
const [owner] = roster.members;

export const asAdmin: RouteVariant = {
	name: 'The employment type question, as an Admin who cannot change it',
	pageData: { session: { ...session, roles: ['admin'] } },
	readyText: 'Change employment type'
};

export const fixture: RouteFixture = {
	name: 'The employment type question',
	component: Page,
	params: { practiceId: 'practice-1', staffId: owner!.staffId },
	url: `https://example.test/practices/practice-1/staff/${owner!.staffId}/employment-type`,
	pageData: rosterFixture.pageData,
	variants: [asAdmin],
	respond: () => jsonResponse(roster),
	readyText: `Is ${owner!.name} an employee or a contractor?`
};
