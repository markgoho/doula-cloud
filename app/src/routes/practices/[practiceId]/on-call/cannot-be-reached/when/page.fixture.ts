import type { RouteFixture } from '../../../../../routeFixture.js';
import { ownerSession, practiceId, seedDraft } from '../journeyFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedDraft();

export const fixture: RouteFixture = {
	name: 'Recording a time a Doula cannot be reached: when',
	component: Page,
	params: { practiceId },
	url: 'https://example.test/practices/practice-1/on-call/cannot-be-reached/when',
	pageData: ownerSession,
	readyText: 'When can '
};
