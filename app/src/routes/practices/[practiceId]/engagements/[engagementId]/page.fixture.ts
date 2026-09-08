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
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../routeFixture.js';
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

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Staff-side Engagement detail hub',
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
	props: {
		data: {
			...detail,
			session: {
				practiceId: 'practice-1',
				practiceName: 'Riverside Doula Collective',
				roles: ['owner', 'doula'],
				isContractor: false
			}
		}
	},
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
		if (path.endsWith('/contract')) return jsonResponse('not found', 404);
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
	readyText: clientName
};
