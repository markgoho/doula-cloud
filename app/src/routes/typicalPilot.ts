/*
 * Typical pilot data (#1981): what a 14-doula agency really holds, for the
 * one question a route fixture asks of typical content rather than hostile
 * content -- where a table route moves from its records to its table.
 * `routeFixture.ts` says why that question takes typical content.
 *
 * Every value has a source, so nobody has to guess whether it is typical:
 *
 * - The Practice and its Staff are Rooted Birth Collective, the World the
 *   simulation map sized to the pilot's 14-doula agency
 *   (`docs/simulation/worlds/rooted-birth-collective.md`): fifteen Staff,
 *   fourteen of them Doulas, ten employees and four contractors, one Admin
 *   who is not a Doula, every one in New York but Yolanda Prieto (PA).
 * - The Clients are that World's own: Camille Boyd is Rooted's Client
 *   Persona, Hannah Sorensen and Nadia Haddad are the two Client Personas
 *   (Maya's, at Okonkwo), and the rest are the anonymous tail's name pool
 *   in `app/e2e/simulation/world.ts` (`TAIL_GIVEN_NAMES` x
 *   `TAIL_FAMILY_NAMES`).
 * - An address is the person's name at a common free mail provider,
 *   `given.family@gmail.com` -- the shape #1945's own measurement used
 *   for "five ordinary Clients".
 * - Statuses, kinds and counts are the domain's own words (GLOSSARY.md)
 *   and the World's own numbers: 58 live Engagements, 32 birth and 26
 *   postpartum (`docs/simulation/calendar.md`).
 *
 * None of this is a fixture's hostile content, and none of it replaces
 * any: a typical variant sits beside its route's hostile screens, never
 * in place of them.
 */

export const TYPICAL_PRACTICE_NAME = 'Rooted Birth Collective';

export interface TypicalStaff {
	readonly staffId: string;
	readonly name: string;
	readonly email: string;
	readonly roles: readonly string[];
	readonly employmentType: 'employee' | 'contractor';
	readonly workState: string;
}

function address(name: string): string {
	return `${name.toLowerCase().replace(' ', '.')}@gmail.com`;
}

function staff(
	index: number,
	name: string,
	roles: readonly string[],
	employmentType: 'employee' | 'contractor',
	workState = 'NY'
): TypicalStaff {
	return { staffId: `staff-${index}`, name, email: address(name), roles, employmentType, workState };
}

/*
 * The fifteen, in the World's own order: the four Personas first, then
 * the eleven Extras. `staff-1` is the Owner, as in every other fixture.
 */
export const TYPICAL_STAFF: readonly TypicalStaff[] = [
	staff(1, 'Renata Alvarez', ['owner', 'doula'], 'employee'),
	staff(2, 'Dee Whitlock', ['admin'], 'employee'),
	staff(3, 'Priya Raman', ['doula'], 'employee'),
	staff(4, 'Lena Vasquez', ['doula'], 'contractor'),
	staff(5, 'Joss Adeyemi', ['doula'], 'employee'),
	staff(6, 'Marisol Terrazas', ['doula'], 'employee'),
	staff(7, 'Bethany Kroll', ['doula'], 'employee'),
	staff(8, 'Aditi Sundaram', ['doula'], 'employee'),
	staff(9, 'Charlene Boateng', ['doula'], 'employee'),
	staff(10, 'Rowan Petrosyan', ['doula'], 'employee'),
	staff(11, 'Delia Marchetti', ['doula'], 'employee'),
	staff(12, 'Kimiko Nakashima', ['doula'], 'employee'),
	staff(13, 'Fern Okada', ['doula'], 'contractor'),
	staff(14, 'Yolanda Prieto', ['doula'], 'contractor', 'PA'),
	staff(15, 'Trish Halvorsen', ['doula'], 'contractor')
];

/**
The fourteen who hold the Doula role -- everyone but Dee Whitlock.
*/
export const TYPICAL_DOULAS: readonly TypicalStaff[] = TYPICAL_STAFF.filter((member) =>
	member.roles.includes('doula')
);

export interface TypicalClient {
	readonly clientId: string;
	readonly givenName: string;
	readonly familyName: string;
	readonly name: string;
	readonly email: string;
}

function client(index: number, givenName: string, familyName: string): TypicalClient {
	const name = `${givenName} ${familyName}`;
	return { clientId: `client-${index}`, givenName, familyName, name, email: address(name) };
}

export const TYPICAL_CLIENTS: readonly TypicalClient[] = [
	client(1, 'Camille', 'Boyd'),
	client(2, 'Hannah', 'Sorensen'),
	client(3, 'Nadia', 'Haddad'),
	client(4, 'Morgan', 'Bishop'),
	client(5, 'Jamie', 'Calloway'),
	client(6, 'Casey', 'Dunmore'),
	client(7, 'Skyler', 'Estrada'),
	client(8, 'Reese', 'Falk'),
	client(9, 'Avery', 'Grier'),
	client(10, 'Quinn', 'Calloway'),
	client(11, 'Harper', 'Dunmore'),
	client(12, 'Morgan', 'Estrada')
];

/**
The typical Staff member or Client at `index`, wrapping round the list.
*/
export function nth<T>(list: readonly T[], index: number): T {
	return list[index % list.length]!;
}
