/*
 * The Staff-side Engagement detail hub, as the continuum check sees it
 * (#595).
 *
 * `onMount` cascades through detail, Visits, Messages, both Plan
 * sections, the Contract, Invoices and (Owner/Admin only) Offers, in
 * that order (see the route's own `onMount`) -- `respond` answers every
 * one of them so the cascade completes rather than stalling partway
 * through. `detail.clientName` is the title, gated on load exactly like
 * the existing approval-screen fixture, and carries #537's hyphenated
 * double-barreled name; the Contract's merge-field values and a Visit's
 * `staffName` carry it again, since both are exactly the free-text shape
 * `DataTable`/`ContractView` were already fixed against.
 *
 * Two sessions (#928). This route reads four predicates -- `isOwner`,
 * `isOwnerOrAdmin`, `isDoula` and `isAmbientContractor` -- and hands
 * three of them down to `ContractStatus`, `InvoiceSection` and
 * `BirthOutcomeSection` as props, so the branching continues below the
 * route and the subset question has to be asked there too.
 *
 * Nearly all of it is additive, and the two reads a narrower session is
 * refused -- the roster and Offers -- answer `undefined` rather than
 * throwing, so their sections are left out silently instead of drawing a
 * refusal; the Contract and the Invoices are `staffauth.AnyStaff`, so no
 * session meets an error Notice here either. One branch is not additive,
 * and it is the reason this route has a second declared session: #971's
 * `onRequestVoid` is wired the other way round -- `isOwnerOrAdmin ?
 * undefined : handleRequestVoidContract` -- so a Doula who is neither
 * Owner nor Admin reads a whole affordance the base cannot draw. See
 * `asDoula` below.
 *
 * Everything else is the base with things taken away. An Admin loses the
 * InvoiceSection's Owner link and her own Visit default. A contractor
 * Doula loses the Contract's PDF download and the birth-outcome control,
 * and since #2002 reads one information line in place of the Invoice
 * list, which no other session draws, so she is `asContractor` below.
 */
import type { Contract } from '#lib/contract.js';
import type { ContractorInvoice, Invoice } from '#lib/invoice.js';
import { jsonResponse } from '#lib/testResponse.js';
import type { EngagementOnCall } from '#lib/onCall.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS, TYPICAL_DOULAS, TYPICAL_STAFF } from '../../../../typicalPilot.js';
import { detail as clientDetail } from '../../clients/[clientId]/page.fixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './EngagementScreen.svelte';

const clientName = 'Anne-Marie Ochieng-Whitfield';

/*
 * Her record (#1610), read only to word the link to her details journey.
 * The Client detail hub's own record, renamed to the Client this
 * Engagement is for and with no phone number on file, so the link reads
 * "Add", the longer of its two wordings.
 */
const clientRecord = {
	...clientDetail,
	id: 'client-1',
	givenName: 'Anne-Marie',
	familyName: 'Ochieng-Whitfield',
	phone: ''
};

/*
 * A roster the size of the pilot's own agency -- fourteen Doulas, plus a
 * bookkeeper who holds no Doula role and so never belongs in a Visit
 * picker. The names are long and double-barreled on purpose: the two
 * Visit pickers (#268) are `<select>`s inside a `DataTable` row-actions
 * cell, and the continuum sweep renders this fixture from 320px up
 * (ADR-0024, ADR-0025), so the widest realistic name is what the check
 * has to survive.
 */
const doulaNames = [
	'Anne-Marie Ochieng-Whitfield',
	'Jordan Reyes',
	'Priyanka Venkataraman-Solberg',
	'Guadalupe Fernández-Castellanos',
	'Nkechi Adaeze Onyekwere-Balogun',
	'Siobhán Ní Mhurchadha-Kavanagh',
	'Maria Aparecida do Nascimento',
	'Tuiasosopo Faamausili-Leota',
	'Beatrix Vandenberghe-Koopmans',
	'Aleksandra Wiśniewska-Rutkowski',
	'Chidinma Oluwaseun Adeyemi-Cole',
	'Rosalind Featherstonehaugh-Payne',
	'Xiomara Delgado-Villanueva',
	'Kanyakumari Balasubramanian'
];

