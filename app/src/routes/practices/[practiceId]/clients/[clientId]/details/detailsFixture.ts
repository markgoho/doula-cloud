/*
 * The Client whose details are being added, as the continuum check and
 * the route specs both see her (#1610, ADR-0025).
 *
 * The same reason `clients/new/intakeFixture.ts` gives for a shared seed:
 * the sweep mounts a `+page.svelte` with no layout in front of it, so each
 * fixture fills the module state `details/+layout.svelte` would have
 * filled. The Practice's name and its template are seeded here, into
 * `intakeFlow`: this journey is the one that reads them (#1611), and a
 * Practice's own sections, a long one among them, are in the journey
 * this sweeps.
 *
 * ## Two states, not one row (`.claude/rules/svelte-tests.md`)
 *
 * Her record holds some facts and not others, which is the case this
 * journey exists for: a long email address and an address with no
 * second line are on file, the phone number is not, and two of the
 * Practice's five questions are answered. The draft adds one answer over
 * a blank, so the check page shows a value on file, a value typed, and
 * a question nobody answered.
 */
import type { ClientDetail } from '#lib/clientDetail.js';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { answersOnFile } from '#lib/clientDetailsJourney.js';
import type { Field } from '#lib/clientFieldTemplate.js';
import { intakeFlow } from '#lib/intakeFlow.svelte.js';
import { practiceId } from '../../new/intakeFixture.js';

export { practiceId } from '../../new/intakeFixture.js';

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

export const clientId = 'client-1';

export const record: ClientDetail = {
	id: clientId,
	givenName: 'Anne-Marie',
	familyName: 'Ochieng-Whitfield',
	preferredName: '',
	email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
	phone: '',
	addressLine1: '4827 Pittsford-Mendon Center Road',
	addressLine2: '',
	addressLocality: 'Honeoye Falls',
	addressRegion: 'NY',
	addressPostalCode: '14472',
	dateOfBirth: '1988-02-09',
	fieldValues: {
		referral: 'A sister who was a Client here in 2024',
		attendees: ['Partner', 'Mother', 'Photographer'],
		// A value under a field the Practice has archived: kept by the
		// save, never asked.
		retired: 'Answered before the Practice stopped asking'
	},
	resolvedFields: [],
	engagements: [],
	history: []
};

/** Fills the module state `details/+layout.svelte` fills. Called at
 * import time by every fixture in the journey. */
export function seedDetails(): void {
	intakeFlow.practiceId = practiceId;
	intakeFlow.practiceName = practiceName;
	intakeFlow.fields = fields;
	intakeFlow.status = 'ready';

	clientDetails.clientId = clientId;
	clientDetails.status = 'ready';
	clientDetails.loadError = '';
	clientDetails.record = record;
	clientDetails.engagementId = undefined;
	clientDetails.draft.scope = clientId;
	clientDetails.draft.answers = { ...answersOnFile(record), phone: '+1 (585) 555-0142' };
	clientDetails.draft.visitedSteps = ['date-of-birth', 'email', 'phone'];
}

