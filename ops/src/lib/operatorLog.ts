import { refusal, type Outcome } from './act.ts';
import { credits } from './foundingGrant.ts';
import { longDate } from './dates.ts';
import { money } from './money.ts';

export type LogSubject = { kind: 'practice' | 'staff' | 'feedback'; id: string; name: string | null; deleted: boolean };

export type LogRow = {
	id: string;
	at: string;
	operator: string;
	action: string;
	subject: LogSubject | null;
	details: Record<string, unknown>;
	requestedBy?: { id: string; name: string | null };
};

export type LogPage = { items: LogRow[]; nextCursor?: string; hasMore: boolean };

/**
Each action operator_acts can hold, as the Operator log names it. A new
action in the table's CHECK needs its words here.
*/
export const ACTION_LABELS: Record<string, string> = {
	sign_in: 'Signed in',
	sign_out: 'Signed out',
	step_up_refused: 'Authenticator code refused',
	refund: 'Refund',
	founding_grant: 'Founding grant',
	second_factor_cleared: 'Two-step sign-in reset',
	owner_contact_read: 'Read Owners’ contacts',
	feedback_read: 'Read a piece of Feedback',
	pilot_address_added: 'Added to the pilot allowlist',
	pilot_gate_opened: 'Opened the pilot gate',
	legal_version_published: 'Published a legal version'
};

const LEGAL_DOCUMENTS: Record<string, string> = { terms: 'The Terms of Service', privacy: 'The Privacy Policy' };

const REFUSAL_REASONS: Record<string, string> = {
	no_sign_in: 'No sign-in came with the code',
	not_verified: 'The sign-in could not be verified',
	other_operator: "The sign-in was another operator's",
	no_authenticator_code: 'The sign-in had no authenticator-app code',
	stale_sign_in: 'The sign-in was more than 5 minutes old'
};

const CONTACT_LISTS: Record<string, string> = { practices: 'The Practices list', deleted: 'The Deleted Practices list' };

function count(n: number, one: string, many: string): string {
	return `${n} ${n === 1 ? one : many}`;
}

function refundDetail(row: LogRow): string {
	const d = row.details as { requestedByStaffId: string; requestedOn: string; credits: number; amountCents: number; taxCents: number };
	const owner = row.requestedBy?.name ?? `a Staff member no longer on file (${d.requestedByStaffId})`;
	return `${credits(d.credits)}, ${money(d.amountCents)} back, ${money(d.taxCents)} of it tax. Asked for by ${owner} on ${longDate(d.requestedOn)}`;
}

/**
What a row's details say, in words, or nothing for a row whose kind says it all.
*/
export function logDetail(row: LogRow): string | undefined {
	switch (row.action) {
		case 'step_up_refused': {
			const reason = String(row.details.reason);
			return REFUSAL_REASONS[reason] ?? reason;
		}
		case 'refund': {
			return refundDetail(row);
		}
		case 'founding_grant': {
			const d = row.details as { staffCount: number; credits: number };
			return `${credits(d.credits)} for ${d.staffCount} Staff`;
		}
		case 'legal_version_published': {
			const d = row.details as { document: string; version: string; material: boolean };
			const takes = d.document === 'terms' ? 'take' : 'takes';
			return `${LEGAL_DOCUMENTS[d.document]} that ${takes} effect ${longDate(d.version)}, ${d.material ? 'material' : 'not material'}`;
		}
		case 'pilot_address_added': {
			return String(row.details.address);
		}
		case 'owner_contact_read': {
			const d = row.details as { list: string; practiceIds: string[] };
			return `${CONTACT_LISTS[d.list] ?? d.list}, ${count(d.practiceIds.length, 'Practice', 'Practices')}`;
		}
	}
	return undefined;
}

/**
The Practice or Staff member a row acted on, named, or by id once no longer on file.
*/
export function subjectFact(subject: LogSubject): { label: string; value: string } {
	const label = subject.kind === 'practice' ? 'Practice' : 'Staff member';
	if (subject.name === null) return { label, value: `No longer on file (${subject.id})` };
	return { label, value: subject.deleted ? `${subject.name} (deleted)` : subject.name };
}

/**
One page of the Operator log, newest first, after cursor.
*/
export async function readOperatorLog(cursor: string | undefined, fetcher: typeof fetch = fetch): Promise<Outcome<LogPage>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher('/api/operator-log' + query, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