const roster = [
	...doulaNames.map((name, index) => ({
		staffId: `staff-${index + 1}`,
		name,
		email: `doula-${index + 1}@example.test`,
		roles: index === 0 ? ['owner', 'doula'] : ['doula'],
		employmentType: index % 3 === 0 ? 'contractor' : 'employee',
		workState: 'NY',
		workStateReportedAt: '2026-01-01T00:00:00Z'
	})),
	{
		staffId: 'staff-bookkeeper',
		name: 'Winifred Abernathy-Castellano',
		email: 'books@example.test',
		roles: ['admin'],
		employmentType: 'employee',
		workState: 'NY',
		workStateReportedAt: '2026-01-01T00:00:00Z'
	}
];

/*
 * Who may be named on a Visit at *this* Engagement (#911) -- what the two
 * Visit pickers are drawn from now, rather than the Practice-wide roster
 * above. Every third roster member is a contractor, and none of them is
 * attached here, so the picker has to carry the widest realistic name
 * *plus* its "(cannot be named yet)" marker at 320px (ADR-0024,
 * ADR-0025). `staff-1` is the caller -- an Owner who is also a Doula, and
 * a contractor -- and she is nameable regardless, because naming yourself
 * takes the self rule and never the attachment one.
 */
export const visitAssignees = roster
	.filter((member) => member.roles.includes('doula'))
	.map(({ staffId, name, employmentType }) => {
		const canBeNamed = staffId === 'staff-1' || employmentType === 'employee';
		return {
			staffId,
			name,
			employmentType,
			nameable: canBeNamed,
			...(!canBeNamed && { reason: 'contractor_without_accepted_offer' })
		};
	});

/*
 * #1093's on-call panel for this birth: a window, a primary on call for
 * all of it, a backup narrowed to the last fortnight, and two gaps --
 * one nobody is covering and one somebody is. The two gap states are
 * both here because the section renders them differently, which is the
 * fixture rule #720 states.
 */
export const onCallPanel: EngagementOnCall = {
	window: { start: '2026-10-09', end: '2026-11-13' },
	rule: {
		startRule: 'gestational_week',
		startWeek: 37,
		graceDays: 14,
		overridden: false
	},
	doulas: [
		{
			staffId: 'staff-1',
			name: 'Persephone Vandermeulen-Achterberg, CD(DONA)',
			onCallFrom: '2026-10-09',
			onCallTo: '2026-11-13'
		},
		{
			staffId: 'staff-2',
			name: 'Bo Ng',
			from: '2026-10-31',
			onCallFrom: '2026-10-31',
			onCallTo: '2026-11-13'
		}
	],
	gaps: [
		{
			id: 'gap-1',
			engagementId: 'engagement-1',
			staffId: 'staff-1',
			staffName: 'Persephone Vandermeulen-Achterberg, CD(DONA)',
			startsAt: '2026-10-17T22:00:00Z',
			endsAt: '2026-10-18T10:00:00Z',
			reason:
				'At a wedding four hours away with no reliable signal, and back the following morning'
		},
		{
			id: 'gap-2',
			engagementId: 'engagement-1',
			staffId: 'staff-2',
			staffName: 'Bo Ng',
			startsAt: '2026-11-01T02:00:00Z',
			endsAt: '2026-11-01T14:00:00Z',
			coveringStaffId: 'staff-1',
			coveringStaffName: 'Persephone Vandermeulen-Achterberg, CD(DONA)'
		}
	]
};

