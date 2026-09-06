/*
 * The address fieldset -- five inputs, content-sized (#466).
 *
 * Seeded through `intakeFixture.ts` rather than through `respond()`:
 * the sequence's shape is read once by the layout the sweep does not
 * mount, so each step's fixture fills the same module state that layout
 * would have. See that file for why the content is what it is.
 */
import type { RouteFixture } from '../../../../../routeFixture.js';
import { practiceId, seedIntake } from '../intakeFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedIntake();

export const fixture: RouteFixture = {
	name: 'A Client’s address',
	component: Page,
	params: { practiceId },
	url: 'https://example.test/practices/practice-1/clients/new/address',
	readyText: 'address?'
};
