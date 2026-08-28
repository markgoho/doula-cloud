// PROTOTYPE (#372) -- throwaway. Stubs for the intake screen: the twelve
// structural columns of ADR-0017, one Practice's field template, and the
// existing Clients the save-time match query from #371 runs against.

export type Kind = 'birth' | 'postpartum';

/**
 * ADR-0017's twelve structural columns, minus practice_id (implicit here).
 */
export interface ClientDraft {
	given_name: string;
	family_name: string;
	preferred_name: string;
	date_of_birth: string;
	email: string;
	phone: string;
	address_line1: string;
	address_line2: string;
	address_locality: string;
	address_region: string;
	address_postal_code: string;
}

export const emptyClient = (): ClientDraft => ({
	given_name: '',
	family_name: '',
	preferred_name: '',
	date_of_birth: '',
	email: '',
	phone: '',
	address_line1: '',
	address_line2: '',
	address_locality: '',
	address_region: '',
	address_postal_code: ''
});

/**
 * The ask, not the Engagement -- no Engagement exists until approval (#393).
 */
export interface RequestDraft {
	kind: Kind | '';
	due_date: string;
	note: string;
}

export const emptyRequest = (): RequestDraft => ({ kind: '', due_date: '', note: '' });

/**
 * ADR-0001's six field types, minus the ones this Practice did not use.
 */
export type FieldType = 'short_text' | 'long_text' | 'single_select' | 'checkbox' | 'section';

export interface PracticeField {
	id: string;
	label: string;
	type: FieldType;
	options?: string[];
}

/**
 * Empty by default at a new Practice; this one has been set up (ADR-0017).
 */
export const practiceFields: PracticeField[] = [
	{ id: 'f_pronouns', label: 'Pronouns', type: 'short_text' },
	{
		id: 'f_referral',
		label: 'How she found us',
		type: 'single_select',
		options: ['Word of mouth', 'Hospital referral', 'Instagram', 'Returning client', 'Other']
	},
	{ id: 'f_emergency', label: 'Emergency contact', type: 'short_text' },
	{ id: 'f_note', label: 'Intake note', type: 'long_text' },
	{ id: 'f_agreed', label: 'Agreed to be contacted by text', type: 'checkbox' }
];

export interface ExistingEngagement {
	kind: Kind;
	span: string;
	status: string;
}

export interface ExistingClient {
	id: string;
	given_name: string;
	family_name: string;
	preferred_name: string;
	date_of_birth: string;
	email: string;
	phone: string;
	address_locality: string;
	engagements: ExistingEngagement[];
}

/**
 * The Practice's book. What the save-time match query (#371) searches.
 */
export const existingClients: ExistingClient[] = [
	{
		id: 'c_priya',
		given_name: 'Priya',
		family_name: 'Raman',
		preferred_name: 'Priya',
		date_of_birth: '1991-03-14',
		email: 'priya.raman@oldmail.example',
		phone: '(917) 555-0163',
		address_locality: 'Astoria',
		engagements: [{ kind: 'birth', span: 'Feb 2024 – Sep 2024', status: 'completed' }]
	},
	{
		id: 'c_dana',
		given_name: 'Dana',
		family_name: 'Whitfield',
		preferred_name: 'Dana',
		date_of_birth: '1998-11-02',
		email: 'whitfieldhouse@example.com',
		phone: '(718) 555-0110',
		address_locality: 'Bay Ridge',
		engagements: []
	},
	{
		id: 'c_marion',
		given_name: 'Marion',
		family_name: 'Whitfield',
		preferred_name: 'Mar',
		date_of_birth: '1968-06-20',
		email: 'whitfieldhouse@example.com',
		phone: '(718) 555-0110',
		address_locality: 'Bay Ridge',
		engagements: [{ kind: 'postpartum', span: 'Jan 2023 – Apr 2023', status: 'completed' }]
	},
	{
		id: 'c_camille',
		given_name: 'Camille',
		family_name: 'Osei',
		preferred_name: 'Cam',
		date_of_birth: '1994-08-09',
		email: 'camille.osei@example.com',
		phone: '(347) 555-0128',
		address_locality: 'Crown Heights',
		engagements: [{ kind: 'birth', span: 'since Jun 2026', status: 'active' }]
	}
];

/**
 * The four walks the ticket names. `typed` is what the search box carried in.
 */