export const detail = {
	engagementId: 'engagement-1',
	clientId: 'client-1',
	// #2004: the conversation name the page's sentences use, and the
	// title, her record's own: she has no preferred name, so given and
	// family names together.
	clientName: clientRecord.givenName,
	clientDisplayName: clientName,
	status: 'active',
	createdAt: '2026-08-01T00:00:00Z',
	// #874: what the Practice sold. 'birth' agrees with the recorded birth
	// outcome below -- a postpartum Engagement never carries one -- and is
	// what earns the Kind section's "Change to Postpartum" control its own
	// row in the sweep.
	kind: 'birth',
	dueDate: '2027-03-01',
	// #253: the one legal move from 'active' every role in the fixture's
	// own role table reaches, so the continuum check sees the status
	// section's "Mark care complete" control rendered, not hidden.
	statusMoves: ['completed'],
	// #943: a recorded pair, so the sweep sees the Birth outcome section's
	// read-back and both of an Owner's controls beside it rather than only
	// its "nothing recorded yet" line. Recorded on an `active` Engagement
	// on purpose -- ADR-0015 forbids auto-completion on a recorded
	// outcome, because postpartum and bereavement care continue after it.
	// The contractor Doula branch is not a variant here: her tree is a
	// strict subset of this one (the same read-back, no controls), so it
	// realizes nothing this fixture has not already measured, and
	// BirthOutcomeSection's own style-guide page sweeps it directly.
	birthOutcome: 'live_birth',
	pregnancyEndedOn: '2026-08-14',
	// #1598: who is on this Engagement. Two Doulas, the on-call panel's
	// own primary and backup, so the summary's row holds its busiest
	// state: the plural label and two names on one line, the first of
	// them the longest name on this screen. The reader is one of them
	// (`staff-1`), so the BFF answers `canAttachSelf: false`: she is on
	// it already. That is the only reason. She is an Owner who is a
	// contractor Doula, and the BFF offers that reader the control on an
	// Engagement she is not on (#1625). The empty state and the control
	// are `asDoula`'s, below.
	doulas: [
		{ staffId: 'staff-1', name: 'Persephone Vandermeulen-Achterberg, CD(DONA)' },
		{ staffId: 'staff-2', name: 'Bo Ng' }
	],
	canAttachSelf: false,
	// #2003: the reader's own agreed fee. She is an Owner who is a
	// contractor Doula on this Engagement, the person ADR-0008's amendment
	// on #1625 still offers work with a fee, so the Contract section's
	// "Your fee" row is drawn on the base. A four-figure fee, the widest
	// the row gets.
	yourFee: { amountCents: 1_262_000, acceptedOn: '2026-07-22' }
};

/*
 * The Contract this Engagement carries (#885). Until this existed the
 * route's `respond` answered `/contract` with a 404, so the Contract
 * section rendered nothing but its "Create Draft Contract" button and
 * the sweep never saw the section's own controls at any width -- not
 * #302's signed-PDF download, and not the Void button that predates it.
 *
 * An Engagement holds one Contract, so "a row set realizing every state
 * a field renders differently" (ADR-0025's Fixtures section) resolves
 * here to one Contract carrying every render-distinct state at once
 * rather than to several rows. `signed` is the status that earns them:
 * ContractStatus offers Download and Void only on a signed Contract, and
 * `voided` renders a strict subset of it (a terminal notice and nothing
 * to click). `amountChangedAt` and `voidRequests` are the section's other
 * two presence-branched fields, both absent from the component's own
 * style-guide demos, so this is the only place either is swept: one open
 * request draws an Owner's Decline control beside who asked and why, and
 * one declined request draws the answer the asker reads afterwards.
 *
 * The `draft` branch is not declared here: it is a different Contract,
 * not a different session, and the route spec reaches it by spreading
 * this one. The Doula branch -- who is offered "Request void" rather than
 * Void -- is a session, and is now declared as `asDoula` below
 * ([#928](https://github.com/markgoho/doula-cloud/issues/928)). This
 * Contract's `signed` status is what makes that branch reachable at all:
 * `ContractStatus` gates the whole request-void affordance on
 * `status === 'signed' && onRequestVoid`.
 *
 * The values are hostile per #537: the Client's own double-barreled name
 * again, the Practice's full name, a four-figure price, a scope that runs
 * to a real paragraph, and a URL inside a void request's reason -- the
 * one shape that made the baseline sweep on #521 go red at all.
 */
