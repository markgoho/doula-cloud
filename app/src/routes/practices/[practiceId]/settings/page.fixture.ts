/*
 * The Settings index, as the continuum check sees it (#595, #2028).
 *
 * Four headed groups of summary rows, each with the setting's current
 * value, and deletion set apart. A row is drawn only for someone who may
 * change it (`MAY_CHANGE`), so the index depends on who is looking.
 *
 * Three subjects (#913). The Owner's is the widest. A Doula's is the
 * narrowest: one group, one row, Plan Templates. An Owner with no second
 * factor (#1532) draws a tree nobody else does, the export row's notice
 * and enrollment link in place of a download. The Admin's sits between
 * the Owner's and the Doula's and is a strict subset of the Owner's (it
 * drops the Owner-only rows and the deletion section and adds none), so
 * it is left undeclared rather than swept as a fourth copy.
 *
 * `respond` answers every read the Owner's index makes, with each value
 * at its longest: the hosted page's address is #530's long URL, the
 * rates are both set and in the thousands, the Connect state is the
 * longest of its five labels, and the plural forms are all reached. The
 * Doula's branch asks for the Plan Templates only, which this answers too.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeWebsite } from '#lib/website.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const pageUrl =
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

export const website: PracticeWebsite = {
	mode: 'hosted',
	ownUrl: '',
	serviceDescription: 'Birth and postpartum support across Monroe County.',
	cancellationPolicy: 'Two weeks notice for a full refund.',
	updatedBy: 'Anne-Marie Ochieng-Whitfield',
	updatedAt: '2026-08-01T00:00:00Z',
	pageState: 'live',
	pageCheckedAt: '2026-08-01T00:00:00Z',
	pageCheckDetail: '',
	pageUrl
};

const question = (id: string, type = 'short_text') => ({ id, type, label: id, order: 0, archived: false });

export function respond(path: string): Response {
	const answers: Record<string, unknown> = {
		website,
		timezone: { timezone: 'America/Indiana/Indianapolis' },
		'client-field-template': {
			fields: [question('a'), question('b', 'section_header'), question('c'), { ...question('d'), archived: true }]
		},
		'plan-templates/care_plan': { planType: 'care_plan', fields: [question('a'), question('b'), question('c')] },
		'plan-templates/birth_plan': { planType: 'birth_plan', fields: [question('a')] },
		'contract-template': { prose: 'These are the terms.' },
		'on-call-settings': { startRule: 'attachment_granted', startWeek: 37, graceDays: 14 },
		'payments/connect': {
			status: 'payouts_restricted',
			cardPaymentsStatus: 'active',
			payoutsStatus: 'restricted',
			requirementsDue: []
		},
		rates: {
			rates: [
				{ kind: 'birth', amountCents: 1_234_500 },
				{ kind: 'postpartum', amountCents: 987_600 }
			]
		},
		billing: {
			balance: 128,
			ledger: { items: [], hasMore: false },
			price: { unitAmountCents: 2500, currency: 'usd' }
		},
		'mfa-required/impact': { required: true, withoutSecondFactor: 3 },
		'email-suppressions': {
			suppressions: [
				{ address: 'a@example.test', cause: 'bounce', createdAt: '2026-08-01T00:00:00Z', clearable: true },
				{ address: 'b@example.test', cause: 'complaint', createdAt: '2026-08-01T00:00:00Z', clearable: false }
			]
		}
	};
	const key = path.replace(/^\/api\/practices\/[^/]+\//, '');
	const body = answers[key];
	return body === undefined ? jsonResponse({ message: `no answer for ${path}` }, 404) : jsonResponse(body);
}

export const asDoula: RouteVariant = {
	name: 'The Settings index, as a Doula',
	pageData: practiceSession(['doula'])
};

export const ownerWithoutSecondFactor: RouteVariant = {
	name: 'The Settings index, as an Owner with no second factor',
	pageData: practiceSession(['owner'], { secondFactor: false })
};

export const fixture: RouteFixture = {
	name: 'The Settings index, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings',
	pageData: practiceSession(['owner']),
	respond,
	readyText: 'Settings',
	variants: [asDoula, ownerWithoutSecondFactor]
};
