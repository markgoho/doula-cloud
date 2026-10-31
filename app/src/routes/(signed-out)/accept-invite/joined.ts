import { employmentTypeLabel, rolesLabel } from '#lib/roles.js';

function withArticle(word: string): string {
	return `${/^[aeiou]/i.test(word) ? 'an' : 'a'} ${word}`;
}

/**
"an Admin", "a Doula", "an Owner, Admin and Doula".
*/
export function asRoles(roles: readonly string[]): string {
	const labels = rolesLabel(roles).split(', ');
	const last = labels.pop() ?? '';
	return withArticle(labels.length > 0 ? `${labels.join(', ')} and ${last}` : last);
}

export interface Joined {
	practiceName: string;
	roles: readonly string[];
	employmentType: string;
	/**
	She already belonged to another Practice, which this left alone.
	*/
	hasOtherPractices: boolean;
	/**
	The work state her account already held, when it carried over.
	*/
	workState?: string;
}

/**
 * What the screen she lands on says after accepting (#2239): that she
 * joined, which Practice, and as what -- the roles and, in words, the
 * employment type.
 */
export function joinedSentence(joined: Joined): string {
	// An Invitation always carries a role and an employment type
	// (parseMembership refuses one without).
	const employment = withArticle(employmentTypeLabel(joined.employmentType).toLowerCase());
	const sentences = [`You've joined ${joined.practiceName}.`, `You're ${asRoles(joined.roles)}, as ${employment}.`];
	if (joined.hasOtherPractices) sentences.push('Your other Practices are unchanged.');
	if (joined.workState) sentences.push(`You still work from ${joined.workState}.`);
	return sentences.join(' ');
}