export const contract: Contract = {
	engagementId: 'engagement-1',
	status: 'signed',
	hasSignedPdf: true,
	prose:
		'This agreement is between {{practice_name}} and {{client_name}}, for {{scope_of_service}}, beginning {{engagement_start_date}} and ending {{engagement_end_date}}, for a fee of {{price}}. Cancellation, refund and on-call terms are published at https://riverside-doula-collective.example.test/policies/client-agreement and are incorporated here by reference.',
	mergeFields: [
		'practice_name',
		'client_name',
		'scope_of_service',
		'engagement_start_date',
		'engagement_end_date',
		'price'
	],
	values: {
		practice_name: 'Riverside Doula Collective',
		client_name: clientName,
		scope_of_service:
			'Two prenatal visits, continuous labor support from active labor through the first hour after birth, one postpartum visit in the home, and unlimited phone and message support from 37 weeks until two weeks after the birth.',
		engagement_start_date: 'January 12, 2027',
		engagement_end_date: 'March 29, 2027',
		price: '$4,500.00'
	},
	// #968: the price moved after this Contract was created, so the
	// section's "Price changed on" line renders rather than staying absent.
	amountChangedAt: '2026-08-19T00:00:00Z',
	// #2104: how it came to be -- sent by a Staff member and signed under
	// a typed legal name, both as long as a real one runs.
	sent: { at: '2026-08-12T15:00:00Z', by: 'Anne-Marie Ochieng-Whitfield' },
	signed: { at: '2026-08-14T20:12:00Z', by: 'Oluwaseun Adebayo-Montgomery Fitzwilliam' },
	// #971: one request still waiting on an Owner's decision, and one
	// already declined -- the two states ContractStatus renders
	// differently. A granted request needs no row: `status` reads 'voided'
	// by the time one exists, which is a different Contract than this.
	voidRequests: [
		{
			id: 'void-request-1',
			requestedBy: 'staff-3',
			requestedByName: 'Priyanka Venkataraman-Solberg',
			reason:
				'The Client moved her care to a hospital outside our service area and asked to end the agreement; her message is at https://example.test/practices/practice-1/engagements/engagement-1/messages/message-1.',
			status: 'open',
			createdAt: '2026-08-20T00:00:00Z'
		},
		{
			id: 'void-request-2',
			requestedBy: 'staff-4',
			requestedByName: 'Guadalupe Fernández-Castellanos',
			reason: 'Asked to void after the due date moved by three weeks.',
			status: 'declined',
			declineReason:
				'A moved due date does not end the agreement — the dates on it are indicative, and the scope of service has not changed.',
			decidedBy: 'staff-1',
			decidedAt: '2026-08-21T00:00:00Z',
			createdAt: '2026-08-18T00:00:00Z'
		}
	]
};

/*
 * The Membership `practices/[practiceId]/+layout.ts` merges in -- an Owner
 * who is also a Doula, the role that sees every control this route draws.
 * Exported so the route's own spec mounts the same reader the sweep does
 * rather than describing a second one beside it (#885).
 */
/*
 * A Birth Plan in progress (#2004): a section header and three questions,
 * one answered, so the read view's Progress row reads "1 of 3 questions
 * answered" and its Client row says it has not been read.
 */
export const birthPlan = {
	engagementId: 'engagement-1',
	planType: 'birth_plan',
	fields: [
		{ id: 'bp-header', type: 'section_header', label: 'Labor and birth', order: 0 },
		{ id: 'bp-atmosphere', type: 'long_text', label: 'Preferences for atmosphere (music, lighting, etc.)', order: 1 },
		{ id: 'bp-support', type: 'short_text', label: 'Who will be with you', order: 2 },
		{ id: 'bp-feeding', type: 'single_select', label: 'Feeding plan', options: ['Breast', 'Bottle', 'Both'], order: 3 }
	],
	answers: { 'bp-atmosphere': 'Low lights and the playlist her partner made.' },
	// #2133: its start, which the last save's row replaces on screen.
	started: { at: '2026-08-20T15:00:00Z', by: 'Anne-Marie Ochieng-Whitfield' },
	// #2104: its last save, by the longest name the roster holds.
	lastEdited: { at: '2026-08-26T15:00:00Z', by: 'Priyanka Venkataraman-Solberg' }
};

/*
 * A Care Plan started and never saved (#2133): its one audit row is the
 * start, so the Plan section's other branch is swept too.
 */
const carePlan = {
	...birthPlan,
	planType: 'care_plan',
	answers: {},
	lastEdited: undefined,
	started: { at: '2026-08-21T15:00:00Z', by: 'Priyanka Venkataraman-Solberg' }
};

