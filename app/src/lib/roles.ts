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

/**
 * Whether the session's caller holds the 'owner' role.
 */
export function isOwner(session: Pick<RoleSession, 'roles'>): boolean {
	return session.roles.includes('owner');
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
