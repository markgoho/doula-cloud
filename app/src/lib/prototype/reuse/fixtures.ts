// PROTOTYPE (#371) -- throwaway. Stubbed Clients for one Practice, chosen
// to cover the four cases the ticket says an exact-email lookup gets
// wrong or right. No API, no persistence.

export type PastEngagement = {
	kind: 'birth' | 'postpartum';
	status: 'intake' | 'active' | 'closed';
	span: string;
};

export type StubClient = {
	id: string;
	givenName: string;
	familyName: string;
	preferredName?: string;
	email?: string;
	phone?: string;
	locality?: string;
	engagements: PastEngagement[];
};

export const clients: StubClient[] = [
	{
		id: 'c-camille',
		givenName: 'Camille',
		familyName: 'Boyd',
		email: 'camille.boyd@example.com',
		phone: '(518) 555-0142',
		locality: 'Troy',
		engagements: [{ kind: 'birth', status: 'closed', span: 'Jan 2024 – Mar 2024' }]
	},
	{
		id: 'c-priya',
		givenName: 'Priya',
		familyName: 'Raman',
		email: 'priya.raman@oldmail.example',
		phone: '(518) 555-0198',
		locality: 'Albany',
		engagements: [{ kind: 'birth', status: 'closed', span: 'Feb 2023 – May 2023' }]
	},
	{
		id: 'c-marisol',
		givenName: 'Marisol',
		familyName: 'Whitfield',
		preferredName: 'Mari',
		email: 'whitfields@example.com',
		phone: '(518) 555-0110',
		locality: 'Cohoes',
		engagements: [{ kind: 'postpartum', status: 'closed', span: 'Sep 2022 – Nov 2022' }]
	},
	{
		id: 'c-alina',
		givenName: 'Alina',
		familyName: 'Novak',
		email: 'alina.novak@example.com',
		phone: '(518) 555-0177',
		locality: 'Watervliet',
		engagements: [{ kind: 'birth', status: 'active', span: 'since Jun 2026' }]
	}
];

// The four cases. `typed` is what the staff member enters; `expect` is
// the trap the ticket names.
export type Scenario = {
	key: string;
	label: string;
	trap: string;
	typed: { givenName: string; familyName: string; email: string; phone: string };
};

export const scenarios: Scenario[] = [
	{
		key: 'exact',
		label: 'Camille Boyd — served in 2024, exact email',
		trap: 'The easy case. An exact email match, and the right answer is reuse.',
		typed: {
			givenName: 'Camille',
			familyName: 'Boyd',
			email: 'camille.boyd@example.com',
			phone: '(518) 555-0142'
		}
	},
	{
		key: 'renamed',
		label: 'Priya Raman — same woman, new email after a divorce',
		trap: 'Email lookup finds nothing. Name and phone both match an existing Client.',
		typed: {
			givenName: 'Priya',
			familyName: 'Raman',
			email: 'priya@newmail.example',
			phone: '(518) 555-0198'
		}
	},
	{
		key: 'household',
		label: 'Dana Whitfield — shares a household email with her mother',
		trap: 'Email lookup matches Marisol Whitfield. Wrong person, right address.'
		,
		typed: {
			givenName: 'Dana',
			familyName: 'Whitfield',
			email: 'whitfields@example.com',
			phone: '(518) 555-0231'
		}
	},
	{
		key: 'live',
		label: 'Alina Novak — already has a live Engagement',
		trap: 'Right person, but a second Engagement would run alongside a live one.',
		typed: {
			givenName: 'Alina',
			familyName: 'Novak',
			email: 'alina.novak@example.com',
			phone: '(518) 555-0177'
		}
	},
	{
		key: 'none',
		label: 'Rosa Delgado — genuinely new',
		trap: 'Nothing matches. Intake must not slow down for her.',
		typed: {
			givenName: 'Rosa',
			familyName: 'Delgado',
			email: 'rosa.delgado@example.com',
			phone: '(518) 555-0264'
		}
	}
];

export type Match = { client: StubClient; on: 'email' | 'phone' | 'name'; confidence: 'exact' | 'near' };

// Deliberately keyed on more than email, so the prototype can show what
// an email-only lookup misses (Priya) and what it wrongly finds (Dana).
export function findMatches(typed: { givenName: string; familyName: string; email: string; phone: string }): Match[] {
	const found: Match[] = [];
	for (const client of clients) {
		if (typed.email && client.email && client.email.toLowerCase() === typed.email.toLowerCase()) {
			found.push({ client, on: 'email', confidence: 'exact' });
			continue;
		}
		if (typed.phone && client.phone === typed.phone) {
			found.push({ client, on: 'phone', confidence: 'near' });
			continue;
		}
		const sameName =
			typed.givenName.trim().toLowerCase() === client.givenName.toLowerCase() &&
			typed.familyName.trim().toLowerCase() === client.familyName.toLowerCase();
		if (sameName) found.push({ client, on: 'name', confidence: 'near' });
	}
	return found;
}

export function search(query: string): StubClient[] {
	const q = query.trim().toLowerCase();
	if (q.length < 2) return [];
	return clients.filter((c) =>
		[c.givenName, c.familyName, c.preferredName, c.email, c.phone, `${c.givenName} ${c.familyName}`, c.preferredName ? `${c.preferredName} ${c.familyName}` : undefined]
			.filter(Boolean)
			.some((f) => String(f).toLowerCase().includes(q))
	);
}

export function displayName(c: StubClient): string {
	return `${c.preferredName ?? c.givenName} ${c.familyName}`;
}

export function hasLiveEngagement(c: StubClient): boolean {
	return c.engagements.some((e) => e.status !== 'closed');
}

// --- Disclosure axis -------------------------------------------------
// What the prompt is allowed to print about an existing Client. The
// ticket says this deserves a decision rather than a default, so the
// prototype makes it a switch rather than picking one.
export type Disclosure = 'confirm-only' | 'named' | 'full';

export const disclosureLabels: Record<Disclosure, string> = {
	'confirm-only': 'Confirm only (no name)',
	named: 'Name only',
	full: 'Name + history'
};

export function matchHeadline(match: Match, level: Disclosure): string {
	if (level === 'confirm-only') {
		const on = match.on === 'email' ? 'email address' : match.on === 'phone' ? 'phone number' : 'name';
		return `A Client at this Practice already uses this ${on}.`;
	}
	return `${displayName(match.client)} may already be a Client here.`;
}

export function matchDetail(match: Match, level: Disclosure): string[] {
	if (level !== 'full') return [];
	const c = match.client;
	return [
		c.email ? `Email ${c.email}` : 'No email on file',
		c.phone ? `Phone ${c.phone}` : 'No phone on file',
		...c.engagements.map((e) => `${e.kind === 'birth' ? 'Birth' : 'Postpartum'} — ${e.span} (${e.status})`)
	];
}