export const session = {
	practiceId: 'practice-1',
	// #909: which roster entry the reader herself is. The type has always
	// required it and the fixture's inline object had always omitted it,
	// which `props`'s own `Record<string, unknown>` could not catch --
	// naming the export is what surfaced it. `staff-1` is the roster's
	// Owner-Doula, so the Visit picker opens on her the way it does for a
	// real reader who is on the roster.
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner', 'doula'],
	// The roster above already says `staff-1` is a contractor, and
	// `visitAssignees` leans on it for the self rule, so naming the
	// `staffId` made the old `false` a contradiction rather than an
	// omission. It changes nothing the screen draws: `isAmbientContractor`
	// is `isContractor && !isOwnerOrAdmin`, so an Owner who contracts is
	// never the ambient contractor ADR-0008 confines. The rule for who is
	// attached without an Offer confines that same contractor and no
	// other (#1625), so this reader may put herself on an Engagement.
	isContractor: true
};

/*
 * The Doula's own reading of this Engagement (#928), and the one session
 * besides the base with something of its own on screen.
 *
 * #971 wires `onRequestVoid` the opposite way from every other role prop
 * on this page -- `isOwnerOrAdmin ? undefined : handleRequestVoidContract`
 * -- because asking for a signed Contract to be voided is the errand of
 * somebody who cannot void it herself. So `ContractStatus` opens a whole
 * region to her that the base fixture's Owner-Doula cannot reach, and
 * until now it was swept never.
 *
 * Which of that region's three states she lands in is the inherited
 * Contract's doing, and the widest of them is the one she gets: the
 * Contract above already carries an open void request, so she reads
 * "Void requested -- waiting for an owner or admin to decide." where the
 * Owner reads her Decline control. The other two are a single secondary
 * Button, and the reason form behind it, which needs a click the sweep
 * never makes.
 *
 * An employee Doula rather than a contractor one: the contractor's own
 * screen is `asContractor`, below.
 *
 * `props`, not `pageData`: this route reads `data.session`, handed to it
 * by its own `+page.ts` (#695), and `props` is restated whole -- so the
 * Engagement itself is spread back in beside the changed session.
 * `respond` is inherited deliberately. It answers the roster and the
 * Offers a real Doula would be refused, which leaves her holding two
 * sections she would not really have; that costs nothing, because both
 * are already measured on the base, and the alternative is a second
 * `respond` restating twelve answers to remove content rather than add
 * it. The fixture is already deliberately not role-consistent for the
 * same reason its Clients-list sibling is: what the sweep asks is how
 * much room the widest line needs.
 */
export const asDoula: RouteVariant<RouteParameters> = {
	name: 'The Staff-side Engagement detail hub, as a Doula',
	// #1598: the other state of the summary's Doula row, and the control
	// that goes with it -- nobody is on the Engagement, so the row reads
	// "No Doula yet" and an employee Doula is offered "Put me on this
	// Engagement". It rides this variant because the base has its Doulas
	// on it, the reader among them, and the sweep needs the empty row.
	// The base's Owner, a contractor Doula, is offered the control too on
	// an Engagement she is not on (#1625).
	props: {
		data: {
			...detail,
			doulas: [],
			canAttachSelf: true,
			session: { ...session, roles: ['doula'], isContractor: false }
		}
	}
};

/*
 * The contractor Doula's own reading (#2002, #2094). She is not a strict
 * subset of `asDoula`: her Invoices section is a table of dates and
 * status with no money, and an information line, which no other session
 * draws. `props` for the same reason `asDoula` gives. Its own `respond`
 * for the one path whose answer is hers alone, the rest the base's: one
 * row of each state her rows draw differently -- an Invoice counted late
 * (Overdue, with no Paid day), one paid (a Paid day), and one void.
 */
export const contractorInvoices: ContractorInvoice[] = [
	{ id: 'invoice-late', status: 'open', raisedOn: '2026-08-01', dueOn: '2026-08-31', daysOverdue: 12 },
	{
		id: 'invoice-paid',
		status: 'paid',
		raisedOn: '2026-07-01',
		dueOn: '2026-07-31',
		paidOn: '2026-07-20',
		daysOverdue: 0
	},
	{ id: 'invoice-void', status: 'void', raisedOn: '2026-06-01', dueOn: '2026-07-01', daysOverdue: 0 }
];

export const asContractor: RouteVariant<RouteParameters> = {
	name: 'The Staff-side Engagement detail hub, as a contractor Doula',
	props: {
		data: {
			...detail,
			session: { ...session, roles: ['doula'], isContractor: true }
		}
	},
	respond: (path) => {
		if (path.endsWith('/contract/invoices')) return jsonResponse({ items: contractorInvoices, hasMore: false });
		return fixture.respond!(path);
	}
};

