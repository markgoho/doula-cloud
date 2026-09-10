/**
 * What one entry of a Membership's history says on screen (#872).
 *
 * The BFF sends the stored values (`owner`, `contractor`) and nothing
 * else -- no display words, no assembled sentence -- because `roles.ts`
 * is the one place a `practice_role` or an `employment_type` is given
 * the word a person reads (#262, gated by `roles.usage.spec.ts`), and a
 * second copy of that map on the Go side is exactly what that ticket
 * closed. So the sentence is built here, out of those helpers.
 *
 * Each action gets its own sentence rather than one sentence with blanks
 * in it, for the reason `workStateChangeSentence` gives on the sibling
 * disclosure: printing "changed from — to Admin" for somebody's joining
 * would invent a change she never made.
 */

import { describeActivityAction } from './activityLedger.js';
import { employmentTypeLabel, rolesLabel } from './roles.js';
import type { MembershipChange } from './staff.js';

/**
 * The sentence for one entry -- what happened to this Membership, in the
 * team's own words.
 *
 * An action this build has no sentence for falls back to
 * `describeActivityAction`, the generic write-side-action-to-phrase
 * function the activity ledger already uses, rather than disappearing or
 * throwing. A write site that adds an action before this function
 * catches up still shows the reader something true, which is the same
 * lenience `roleLabel` applies to a role it does not know.
 */
export function membershipChangeSentence(change: MembershipChange): string {
	switch (change.action) {
		case 'joined': {
			// What she arrived as, both halves of it (ADR-0008) -- but
			// joined by a filter rather than a template, so an entry
			// missing one half reads as the half it has instead of
			// trailing a comma into nothing.
			return `Joined as ${[rolesLabel(change.roles ?? []), employmentTypeLabel(change.employmentType ?? '')].filter(Boolean).join(', ')}`;
		}
		case 'roles_changed': {
			return `Roles changed from ${rolesLabel(change.previousRoles ?? [])} to ${rolesLabel(change.roles ?? [])}`;
		}
		case 'employment_type_changed': {
			return `Employment type changed from ${employmentTypeLabel(change.previousEmploymentType ?? '')} to ${employmentTypeLabel(change.employmentType ?? '')}`;
		}
		case 'removed': {
			return 'Removed from this practice';
		}
		case 'sessions_ended': {
			return 'Signed out of every device';
		}
		default: {
			return describeActivityAction(change.action);
		}
	}
}
