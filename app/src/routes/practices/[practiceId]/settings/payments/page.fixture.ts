/*
 * The Stripe Connect settings screen, as the continuum check sees it
 * (#595).
 *
 * `requirementsDue` is a list of Stripe field paths, not Practice-typed
 * content, and `website` only gates a boolean here -- so this measures
 * the widest realistic status combination rather than any hostile
 * string.
 *
 * This screen reads `isOwner` and `isOwnerOrAdmin` and renders three
 * different trees from them, so it declares all three (#913): the Owner's
 * checklist of what Stripe will ask her for, the Admin's shorter reading
 * of the same account, and the single Notice anyone else meets. Only the
 * Owner's was ever swept before, because a fixture could name only one
 * session.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ConnectStatusResult } from '#lib/payments.js';
import type { PracticeWebsite } from '#lib/website.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const status: ConnectStatusResult = {
	status: 'payouts_restricted',
	cardPaymentsStatus: 'active',
	payoutsStatus: 'restricted',
	requirementsDue: ['individual.verification.document', 'external_account', 'business_profile.url']
};

const website: PracticeWebsite = {
	mode: 'hosted',
	ownUrl: '',
	serviceDescription: 'Full-spectrum birth and postpartum doula support',
	cancellationPolicy: 'Full refund up to 30 days before the due date',
	updatedBy: 'Anne-Marie Ochieng-Whitfield',
	updatedAt: '2026-08-01T00:00:00Z',
	pageState: 'live',
	pageCheckedAt: '2026-08-01T00:00:00Z',
	pageCheckDetail: '',
	pageUrl: 'https://doula.cloud/p/riverside-doula-collective'
};

/*
 * The Admin's branch. She reads the same status the Owner does -- ADR-0008
 * puts the Connect state row in Owner-and-Admin hands (#267) -- so it
 * answers the same two fetches, and the tree differs after that: the
 * requirements count is written to her as a fact about the Practice's
 * account rather than an errand of her own, and the Owner's checklist and
 * button are replaced by one sentence naming who has to do it (#916).
 *
 * Its content is inherited deliberately rather than by omission. What this
 * branch renders that the Owner's does not is repo copy, not Practice-typed
 * content, and its own longest line is the plural requirements sentence --
 * "Stripe needs 3 more details from a Practice Owner." -- which the
 * inherited three-item `requirementsDue` already selects. A hostile value
 * of its own would have nowhere to sit.
 */
export const asAdmin: RouteVariant = {
	name: 'The Stripe Connect settings screen, as an Admin',
	pageData: practiceSession(['admin'])
};

/*
 * Anyone else's branch: one Notice and nothing further. She is not asked
 * for the status at all -- the screen returns before either fetch -- so
 * this branch answers no request, and its `respond` is the inherited one
 * only because nothing reaches it.
 */
/*
 * #768's payment-terms fieldset. The default state, not a Practice's own
 * chosen number, because it is the longer of the two sentences the
 * section renders (#537: the longest realistic value, not a
 * representative one) -- a Practice that has set its own reads the short
 * form, which cannot need more room than this one.
 */
const paymentTerms = { netDays: 30, isDefault: true };

/* This screen's three fetches, answered once: the website and the payment
   terms are the same on every branch, and the Connect status is the only
   thing a branch has any reason to change. Both the base fixture and
   #917's variant below build their `respond` from it. */
function respondWith(connectStatus: ConnectStatusResult): (path: string) => Response {
	return (path) => {
		if (path.endsWith('/website')) return jsonResponse(website);
		if (path.endsWith('/payments/payment-terms')) return jsonResponse(paymentTerms);
		return jsonResponse(connectStatus);
	};
}

// `isContractor` is not read on this screen, so a contractor Doula and an
// employee Doula meet the same Notice.
export const asDoula: RouteVariant = {
	name: 'The Stripe Connect settings screen, as a Doula',
	pageData: practiceSession(['doula'])
};

/*
 * #917's own branch, and the one reason this screen needs a fourth
 * session rather than three. The Admin variant above inherits a
 * `payouts_restricted` account with three outstanding requirements, which
 * is a Practice #343's payout Notification has already mailed -- so she
 * reads a single sentence saying so and is offered nothing. A Practice
 * with no Stripe account at all is the state no webhook can reach
 * (ADR-0035), and it is the only one that puts a control on her screen:
 * a sentence, and a button under it. That tree is not a subset of any
 * branch declared above, so the sweep would otherwise never mount it.
 *
 * A variant's `respond` is a replacement rather than a merge, so this one
 * would have restated the website and payment-terms answers verbatim.
 * `respondWith` is those two written once, taking only the thing that
 * actually differs -- which is the spread the fixture rule asks for, in
 * the one form a function-valued field can take it.
 *
 * `requirementsDue` is empty because a Practice with no account has
 * nothing outstanding: the count sentence is one the Admin variant above
 * already carries.
 */
export const asAdminWithNoAccount: RouteVariant = {
	name: 'The Stripe Connect settings screen, as an Admin with no Stripe account',
	pageData: practiceSession(['admin']),
	respond: respondWith({
		status: 'not_connected',
		cardPaymentsStatus: 'unsupported',
		payoutsStatus: 'unsupported',
		requirementsDue: []
	})
};

export const fixture: RouteFixture = {
	name: 'The Stripe Connect settings screen, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/payments',
	pageData: practiceSession(['owner']),
	respond: respondWith(status),
	readyText: 'Getting paid',
	variants: [asAdmin, asAdminWithNoAccount, asDoula]
};