/*
 * #1885: the Contract section's other tree -- no Contract yet, and no rate
 * for this Engagement's kind, so "Create Draft Contract" is withheld with
 * the reason and, for the base's Owner, the link to the Rates page. Its
 * own `respond`, because the branch is two answers the base gives
 * differently: `/contract` is a 404 and `/rates` holds no amount. Every
 * other path is the base's answer.
 */
export const withNoRate: RouteVariant<RouteParameters> = {
	name: 'The Staff-side Engagement detail hub, with no Contract and no rate for its kind',
	respond: (path) => {
		if (path.endsWith('/contract')) return jsonResponse('not found', 404);
		if (path.endsWith('/rates')) {
			return jsonResponse({
				rates: [
					// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
					{ kind: 'birth', amountCents: null },
					// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
					{ kind: 'postpartum', amountCents: null }
				]
			});
		}
		return fixture.respond!(path);
	}
};

/*
 * Typical pilot data (#1981), the screen this route's two table floors are
 * measured on (`typicalPilot.ts` says where each value comes from). Camille
 * Boyd's birth Engagement at a 14-doula agency, with one Doula on it:
 * five Visits so far, each logged by that Doula with a short note or none
 * (Nadia Haddad's World Engagement has eleven by its twentieth week), the
 * agency's own roster in the Visit pickers, and the ledger's ordinary
 * week -- Visits logged, a Contract sent, an Invoice raised. The long note,
 * the long names and the corrected birth outcome stay on the hostile
 * screen.
 */
const typicalClientName = TYPICAL_CLIENTS[0]!.name;
const typicalDoula = nth(TYPICAL_DOULAS, 2);
const typicalDetail = {
	...detail,
	clientName: TYPICAL_CLIENTS[0]!.givenName,
	clientDisplayName: typicalClientName,
	// #2004: a Client in care has a portal account, so the Messages
	// section reads her thread with its form behind one opener.
	clientPortalInviteStatus: 'accepted',
	doulas: [{ staffId: typicalDoula.staffId, name: typicalDoula.name }]
};
const typicalInvoices: Invoice[] = [
	{
		id: 'invoice-balance',
		contractId: 'contract-1',
		status: 'open',
		amountCents: 90_000,
		currency: 'usd',
		createdAt: '2026-10-09T15:00:00Z',
		raisedOn: '2026-10-09',
		dueOn: '2026-11-08',
		reference: '4F2C9A1B-0002',
		billingMode: 'stripe',
		refundedCents: 0,
		daysOverdue: 0
	},
	{
		id: 'invoice-deposit',
		contractId: 'contract-1',
		status: 'paid',
		amountCents: 60_000,
		currency: 'usd',
		createdAt: '2026-08-20T15:00:00Z',
		raisedOn: '2026-08-20',
		dueOn: '2026-09-19',
		paidOn: '2026-08-24',
		reference: '4F2C9A1B-0001',
		billingMode: 'stripe',
		activePaymentId: 'payment-deposit',
		activePaymentKind: 'stripe',
		refundedCents: 15_000,
		daysOverdue: 0
	}
];
const typicalRoster = TYPICAL_STAFF.map((member) => ({
	...member,
	roles: [...member.roles],
	workStateReportedAt: '2026-09-08T15:00:00Z'
}));
const typicalNotes = [
	'Talked through comfort measures and what to pack.',
	'',
	// Two sentences, as a Doula writes up a Visit that covered something
	// (the demo Practice's are this long, #1999): longer than `--measure`,
	// the Notes cell's cap, so the floor below is measured on the widest
	// the column gets rather than on one short line.
	'Walked through the birth plan draft with her partner. She wants the lights low and music on in early labor, and her partner calls us when the contractions are five minutes apart.',
	'Feeding is going well.',
	''
];

