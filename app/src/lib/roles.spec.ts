import { describe, it, expect } from 'vitest';
import {
	isOwner,
	isOwnerOrAdmin,
	isAmbientContractor,
	ROLE_LABELS,
	EMPLOYMENT_TYPE_LABELS,
	roleLabel,
	rolesLabel,
	employmentTypeLabel,
	type RoleSession
} from './roles';

function session(roles: string[], isContractor = false): RoleSession {
	return { roles, isContractor };
}

describe('isOwner', () => {
	it('is true for an owner', () => {
		expect(isOwner(session(['owner']))).toBe(true);
	});

	it('is false for an admin', () => {
		expect(isOwner(session(['admin']))).toBe(false);
	});

	it('is false for a plain doula', () => {
		expect(isOwner(session(['doula']))).toBe(false);
	});
});

describe('isOwnerOrAdmin', () => {
	it('is true for an owner', () => {
		expect(isOwnerOrAdmin(session(['owner']))).toBe(true);
	});

	it('is true for an admin', () => {
		expect(isOwnerOrAdmin(session(['admin']))).toBe(true);
	});

	it('is false for a plain doula', () => {
		expect(isOwnerOrAdmin(session(['doula']))).toBe(false);
	});

	it('is false with no roles at all', () => {
		expect(isOwnerOrAdmin(session([]))).toBe(false);
	});
});

describe('isAmbientContractor', () => {
	it('is true for a contractor doula holding neither owner nor admin', () => {
		expect(isAmbientContractor(session(['doula'], true))).toBe(true);
	});

	it('is false for an employee doula', () => {
		expect(isAmbientContractor(session(['doula'], false))).toBe(false);
	});

	// ADR-0017's solo-Practice case: an Owner whose own Membership also
	// carries contractor employment type keeps the ambient reach her role
	// grants -- the same carve-out staffauth.Reader.IsAmbientContractor
	// makes on the api tier.
	it('is false for an owner who also holds contractor employment type', () => {
		expect(isAmbientContractor(session(['owner'], true))).toBe(false);
	});

	it('is false for an admin who also holds contractor employment type', () => {
		expect(isAmbientContractor(session(['admin', 'doula'], true))).toBe(false);
	});

	it('is true for a contractor with no roles at all -- holding neither owner nor admin', () => {
		expect(isAmbientContractor(session([], true))).toBe(true);
	});
});

describe('ROLE_LABELS', () => {
	it('names all three stored role values, owner first', () => {
		expect(ROLE_LABELS).toEqual([
			{ value: 'owner', label: 'Owner' },
			{ value: 'admin', label: 'Admin' },
			{ value: 'doula', label: 'Doula' }
		]);
	});
});

describe('EMPLOYMENT_TYPE_LABELS', () => {
	it('names both stored employment-type values', () => {
		expect(EMPLOYMENT_TYPE_LABELS).toEqual([
			{ value: 'employee', label: 'Employee' },
			{ value: 'contractor', label: 'Contractor' }
		]);
	});
});

describe('roleLabel', () => {
	it.each([
		['owner', 'Owner'],
		['admin', 'Admin'],
		['doula', 'Doula']
	])('writes %s as "%s"', (role, expected) => {
		expect(roleLabel(role)).toBe(expected);
	});

	// A role the BFF grows before this map catches up still prints, rather
	// than throwing or vanishing from a person's own list of what she is.
	it('capitalizes an unrecognized role rather than throwing', () => {
		expect(roleLabel('midwife')).toBe('Midwife');
	});
});

describe('rolesLabel', () => {
	it.each([
		[['owner', 'admin'], 'Owner, Admin'],
		[['owner', 'admin', 'doula'], 'Owner, Admin, Doula'],
		[['doula'], 'Doula'],
		[['midwife'], 'Midwife'],
		[[], '']
	])('writes %s as "%s"', (roles, expected) => {
		expect(rolesLabel(roles)).toBe(expected);
	});
});

describe('employmentTypeLabel', () => {
	it.each([
		['employee', 'Employee'],
		['contractor', 'Contractor']
	])('writes %s as "%s"', (employmentType, expected) => {
		expect(employmentTypeLabel(employmentType)).toBe(expected);
	});

	it('capitalizes an unrecognized employment type rather than throwing', () => {
		expect(employmentTypeLabel('apprentice')).toBe('Apprentice');
	});
});
