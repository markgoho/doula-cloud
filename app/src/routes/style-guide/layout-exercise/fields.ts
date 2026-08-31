/*
 * The exercise's content, shared by START, FINISHED and the marking
 * spec so all three are given the same thing to hold (#534).
 *
 * Hostile rather than polite, per ADR-0025 and #537: the last value is
 * the exact URL #530 measured on a real screen. A polite fixture is how
 * a broken card goes green.
 */
export interface Field {
	label: string;
	value: string;
}

export const exerciseFields: readonly Field[] = [
	{ label: 'Client', value: 'Anne-Marie Ochieng-Whitfield' },
	{ label: 'Due date', value: '14 September 2027' },
	{ label: 'Address', value: '128 Meadowbrook Lane, Apartment 4B, Rochester, NY 14620' },
	{
		label: 'Referral',
		value:
			'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
	}
];
