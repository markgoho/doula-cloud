/*
 * A Practice's own questions, in a Client's details journey (#1610).
 *
 * Section 1, for the reason intake's own fixture gives: it is the
 * Practice-named section holding every field type the value renderer
 * draws differently, under a heading a Practice wrote as a sentence, so
 * it is the wider screen. Seeded through `detailsFixture.ts`.
 */
import type { RouteFixture } from '../../../../../../../routeFixture.js';
import { clientId, practiceId, seedDetails } from '../../detailsFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedDetails();

export const fixture: RouteFixture = {
	name: 'A Practice’s own questions, added from the record',
	component: Page,
	params: { practiceId, clientId, sectionIndex: '1' },
	url: 'https://example.test/practices/practice-1/clients/client-1/details/sections/1',
	readyText: 'continuous labor support'
};
