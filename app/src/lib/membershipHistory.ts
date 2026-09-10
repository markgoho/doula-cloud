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
			// What she arrived as, both halves of it (ADR-0008) -- and the
			// employment type in parentheses rather than appended to the
			// role list, because the two are different axes: "Owner, Admin,
			// Doula, Employee" reads as a fourth role.
			const roles = rolesLabel(change.roles ?? []);
			const employmentType = employmentTypeLabel(change.employmentType ?? '');
			if (!roles) return `Joined as ${employmentType}`;
			if (!employmentType) return `Joined as ${roles}`;
			return `Joined as ${roles} (${employmentType})`;
		}
		case 'roles_changed': {
			const from = rolesLabel(change.previousRoles ?? []);
			const to = rolesLabel(change.roles ?? []);
			return from && to ? `Roles changed from ${from} to ${to}` : fallback(change.action);
		}
		case 'employment_type_changed': {
			const from = employmentTypeLabel(change.previousEmploymentType ?? '');
			const to = employmentTypeLabel(change.employmentType ?? '');
			return from && to
				? `Employment type changed from ${from} to ${to}`
				: fallback(change.action);
		}
		case 'removed': {
			return 'Removed from this practice';
		}
		case 'sessions_ended': {
			return 'Signed out of every device';
		}
		default: {
			return fallback(change.action);
		}
	}
}

/**
 * What an entry reads as when its own sentence cannot be built: an
 * action this build has no words for, or one whose before/after facts
 * did not arrive.
 *
 * `describeActivityAction` is the activity ledger's own generic
 * action-to-phrase function, so "roles_changed" still reads "Roles
 * changed" -- true, if less than the full sentence. Better than a
 * sentence with a hole in it: printing "Roles changed from  to " would
 * tell a reader two facts, and both of them would be nothing. The same
 * leniency `roleLabel` applies to a role it has no word for, applied one
 * level up.
 */
function fallback(action: string): string {
	return describeActivityAction(action);
}
