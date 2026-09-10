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
 * double-barrelled name; the Contract's merge-field values and a Visit's
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
 * InvoiceSection's Owner link and her own Visit default; a contractor
 * Doula loses what `asDoula` has plus the Contract's PDF download and the
 * birth-outcome control, so she is a strict subset of that variant rather
 * than a third screen. A strict subset realizes nothing the sweep has
 * already measured (svelte-tests.md, ADR-0025).
 */
import type { Contract } from '#lib/contract.js';
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

const clientName = 'Anne-Marie Ochieng-Whitfield';

/*
 * A roster the size of the pilot's own agency -- fourteen Doulas, plus a
 * bookkeeper who holds no Doula role and so never belongs in a Visit
 * picker. The names are long and double-barrelled on purpose: the two
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

export const detail = {
	engagementId: 'engagement-1',
	clientId: 'client-1',
	clientName,
	status: 'active',
	createdAt: '2026-08-01T00:00:00Z',
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
	pregnancyEndedOn: '2026-08-14'
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
	// never the ambient contractor ADR-0008 confines.
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
 * An employee Doula rather than a contractor one, because she is the
 * wider of the two: `isAmbientContractor` would take away the Contract's
 * PDF download and the birth-outcome controls and add nothing, so the
 * contractor is a strict subset of this branch.
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
	props: { data: { ...detail, session: { ...session, roles: ['doula'], isContractor: false } } }
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
		if (path.endsWith('/visit-assignees')) return jsonResponse({ items: visitAssignees });
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
		if (path.includes('/plans/')) return jsonResponse('not found', 404);
		if (path.endsWith('/contract/invoices')) return jsonResponse({ items: [] });
		// #271: read alongside the Invoice list on every mount, whether or
		// not this Engagement currently has a Contract to bill against.
		if (path.endsWith('/payments/billing-mode')) return jsonResponse({ billingMode: 'stripe' });
		if (path.endsWith('/contract')) return jsonResponse(contract);
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
					}
				],
				hasMore: false
			});
		}
		throw new Error(`engagements/[engagementId] fixture: unmatched fetch path ${path}`);
	},
	readyText: clientName,
	variants: [asDoula]
};
