/**
 * The 50 US states and the District of Columbia, as the USPS two-letter
 * abbreviation the API stores (`staff.work_state`, migration 00043) and
 * the full name a person actually recognizes in a dropdown.
 *
 * New York sources a sale of remotely accessed software to where the
 * purchaser uses it (TB-ST-128), so a Practice's sales tax is apportioned
 * over where its people work -- which is why this list is asked for at
 * onboarding at all. Anyone working outside the US cannot be recorded and
 * is refused rather than mis-taxed; see #415.
 */
export interface WorkState {
	code: string;
	name: string;
}

export const WORK_STATES: readonly WorkState[] = [
	{ code: 'AL', name: 'Alabama' },
	{ code: 'AK', name: 'Alaska' },
	{ code: 'AZ', name: 'Arizona' },
	{ code: 'AR', name: 'Arkansas' },
	{ code: 'CA', name: 'California' },
	{ code: 'CO', name: 'Colorado' },
	{ code: 'CT', name: 'Connecticut' },
	{ code: 'DE', name: 'Delaware' },
	{ code: 'DC', name: 'District of Columbia' },
	{ code: 'FL', name: 'Florida' },
	{ code: 'GA', name: 'Georgia' },
	{ code: 'HI', name: 'Hawaii' },
	{ code: 'ID', name: 'Idaho' },
	{ code: 'IL', name: 'Illinois' },
	{ code: 'IN', name: 'Indiana' },
	{ code: 'IA', name: 'Iowa' },
	{ code: 'KS', name: 'Kansas' },
	{ code: 'KY', name: 'Kentucky' },
	{ code: 'LA', name: 'Louisiana' },
	{ code: 'ME', name: 'Maine' },
	{ code: 'MD', name: 'Maryland' },
	{ code: 'MA', name: 'Massachusetts' },
	{ code: 'MI', name: 'Michigan' },
	{ code: 'MN', name: 'Minnesota' },
	{ code: 'MS', name: 'Mississippi' },
	{ code: 'MO', name: 'Missouri' },
	{ code: 'MT', name: 'Montana' },
	{ code: 'NE', name: 'Nebraska' },
	{ code: 'NV', name: 'Nevada' },
	{ code: 'NH', name: 'New Hampshire' },
	{ code: 'NJ', name: 'New Jersey' },
	{ code: 'NM', name: 'New Mexico' },
	{ code: 'NY', name: 'New York' },
	{ code: 'NC', name: 'North Carolina' },
	{ code: 'ND', name: 'North Dakota' },
	{ code: 'OH', name: 'Ohio' },
	{ code: 'OK', name: 'Oklahoma' },
	{ code: 'OR', name: 'Oregon' },
	{ code: 'PA', name: 'Pennsylvania' },
	{ code: 'RI', name: 'Rhode Island' },
	{ code: 'SC', name: 'South Carolina' },
	{ code: 'SD', name: 'South Dakota' },
	{ code: 'TN', name: 'Tennessee' },
	{ code: 'TX', name: 'Texas' },
	{ code: 'UT', name: 'Utah' },
	{ code: 'VT', name: 'Vermont' },
	{ code: 'VA', name: 'Virginia' },
	{ code: 'WA', name: 'Washington' },
	{ code: 'WV', name: 'West Virginia' },
	{ code: 'WI', name: 'Wisconsin' },
	{ code: 'WY', name: 'Wyoming' }
];

export const WORK_STATE_NAMES: readonly string[] = WORK_STATES.map((s) => s.name);

/**
 * The USPS code for a full state name, or '' if the name is not one.
 */
export function workStateCode(name: string): string {
	return WORK_STATES.find((s) => s.name === name)?.code ?? '';
}

/**
 * The full state name for a USPS code, or the code itself if it is not
 * one of ours. The fallback is deliberate: the API is the authority on
 * what is stored, so a code this list has not heard of is shown as it
 * came rather than swallowed into a blank -- a wrong-looking value on
 * the screen is how anyone finds out the two lists have drifted apart.
 *
 * The inverse of workStateCode(). Both directions are needed because the
 * dropdown speaks names and the API speaks codes (#437 added this one,
 * for the screens that read a recorded state back out).
 */
export function workStateName(code: string): string {
	return WORK_STATES.find((s) => s.code === code)?.name ?? code;
}

/**
 * The day a work state was last asserted, in the one format every screen
 * that shows it uses -- the roster's "self-reported <date>" column, the
 * account screen's "last confirmed" line, and the read-only value shown
 * to someone accepting a second Invitation.
 *
 * The date carries real weight rather than being decoration: nothing
 * prompts a re-assertion, so how old it is is the only staleness signal
 * the design has (#415). Three screens showing the same fact in three
 * shapes would make it harder to compare, so the format lives here once.
 *
 * The locale is left to the browser (`undefined`), the way every other
 * date in this app is formatted -- we do not know better than the
 * reader's own machine which order she reads a date in.
 */
export function workStateReportedOn(timestamp: string): string {
	return new Date(timestamp).toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}
