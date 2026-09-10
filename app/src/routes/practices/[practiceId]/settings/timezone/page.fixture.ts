/*
 * The Timezone screen, as the continuum check sees it (#1166).
 *
 * The zone it opens on is deliberately one the seven-entry US list does
 * not carry. That is the hostile case ADR-0025 asks a fixture to realize
 * here: the select has to show the name the Practice actually holds
 * rather than render blank, and the longest label on the screen is then
 * the one the option list grew for it, which is what the 320px sweep has
 * to survive.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeTimezone } from '#lib/practiceTimezone.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const timezone: PracticeTimezone = { timezone: 'America/Indiana/Indianapolis' };

export const fixture: RouteFixture = {
	name: 'Timezone',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/timezone',
	respond: () => jsonResponse(timezone),
	readyText: 'Timezone'
};
