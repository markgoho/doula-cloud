/**
 * ADR-0008's role predicates, read off the Reader `practices/[practiceId]/
 * +layout.ts` resolves once per navigation -- the app-side mirror of
 * `staffauth.Reader.IsOwnerOrAdmin`/`IsAmbientContractor` (api/internal/
 * staffauth/reader.go). This module is drawing only, never a gate: the BFF
 * endpoint is what actually refuses a write (ADR-0006/ADR-0008), and every
 * caller here is deciding what to show, not what to allow.
 */

export interface RoleSession {
	roles: string[];
	isContractor: boolean;
}

/** The two stored `employment_type` values, named once here rather than
 * spelled out as a union by every screen that edits a Membership. */
export type EmploymentType = 'employee' | 'contractor';

/** A stored enum value paired with the word shown for it -- the shape
 * `RadioGroup` and the Roles checkboxes both take their options in. */
export interface LabeledValue<Value extends string = string> {
	value: Value;
	label: string;
}

/**
 * The three stored role values paired with the words the product shows for
 * them -- the one place they are named, so a screen never spells out
 * `owner`, `admin` or `doula` itself (#290). `MembershipFields` renders
 * these as its Roles checkboxes; the signup screen reads the same array to
 * name the roles it grants (#290); the Staff roster and the pending
 * Invitation list read it through `rolesLabel` (#262).
 */
export const ROLE_LABELS: readonly LabeledValue[] = [
	{ value: 'owner', label: 'Owner' },
	{ value: 'admin', label: 'Admin' },
	{ value: 'doula', label: 'Doula' }
];

/** The two stored `employment_type` values and the words shown for them --
 * what a person *is to the business*, as against what she does, which is
 * her roles. `MembershipFields` renders these as its Employment type
 * radios; the roster and Invitation list read them through
 * `employmentTypeLabel` (#262). */
export const EMPLOYMENT_TYPE_LABELS: readonly LabeledValue<EmploymentType>[] = [
	{ value: 'employee', label: 'Employee' },
	{ value: 'contractor', label: 'Contractor' }
];

/**
 * The display word for one stored `practice_role` value.
 *
 * **Unknown values are capitalized and printed, never thrown on.** A role
 * the BFF grows before this map catches up still appears in a person's own
 * list of what she is, rather than vanishing from her row or taking the
 * screen down with it. That is the opposite of `clientRegister.ts`, whose
 * lookups throw on an unrecognized value, and the difference is
 * deliberate: ADR-0005 says a Client must never meet a domain word, so
 * quietly printing a raw enum there would be the exact defect that module
 * exists to prevent. Staff already speak these words -- an Owner reading
 * `Midwife` for a role this build has not labeled is informed, not
 * confused.
 */
export function roleLabel(role: string): string {
	return labelFor(ROLE_LABELS, role);
}

/** A whole `roles` array as one readable string -- `Owner, Admin, Doula`
 * -- in the order the Membership carries them. Lenient on an unknown
 * value, for the reason `roleLabel` gives. */
export function rolesLabel(roles: readonly string[]): string {
	return roles.map((role) => roleLabel(role)).join(', ');
}

/** The display word for one stored `employment_type` value. Lenient on an
 * unknown value, for the reason `roleLabel` gives. */
export function employmentTypeLabel(employmentType: string): string {
	return labelFor(EMPLOYMENT_TYPE_LABELS, employmentType);
}

/**
 * The lenient lookup both helpers above are: the labeled word if the map
 * has one, otherwise the stored value with its first letter raised.
 */
function labelFor(options: readonly LabeledValue[], value: string): string {
	return options.find((option) => option.value === value)?.label ?? value.charAt(0).toLocaleUpperCase() + value.slice(1);
}

/**
 * Whether the session's caller holds the 'owner' role.
 */
export function isOwner(session: Pick<RoleSession, 'roles'>): boolean {
	return session.roles.includes('owner');
}

/**
 * Whether the session's caller holds the 'doula' role -- the role that
 * puts a person on a birth, and so the one that decides whether she has a
 * Visit of her own to log (#268). An Owner or Admin who does not hold it
 * schedules other people's Visits and has none of her own.
 */
export function isDoula(session: Pick<RoleSession, 'roles'>): boolean {
	return session.roles.includes('doula');
}

/**
 * Whether the session's caller holds the 'owner' or 'admin' role.
 */
export function isOwnerOrAdmin(session: Pick<RoleSession, 'roles'>): boolean {
	return isOwner(session) || session.roles.includes('admin');
}

/**
 * Whether the session's caller is a plain contractor Doula -- employment
 * type contractor, holding neither the owner nor admin role -- the
 * population ADR-0008 confines to what she is attached to, rather than
 * granting the Practice-wide ambient reach an owner, admin, or employee
 * Doula all hold.
 */
export function isAmbientContractor(session: RoleSession): boolean {
	return session.isContractor && !isOwnerOrAdmin(session);
}
