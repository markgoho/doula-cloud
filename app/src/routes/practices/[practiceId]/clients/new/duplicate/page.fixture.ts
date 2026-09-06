/*
 * The save-time duplicate check, as a page (#466, ADR-0017).
 *
 * Seeded through `intakeFixture.ts` rather than through `respond()`:
 * the sequence's shape is read once by the layout the sweep does not
 * mount, so each step's fixture fills the same module state that layout
 * would have. The two matches this page offers are seeded there too --
 * see that file for why they are not set here.
 */
import type { RouteFixture } from '../../../../../routeFixture.js';
import { practiceId, seedIntake } from '../intakeFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedIntake();

export const fixture: RouteFixture = {
	name: 'The intake duplicate check',
	component: Page,
	params: { practiceId },
	url: 'https://example.test/practices/practice-1/clients/new/duplicate',
	readyText: 'Is this the same person?'
};
