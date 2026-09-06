/*
 * A Practice's own questions, asked (#466).
 *
 * Section 1 rather than section 0: index 0 is the run of un-headed
 * fields that takes the Practice's own name, and index 1 is the
 * Practice-named section holding every field type the value renderer
 * draws differently. The second is the wider screen, so it is the one
 * the sweep measures.
 */
import type { RouteFixture } from '../../../../../../routeFixture.js';
import { practiceId, seedIntake } from '../../intakeFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedIntake();

export const fixture: RouteFixture = {
	name: 'A Practice’s own intake questions',
	component: Page,
	params: { practiceId, sectionIndex: '1' },
	url: 'https://example.test/practices/practice-1/clients/new/sections/1',
	readyText: 'continuous labor support'
};
