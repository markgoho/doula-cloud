import { refusal, type Outcome } from './act.ts';
import type { FormError } from './authRefusal.ts';

export const LOOKUP_EMAIL_FIELD = 'lookup-email';

export type FoundStaff = { staffId: string; name: string; email: string; practiceNames: string[] };

/**
What is missing from the email before doula-ops is asked.
*/
export function lookupErrors(email: string): FormError[] {
	const trimmed = email.trim();
	if (trimmed === '') return [{ message: 'Enter the email address the person signs in with', targetId: LOOKUP_EMAIL_FIELD }];
	const at = trimmed.indexOf('@');
	if (at <= 0 || at === trimmed.length - 1) {
		return [{ message: 'Enter an email address in the correct format, like name@example.com', targetId: LOOKUP_EMAIL_FIELD }];
	}
	return [];
}

/**
Each Staff member who signs in with email. doula-ops records that their contacts were shown.
*/
export async function findStaff(email: string, fetcher: typeof fetch = fetch): Promise<Outcome<FoundStaff[]>> {
	const response = await fetcher(`/api/staff?email=${encodeURIComponent(email.trim())}`, { credentials: 'include' });
	if (!response.ok) return refusal(response, { email: LOOKUP_EMAIL_FIELD });
	const body: { staff: FoundStaff[] } = await response.json();
	return { kind: 'ok', value: body.staff };
}
