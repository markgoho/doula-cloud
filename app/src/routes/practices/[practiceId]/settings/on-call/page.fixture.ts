/*
 * The On-call rule screen, as the continuum check sees it (#1093).
 *
 * It opens on the gestational-week rule, which is the branch that
 * renders the most: the week field only exists under that rule, so the
 * other branch is a strict subset of this one and realizes nothing the
 * sweep has not already measured. The hint lines and the last-changed
 * line are the widest content on the screen, and they are the sentences
 * the product actually says; the last change is a Doula-length name, the
 * hostile case for the line under the title.
 *
 * The screen reads `isOwnerOrAdmin` and renders two trees from it, so it
 * declares both (#913): the Owner's form and the reader's notice. The
 * Admin's tree is the Owner's exactly, so it is left out on purpose.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { OnCallSettingsRead } from '#lib/onCallRule.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const settings: OnCallSettingsRead = {
	startRule: 'gestational_week',
	startWeek: 37,
	graceDays: 14,
	lastChanged: { staffName: 'Dee Whitlock', at: '2026-11-12T15:02:11Z' }
};

export const asDoula: RouteVariant = {
	name: 'On-call rule, as a Doula',
	pageData: practiceSession(['doula'])
};

export const fixture: RouteFixture = {
	name: 'The on-call rule',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/on-call',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(settings),
	readyText: 'On-call rule',
	variants: [asDoula]
};