export interface Case {
	key: string;
	name: string;
	blurb: string;
	typed: string;
	seed: Partial<ClientDraft>;
	kind: Kind | '';
}

export const cases: Case[] = [
	{
		key: 'ordinary',
		name: 'Ordinary birth intake',
		blurb:
			'A first call from a woman due in the spring. Everything the form asks for exists, so this is the walk that says whether the form is too long.',
		typed: 'Rosa Delgado',
		seed: { given_name: 'Rosa', family_name: 'Delgado', preferred_name: 'Rosa' },
		kind: 'birth'
	},
	{
		key: 'postpartum',
		name: 'Postpartum-only, baby already here',
		blurb:
			'She gave birth nine days ago and wants postpartum support. There is no due date to type. The screen must not read as though something is missing.',
		typed: 'Aisha Bello',
		seed: { given_name: 'Aisha', family_name: 'Bello', preferred_name: 'Aisha' },
		kind: 'postpartum'
	},
	{
		key: 'bereavement',
		name: 'Bereavement support after a loss',
		blurb:
			'Her pregnancy ended. She is taken on for bereavement support. Nothing on this screen may assume a living baby (ADR-0015).',
		typed: 'Nell Farrow',
		seed: { given_name: 'Nell', family_name: 'Farrow', preferred_name: 'Nell' },
		kind: 'postpartum'
	},
	{
		key: 'returning',
		name: 'Returning Client, new email',
		blurb:
			'Priya was here in 2024 and has a new address since her divorce. The search was run on the new email and came back empty. The save-time match (#371) is the net that catches her.',
		typed: 'Priya Raman',
		seed: {
			given_name: 'Priya',
			family_name: 'Raman',
			preferred_name: 'Priya',
			email: 'priya@newmail.example'
		},
		kind: 'birth'
	},
	{
		key: 'household',
		name: 'Mother and daughter, one household email',
		blurb:
			'Dana shares an email and a phone with her mother Marion. No key separates them. This is what the deliberate override exists for.',
		typed: 'Dana Whitfield',
		seed: {
			given_name: 'Dana',
			family_name: 'Whitfield',
			preferred_name: 'Dana',
			email: 'whitfieldhouse@example.com',
			phone: '(718) 555-0110'
		},
		kind: 'postpartum'
	}
];

/**
 * AC 3 -- what the form demands is a decision, so it is a dial, not a default.
 */
export type Demands = 'name-only' | 'name-plus-reach' | 'name-reach-dob';

export const demandLabels: Record<Demands, string> = {
	'name-only': 'First name only',
	'name-plus-reach': 'First name + one way to reach her',
	'name-reach-dob': 'First name + reach + date of birth'
};

export function missingDemanded(client: ClientDraft, demands: Demands): string[] {
	const missing: string[] = [];
	if (!client.given_name.trim()) missing.push('First name');
	if (demands !== 'name-only' && !client.email.trim() && !client.phone.trim()) {
		missing.push('An email address or a phone number');
	}
	if (demands === 'name-reach-dob' && !client.date_of_birth.trim()) {
		missing.push('Date of birth');
	}
	return missing;
}

/**
 * #371's match query, re-run at save: name, date of birth, email, phone.
 */
export function matchExisting(client: ClientDraft): ExistingClient[] {
	const name = `${client.given_name} ${client.family_name}`.trim().toLowerCase();
	const email = client.email.trim().toLowerCase();
	const phone = client.phone.replaceAll(/\D/g, '');
	const dob = client.date_of_birth.trim();
	return existingClients.filter((existing) => {
		const existingName = `${existing.given_name} ${existing.family_name}`.toLowerCase();
		const existingPhone = existing.phone.replaceAll(/\D/g, '');
		return (
			(name.length > 0 && existingName === name) ||
			(email.length > 0 && existing.email.toLowerCase() === email) ||
			(phone.length > 0 && existingPhone === phone) ||
			(dob.length > 0 && existing.date_of_birth === dob)
		);
	});
}

/**
 * Second live Engagement -- #371 warns, never refuses.
 */
export function liveEngagement(existing: ExistingClient): ExistingEngagement | undefined {
	return existing.engagements.find((engagement) => engagement.status === 'active');
}

export function fullName(client: ClientDraft): string {
	return [client.given_name, client.family_name].filter(Boolean).join(' ') || 'this Client';
}