export const typical: RouteVariant<RouteParameters> = {
	name: 'The Staff-side Engagement detail hub, with typical pilot data',
	typical: true,
	readyText: typicalClientName,
	props: { data: { ...typicalDetail, session } },
	respond: (path) => {
		if (/\/engagements\/engagement-1$/.test(path)) return jsonResponse(typicalDetail);
		if (path.endsWith('/visit-assignees')) {
			return jsonResponse({
				items: TYPICAL_DOULAS.map(({ staffId, name, employmentType }) => ({
					staffId,
					name,
					employmentType,
					nameable: employmentType === 'employee'
				}))
			});
		}
		if (path.endsWith('/staff')) return jsonResponse({ members: typicalRoster, invitations: { items: [] } });
		// #2004: both Plans started, the Birth Plan answered in full and
		// read by the Client, and a short thread, as a Client weeks into
		// care has.
		if (path.endsWith('/plans/birth_plan')) {
			return jsonResponse({
				...birthPlan,
				answers: {
					'bp-atmosphere': 'Low lights and music.',
					'bp-support': 'Her partner',
					'bp-feeding': 'Breast'
				},
				clientAcknowledgedAt: '2026-10-03T15:00:00Z'
			});
		}
		if (path.endsWith('/plans/care_plan')) return jsonResponse(carePlan);
		if (path.includes('/messages')) {
			// Newest first, as the BFF sends a page.
			return jsonResponse({
				items: [
					{
						messageId: 'message-2',
						senderType: 'client',
						senderId: 'client-1',
						senderName: TYPICAL_CLIENTS[0]!.givenName,
						body: 'Thursday works. See you at 2.',
						createdAt: '2026-10-08T15:30:00Z'
					},
					{
						messageId: 'message-1',
						senderType: 'staff',
						senderId: typicalDoula.staffId,
						senderName: typicalDoula.name,
						body: 'Does Thursday at 2 still work for the visit?',
						createdAt: '2026-10-08T14:00:00Z'
					}
				],
				hasMore: false
			});
		}
		if (path.includes('/visits')) {
			return jsonResponse({
				items: typicalNotes.map((notes, index) => ({
					visitId: `visit-${index + 1}`,
					staffId: typicalDoula.staffId,
					staffName: typicalDoula.name,
					createdAt: `2026-09-${String(1 + index * 6).padStart(2, '0')}T15:00:00Z`,
					scheduledAt: `2026-09-${String(2 + index * 6).padStart(2, '0')}T15:00:00Z`,
					...(notes && { notes }),
					type: index === 3 ? 'postpartum' : 'prenatal'
				})),
				hasMore: false
			});
		}
		// #2146: the ledger's "Invoice raised", and the deposit before it,
		// paid: the Invoices table this screen measures, with every column
		// an Owner reads filled on the paid row.
		if (path.endsWith('/contract/invoices')) return jsonResponse({ items: typicalInvoices, hasMore: false });
		if (path.includes('/activity')) {
			return jsonResponse({
				items: ['visit_logged', 'invoice_raised', 'contract_sent', 'visit_logged', 'visit_scheduled'].map(
					(action, index) => ({
						subjectKind: 'engagement',
						subjectId: 'engagement-1',
						action,
						actorKind: 'staff',
						actorName: index % 2 === 0 ? typicalDoula.name : TYPICAL_STAFF[0]!.name,
						createdAt: `2026-10-${String(10 - index).padStart(2, '0')}T15:00:00Z`
					})
				),
				hasMore: false
			});
		}
		return fixture.respond!(path);
	}
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Staff-side Engagement detail hub, as an Owner who is also a Doula',
	component: Page,
	params: { practiceId: 'practice-1', engagementId: 'engagement-1' },
	url: 'https://example.test/practices/practice-1/engagements/engagement-1',
	// The Engagement arrives as a prop now, from +page.ts's load (#695),
	// rather than through the cascade below. The route's `respond` still
	// answers it, harmlessly, for the same reason the URL builders stayed
	// exported: nothing else has to change if it ever moves back.
	// #268: the page reads `data.session` to decide which Visit controls a
	// reader can complete, so the fixture carries the Membership
	// practices/[practiceId]/+layout.ts merges in -- an Owner who is also a
	// Doula, the role that sees every control this route draws.
	props: { data: { ...detail, session } },
	respond: (path) => {
		if (/\/engagements\/engagement-1$/.test(path)) return jsonResponse(detail);
		if (/\/clients\/client-1$/.test(path)) return jsonResponse(clientRecord);
		if (path.endsWith('/visit-assignees')) return jsonResponse({ items: visitAssignees });
		// #1093's panel: a live window with a primary and a narrowed
		// backup, one gap nobody is covering and one somebody is. Both
		// gap states are here on purpose -- the section renders them
		// differently, and a fixture holding only the covered one would
		// measure a screen with no hole in it.
		if (path.endsWith('/on-call')) return jsonResponse(onCallPanel);
		if (path.includes('/visits')) {
			return jsonResponse({
				items: [
					{
						visitId: 'visit-1',
						staffId: 'staff-1',
						staffName: 'Anne-Marie Ochieng-Whitfield',
						createdAt: '2026-08-05T00:00:00Z',
						scheduledAt: '2027-03-15T14:30:00Z',
						notes:
							'She asked a lot of questions about pain management options and wants to keep her options open rather than commit to an unmedicated birth ahead of time. Her partner is nervous about the hospital transfer distance and would like a practice run of the drive before the due date. Follow up next visit on the birth plan draft she is writing.',
						// #281: this fixture's two rows carry the two Visit types the
						// screen renders with a different label -- 'birth' shares
						// postpartum's label-lookup path, so it earns no third row.
						type: 'postpartum'
					},
					{
						visitId: 'visit-2',
						staffId: 'staff-2',
						staffName: 'Jordan Reyes',
						createdAt: '2026-08-06T00:00:00Z',
						type: 'prenatal'
					}
				],
				hasMore: false
			});
		}
		if (path.includes('/messages')) return jsonResponse({ items: [], hasMore: false });
		// #2004: one Plan of each state the read view draws -- a Birth Plan
		// started and not yet read by the Client (its rows and its print
		// link), and a Care Plan not started (its empty state).
		if (path.endsWith('/plans/birth_plan')) return jsonResponse(birthPlan);
		if (path.includes('/plans/')) return jsonResponse('not found', 404);
		if (path.endsWith('/contract/invoices')) return jsonResponse({ items: [] });
		// #271: read alongside the Invoice list on every mount, whether or
		// not this Engagement currently has a Contract to bill against.
		if (path.endsWith('/payments/billing-mode')) return jsonResponse({ billingMode: 'stripe' });
		// #1280: read the same way, so InvoiceSection's own todayIsoDate has
		// a real zone rather than mounting behind the "still loading" gap.
		if (path.endsWith('/timezone')) return jsonResponse({ timezone: 'America/New_York' });
		if (path.endsWith('/contract')) return jsonResponse(contract);
		// #1885: read only while there is no Contract, so this base never
		// asks; answered for the specs whose `/contract` is a 404, with both
		// kinds set, the state that leaves "Create Draft Contract" live.
		if (path.endsWith('/rates')) {
			return jsonResponse({
				rates: [
					{ kind: 'birth', amountCents: 150_000 },
					{ kind: 'postpartum', amountCents: 90_000 }
				]
			});
		}
		if (path.endsWith('/offers')) return jsonResponse({ items: [] });
		if (path.endsWith('/staff')) return jsonResponse({ members: roster, invitations: { items: [] } });
		// #486: the same ledger treatment, last in this page's own sections.
		if (path.includes('/activity')) {
			return jsonResponse({
				items: [
					{
						subjectKind: 'engagement',
						subjectId: 'engagement-1',
						action: 'visit_logged',
						actorKind: 'staff',
						actorName: clientName,
						createdAt: '2026-08-05T00:00:00Z'
					},
					// #1423: the widest What-column sentence this ledger can
					// hold -- a birth outcome corrected, both sides dated, each
					// side in ADR-0015's own long wording.
					{
						action: 'birth_outcome_recorded',
						actorKind: 'staff',
						actorName: clientName,
						diff: {
							birthOutcomeBefore: 'unknown',
							birthOutcomeAfter: 'loss',
							pregnancyEndedOnBefore: '2027-03-04',
							pregnancyEndedOnAfter: '2027-03-05'
						},
						createdAt: '2026-08-06T00:00:00Z'
					}
				],
				hasMore: false
			});
		}
		throw new Error(`engagements/[engagementId] fixture: unmatched fetch path ${path}`);
	},
	readyText: clientName,
	variants: [asDoula, asContractor, withNoRate, typical]
};
