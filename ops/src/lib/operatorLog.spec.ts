/* eslint-disable unicorn/no-null -- doula-ops answers null for a field with nothing in it */
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, vi } from 'vitest';
import { ACTION_LABELS, logDetail, readOperatorLog, subjectFact, type LogRow } from './operatorLog.ts';

const byText = (a: string, b: string) => a.localeCompare(b);

const migrations = fileURLToPath(new URL('../../../api/db/migrations/', import.meta.url));

/**
The actions operator_acts' CHECK admits after the newest migration's Up
section that redefines it.
*/
function checkedActions(): string[] {
	let actions: string[] = [];
	const files = readdirSync(migrations)
		.filter((name) => name.endsWith('.sql'))
		.toSorted(byText);
	for (const file of files) {
		const up = readFileSync(path.join(migrations, file), 'utf8').split('-- +goose Down', 1)[0];
		for (const match of up.matchAll(/CHECK \(action IN \(([^)]*)\)\)/g)) {
			actions = match[1].split(',').map((quoted) => quoted.trim().replaceAll("'", ''));
		}
	}
	return actions;
}

function row(overrides: Partial<LogRow>): LogRow {
	return {
		id: 'a-1',
		at: '2026-11-02T14:05:00Z',
		operator: 'Mark Goho',
		action: 'sign_in',
		subject: null,
		details: {},
		...overrides
	};
}

describe('ACTION_LABELS', () => {
	it('says every action operator_acts can hold in words, and no other', () => {
		const actions = checkedActions();

		expect(actions).toContain('sign_in');
		expect(Object.keys(ACTION_LABELS).toSorted(byText)).toEqual(actions.toSorted(byText));
	});
});

describe('logDetail', () => {
	it('says why a code check was refused', () => {
		expect(logDetail(row({ action: 'step_up_refused', details: { reason: 'stale_sign_in' } }))).toBe(
			'The sign-in was more than 5 minutes old'
		);
		expect(logDetail(row({ action: 'step_up_refused', details: { reason: 'no_sign_in' } }))).toBe(
			'No sign-in came with the code'
		);
		expect(logDetail(row({ action: 'step_up_refused', details: { reason: 'not_verified' } }))).toBe(
			'The sign-in could not be verified'
		);
		expect(logDetail(row({ action: 'step_up_refused', details: { reason: 'other_operator' } }))).toBe(
			"The sign-in was another operator's"
		);
		expect(logDetail(row({ action: 'step_up_refused', details: { reason: 'no_authenticator_code' } }))).toBe(
			'The sign-in had no authenticator-app code'
		);
	});

	it('shows a refusal reason it has no words for as recorded', () => {
		expect(logDetail(row({ action: 'step_up_refused', details: { reason: 'new_reason' } }))).toBe('new_reason');
	});

	it('names the Owner who asked for a refund and the day she asked', () => {
		const refund = row({
			action: 'refund',
			details: { requestedByStaffId: 's-1', requestedOn: '2026-10-30', credits: 2, amountCents: 5000, taxCents: 400 },
			requestedBy: { id: 's-1', name: 'Renata Alvarez' }
		});

		expect(logDetail(refund)).toBe('2 Credits, $50.00 back, $4.00 of it tax. Asked for by Renata Alvarez on October 30, 2026');
	});

	it('still says who asked for a refund once that Owner is no longer on file', () => {
		const refund = row({
			action: 'refund',
			details: { requestedByStaffId: 's-1', requestedOn: '2026-10-30', credits: 1, amountCents: 2500, taxCents: 0 },
			requestedBy: { id: 's-1', name: null }
		});

		expect(logDetail(refund)).toBe(
			'1 Credit, $25.00 back, $0.00 of it tax. Asked for by a Staff member no longer on file (s-1) on October 30, 2026'
		);
	});

	it('says what a founding grant gave', () => {
		expect(logDetail(row({ action: 'founding_grant', details: { staffCount: 3, credits: 30 } }))).toBe('30 Credits for 3 Staff');
	});

	it('says which address was let into the pilot', () => {
		expect(logDetail(row({ action: 'pilot_address_added', details: { address: 'ana@example.com', requestKey: 'k' } }))).toBe(
			'ana@example.com'
		);
	});

	it('says which version was published, and whether it was material', () => {
		const details = { document: 'privacy', version: '2027-01-01', material: true, requestKey: 'k' };
		expect(logDetail(row({ action: 'legal_version_published', details }))).toBe(
			'The Privacy Policy that takes effect January 1, 2027, material'
		);
		expect(logDetail(row({ action: 'legal_version_published', details: { ...details, document: 'terms', material: false } }))).toBe(
			'The Terms of Service that take effect January 1, 2027, not material'
		);
	});

	it('says which list showed Owners’ contacts, and for how many Practices', () => {
		expect(logDetail(row({ action: 'owner_contact_read', details: { list: 'practices', practiceIds: ['p-1', 'p-2'] } }))).toBe(
			'The Practices list, 2 Practices'
		);
		expect(logDetail(row({ action: 'owner_contact_read', details: { list: 'deleted', practiceIds: ['p-1'] } }))).toBe(
			'The Deleted Practices list, 1 Practice'
		);
		expect(logDetail(row({ action: 'owner_contact_read', details: { list: 'dormant', practiceIds: [] } }))).toBe('dormant, 0 Practices');
	});

	it('has nothing more to say about a sign-in', () => {
		expect(logDetail(row({ action: 'sign_in' }))).toBeUndefined();
	});
});

describe('subjectFact', () => {
	it('names a Practice, and says when its Deletion finalized', () => {
		expect(subjectFact({ kind: 'practice', id: 'p-1', name: 'Rooted Birth Collective', deleted: false })).toEqual({
			label: 'Practice',
			value: 'Rooted Birth Collective'
		});
		expect(subjectFact({ kind: 'practice', id: 'p-2', name: 'Gone Births', deleted: true })).toEqual({
			label: 'Practice',
			value: 'Gone Births (deleted)'
		});
	});

	it('names a Staff member, or says by id that one is no longer on file', () => {
		expect(subjectFact({ kind: 'staff', id: 's-1', name: 'Priya Raman', deleted: false })).toEqual({
			label: 'Staff member',
			value: 'Priya Raman'
		});
		expect(subjectFact({ kind: 'staff', id: 's-2', name: null, deleted: false })).toEqual({
			label: 'Staff member',
			value: 'No longer on file (s-2)'
		});
		expect(subjectFact({ kind: 'practice', id: 'p-3', name: null, deleted: false })).toEqual({
			label: 'Practice',
			value: 'No longer on file (p-3)'
		});
	});
});

describe('readOperatorLog', () => {
	it('reads the first page, then the page after a cursor', async () => {
		const fetcher = vi.fn<typeof fetch>(async () => Response.json({ items: [], hasMore: false }));

		await readOperatorLog(undefined, fetcher);
		await readOperatorLog('next/page', fetcher);

		expect(fetcher.mock.calls.map(([url]) => url)).toEqual(['/api/operator-log', '/api/operator-log?cursor=next%2Fpage']);
	});

	it('answers signed-out when the session has ended', async () => {
		const fetcher = vi.fn<typeof fetch>(async () => new Response(null, { status: 401 }));

		expect(await readOperatorLog(undefined, fetcher)).toEqual({ kind: 'signed-out' });
	});
});
