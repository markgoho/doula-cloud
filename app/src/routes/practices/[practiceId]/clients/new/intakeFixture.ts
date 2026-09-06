/*
 * The Practice intake is being walked for, as the continuum check and
 * the route specs both see it (#570, #596, ADR-0025).
 *
 * ## Why a shared seed rather than eight `respond()`s
 *
 * The sequence's shape -- the Practice's name and its Client Field
 * Template -- is read once by `clients/new/+layout.svelte` and held in
 * `intakeFlow`, precisely so a reader four questions in does not meet a
 * loading state on every navigation. The sweep mounts a `+page.svelte`
 * on its own, without that layout, so each fixture seeds the same module
 * state the layout would have filled. The draft is seeded the same way
 * and for the same reason: a question page shows what was typed on the
 * pages before it, and an empty draft measures a screen no doula ever
 * sees.
 *
 * ## Hostile, never polite (#537)
 *
 * Every value here is the longest, busiest one a Practice could
 * plausibly produce: a hyphenated double-barrelled name, a section a
 * Practice named in a sentence, a multi-select with five options. The
 * fixture's job is to find the width at which the screen breaks, and a
 * representative value never will.
 */
import type { ClientMatch } from '#lib/client.js';
import type { Field } from '#lib/clientFieldTemplate.js';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import { intakeFlow } from '#lib/intakeFlow.svelte.js';

export const practiceId = 'practice-1';

export const practiceName = 'Finger Lakes Midwifery & Doula Collective';

/*
 * Two shapes in one template, which is `.claude/rules/svelte-tests.md`'s
 * row rule: a run of un-headed fields (which becomes a section named
 * after the Practice itself), and a Practice-named section holding every
 * field type the value renderer draws differently. The archived field is
 * here to be absent from the form.
 */
export const fields: Field[] = [
	{
		id: 'referral',
		type: 'short_text',
		label: 'Who told this Client about the Practice?',
		order: 0,
		archived: false
	},
	{
		id: 'section-care',
		type: 'section_header',
		label: 'What this Client wants from continuous labor support',
		order: 1,
		archived: false
	},
	{
		id: 'hopes',
		type: 'long_text',
		label: 'In this Client’s own words, what would make this birth feel like a good one?',
		order: 2,
		archived: false
	},
	{
		id: 'birthplace',
		type: 'single_select',
		label: 'Planned place of birth',
		options: ['Home', 'Birth center', 'Strong Memorial Hospital', 'Rochester General Hospital'],
		order: 3,
		archived: false
	},
	{
		id: 'attendees',
		type: 'multi_select',
		label: 'Who else is expected to be in the room',
		options: ['Partner', 'Mother', 'Mother-in-law', 'Sibling', 'Photographer'],
		order: 4,
		archived: false
	},
	{
		id: 'photos',
		type: 'checkbox',
		label: 'Consents to photographs being taken during labor and after the birth',
		order: 5,
		archived: false
	},
	{
		id: 'retired',
		type: 'short_text',
		label: 'A question this Practice stopped asking',
		order: 6,
		archived: true
	}
];

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
	intakeFlow.practiceId = practiceId;
	intakeFlow.practiceName = practiceName;
	intakeFlow.fields = fields;
	intakeFlow.status = 'ready';

	intakeDraft.practiceId = practiceId;
	intakeDraft.answers = {
		givenName: 'Anne-Marie',
		familyName: 'Ochieng-Whitfield',
		preferredName: 'Anne-Marie',
		email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
		phone: '+1 (585) 555-0142',
		addressLine1: '4827 Pittsford-Mendon Center Road',
		addressLine2: 'Apartment 12B, rear entrance off the alley',
		addressLocality: 'Honeoye Falls',
		addressRegion: 'NY',
		addressPostalCode: '14472',
		dateOfBirth: '1988-02-09',
		fieldValues: {
			referral: 'A sister who was a Client here in 2024',
			hopes: 'To be able to move around freely and to have nobody ask the same question twice.',
			birthplace: 'Strong Memorial Hospital',
			attendees: ['Partner', 'Mother', 'Photographer'],
			photos: true
		}
	};
	intakeDraft.visitedSteps = ['name', 'date-of-birth', 'email', 'phone', 'address'];
	intakeDraft.matches = matches;
}
