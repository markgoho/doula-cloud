/*
 * The Practice intake is being walked for, as the continuum check and
 * the route specs both see it (#570, #596, ADR-0025).
 *
 * ## Why a shared seed rather than a `respond()`
 *
 * Intake's draft is module state that `clients/new/+layout.svelte`
 * opens, and the sweep mounts a `+page.svelte` on its own, without that
 * layout, so each fixture seeds the state the layout would have filled.
 *
 * ## Hostile, never polite (#537)
 *
 * Every value here is the longest, busiest one a Practice could
 * plausibly produce: a hyphenated double-barreled name, an email
 * address that runs past the column, two matches with one name. The
 * fixture's job is to find the width at which the screen breaks, and a
 * representative value never will.
 *
 * ## Two states, not one row (`.claude/rules/svelte-tests.md`)
 *
 * The draft carries one Client with every key the search can carry, so
 * the name question's list of carried values is in the screen the sweep
 * measures, its long email address among them. A draft that carried
 * none draws a strict subset of that tree -- no list -- so it is not a
 * second state to seed.
 */
import type { ClientMatch } from '#lib/client.js';
import { blankAnswers, intakeDraft } from '#lib/intakeDraft.svelte.js';

export const practiceId = 'practice-1';

/*
 * The two Clients the save-time duplicate check offers. Two, not one,
 * and both named the same: that page's whole job is telling two people
 * apart, so a single match measures the one screen it does not exist
 * for. Seeded for every step rather than only for `duplicate/`, because
 * the fixtures are imported once each and in glob order -- a seed that
 * one file set and the next cleared would depend on which of them the
 * sweep read last.
 */
const matches: ClientMatch[] = [
	{
		id: 'client-1',
		givenName: 'Anne-Marie',
		familyName: 'Ochieng-Whitfield',
		preferredName: '',
		email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
		phone: '+1 (585) 555-0142',
		addressLine1: '4827 Pittsford-Mendon Center Road',
		addressLine2: '',
		addressLocality: 'Honeoye Falls',
		addressRegion: 'NY',
		addressPostalCode: '14472',
		dateOfBirth: '1988-02-09',
		engagements: [
			{ engagementId: 'e1', kind: 'birth', status: 'completed', createdAt: '2024-03-01T00:00:00Z' },
			{ engagementId: 'e2', kind: 'postpartum', status: 'active', createdAt: '2026-01-04T00:00:00Z' }
		]
	},
	{
		id: 'client-2',
		givenName: 'Anne-Marie',
		familyName: 'Ochieng-Whitfield',
		preferredName: '',
		email: '',
		phone: '',
		addressLine1: '',
		addressLine2: '',
		addressLocality: '',
		addressRegion: '',
		addressPostalCode: '',
		dateOfBirth: '',
		engagements: []
	}
];

/**
 * Fills the module state `clients/new/+layout.svelte` fills, so a
 * `+page.svelte` mounted on its own renders the screen a reader would
 * actually meet. Called at import time by every fixture in the sequence
 * -- they all seed the same Practice, so the order they are imported in
 * does not matter.
 */
export function seedIntake(): void {
	// The name, and every key the search can carry (#1611): intake asks
	// nothing else, so an address or a Practice's own answer is never in
	// its draft.
	intakeDraft.scope = practiceId;
	intakeDraft.answers = {
		...blankAnswers(),
		givenName: 'Anne-Marie',
		familyName: 'Ochieng-Whitfield',
		email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
		phone: '+1 (585) 555-0142',
		dateOfBirth: '1988-02-09'
	};
	intakeDraft.visitedSteps = [];
	intakeDraft.matches = matches;
	// Reached from the search, which is the way in at a Practice that
	// holds Clients -- as this one does (#1609).
	intakeDraft.origin = undefined;
}
