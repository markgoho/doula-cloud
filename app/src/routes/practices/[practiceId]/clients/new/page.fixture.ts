/*
 * Client intake, as the continuum check sees it (#595).
 *
 * A client-side, three-step wizard with no per-step URL (see the route's
 * own header comment) -- the sweep can only ever mount step one, which
 * asks a single static question. The carried name comes from the search
 * screen's own miss handoff and pre-fills a text input rather than
 * rendering anywhere free-form, so #537's vocabulary is exercised for
 * completeness rather than because this step needs it.
 */
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Client intake',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/clients/new?name=Anne-Marie%20Ochieng-Whitfield',
	readyText: "What is the Client's name?"
};
