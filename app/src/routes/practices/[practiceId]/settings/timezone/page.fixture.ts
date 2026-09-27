/*
 * The Timezone screen, as the continuum check sees it (#1166).
 *
 * The zone it opens on is deliberately one the seven-entry US list does
 * not carry. That is the hostile case ADR-0025 asks a fixture to realize
 * here: the select has to show the name the Practice actually holds
 * rather than render blank, and the longest label on the screen is then
 * the one the option list grew for it, which is what the 320px sweep has
 * to survive.
 *
 * The screen reads `isOwnerOrAdmin` and renders two trees from it
 * (#1441), so it declares both (#913): the Owner's editable field and
 * Save button, and the read-only zone anyone else meets. Every Staff can
 * read the zone -- `InvoiceSection`'s future-date ceiling needs it (#1280)
 * -- but only an Owner or Admin can state it. The Admin's tree is the
 * Owner's exactly, so it is left out on purpose.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeTimezone } from '#lib/practiceTimezone.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const timezone: PracticeTimezone = { timezone: 'America/Indiana/Indianapolis' };

/*
 * Anyone else's branch. It answers the same one read, so `respond` is
 * inherited; the hostile zone above is the value it shows as text, which
 * is the longest line this branch has.
 */
export const asDoula: RouteVariant = {
	name: 'Timezone, as a Doula',
	pageData: practiceSession(['doula'])
};

export const fixture: RouteFixture = {
	name: 'Timezone',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/timezone',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(timezone),
	readyText: 'Timezone',
	variants: [asDoula]
};
