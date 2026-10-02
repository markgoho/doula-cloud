/*
 * PROTOTYPE -- throwaway, wayfinder ticket #1496. The sample content and
 * the words of the route. Each sentence names the ticket that decided it,
 * so that a changed reaction has one place to change.
 */

export type Content = 'long' | 'typical';
export type Owner = 'solo' | 'agency';

export interface Sample {
	firstName: string;
	lastName: string;
	practiceName: string;
	workState: string;
	timezone: string;
	email: string;
	password: string;
	client: { givenName: string; familyName: string; preferredName: string };
	dueDate: string;
}

// ADR-0024: the 320px check is done with a real Practice's content, so
// the long sample is the default.
export const samples: Record<Content, Sample> = {
	long: {
		firstName: 'Maria-Guadalupe',
		lastName: 'Hernandez-Castellanos',
		practiceName: 'Genesee Valley Birth and Postpartum Doula Collective of Greater Rochester',
		workState: 'District of Columbia',
		timezone: 'America/Phoenix',
		email: 'maria-guadalupe.hernandez-castellanos@geneseevalleydoulacollective.example',
		password: 'correct horse battery staple',
		client: { givenName: 'Oluwaseun-Adebayo', familyName: 'Fitzgerald-Montgomery', preferredName: '' },
		dueDate: '2027-03-14'
	},
	typical: {
		firstName: 'Tasha',
		lastName: 'Bell',
		practiceName: '',
		workState: 'New York',
		timezone: 'America/New_York',
		email: 'tasha@example.com',
		password: 'correct horse battery staple',
		client: { givenName: 'Ana', familyName: 'Reyes', preferredName: '' },
		dueDate: '2027-03-14'
	}
};

// The employee Doulas at an agency (#1596), without the Owner herself.
// A contractor Doula and a person with a pending Invitation are never in
// this list. Thirteen and the Owner make the pilot agency's fourteen.
export const agencyDoulas: Record<Content, string[]> = {
	long: [
		'Anneliese Vandenberghe-Okonkwo',
		'Bernadette Christodoulopoulos',
		'Chidinma Nwachukwu-Abernathy',
		'Dominique Saint-Barthélemy',
		'Evangelina Montgomery-Whitfield',
		'Francesca Di Bartolomeo',
		'Guinevere Featherstonehaugh',
		'Henrietta Wojciechowski',
		'Isabella Papadimitriou-Lang',
		'Josephine Throckmorton',
		'Katarzyna Brzezinska-Murphy',
		'Leopoldine Schwarzenberger',
		'Magdalena Villanueva-Ruiz'
	],
	typical: [
		'Amy Cole',
		'Bea Diaz',
		'Cara Evans',
		'Dee Fox',
		'Eve Grant',
		'Fay Hill',
		'Gia Ives',
		'Hana Jones',
		'Ida King',
		'Joy Lane',
		'Kim Moss',
		'Lena Nash',
		'Mia Ortiz'
	]
};

export const words = {
	// #1536, second acceptance criterion: the hint says two things.
	practiceNameHint:
		'If you leave this empty, your Practice takes your own name. Clients see this name on a Contract, in the portal and on a card statement.',
	// #1538 and #1539: the two rules, stated before she types.
	passwordHint: 'Must be 15 characters or more, and not a commonly used password',
	// #290, #878: the role notice, as built.
	roleNotice:
		"This account will hold every role — Owner, Admin and Doula — because a Practice's founder is usually its only Doula and needs to hold their own Visits. Anyone you invite later holds only the roles you choose for them. Roles are shown and changed on your Practice's staff roster.",
	// #1515 decision 4, #1599.
	emptyPractice:
		"Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice.",
	// #1611: as built, and still true when the name is the only question.
	nameHint: 'Only the given name is needed to save the record. The rest can be added at any time.'
};

// #1612. "Welcome credits" only where the balance is the signup bonus
// alone; the prototype never grants or sells a Credit, so the balance is
// the bonus until one is used.
export function emptyPracticeCredits(balance: number): string {
	return `Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has ${balance} Welcome credits.`;
}

// #1612, the Start work form. As #1612 wrote it: "Credits", not "Welcome
// credits", although the balance here is still the signup bonus alone.
export function startWorkCredits(name: string, balance: number): string {
	return `Starting work with ${name} uses 1 Credit. This Practice has ${balance} Credits. After this, it has ${balance - 1}.`;
}

// #1539: a list shipped to the browser stands in for the real check.
export const COMMON_PASSWORDS = new Set(['passwordpassword', '123456789012345', 'qwertyuiopasdfg']);

export const PASSWORD_MIN = 15;
