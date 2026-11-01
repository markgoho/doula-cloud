/* eslint-disable unicorn/no-null -- doula-ops answers null for a version not yet published */
import { describe, expect, it, vi } from 'vitest';
import { owners, readLegalVersionCheck, readLegalVersions, sendLegalPublication } from './legalVersions.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

describe('owners', () => {
	it('counts the Owners a material version emails', () => {
		expect(owners(true, 1)).toBe('1 Owner');
		expect(owners(true, 4)).toBe('4 Owners');
	});

	it('says a version that is not material emails no one', () => {
		expect(owners(false, 0)).toBe('None. A version that is not material emails no one.');
	});
});

describe('readLegalVersions', () => {
	it('reads every version', async () => {
		const items = [
			{
				document: 'privacy',
				documentName: 'Privacy Policy',
				version: '2026-12-15',
				change: 'Names a processor.',
				material: true,
				publishedAt: null
			}
		];
		const fetcher = answering(200, { items });

		expect(await readLegalVersions(fetcher)).toEqual({ kind: 'ok', value: items });
		expect(fetcher).toHaveBeenCalledWith('/api/legal-versions', { credentials: 'include' });
	});

	it('sends a signed-out read to sign in', async () => {
		expect(await readLegalVersions(answering(401, { code: 'UNAUTHORIZED', message: 'Sign in' }))).toEqual({
			kind: 'signed-out'
		});
	});
});

describe('readLegalVersionCheck', () => {
	it('reads the version the address names', async () => {
		const check = {
			document: 'privacy',
			documentName: 'Privacy Policy',
			version: '2026-12-15',
			change: 'Names a processor.',
			material: true,
			owners: 4,
			publishedAt: null
		};
		const fetcher = answering(200, check);

		expect(await readLegalVersionCheck('privacy', '2026-12-15', fetcher)).toEqual({ kind: 'ok', value: check });
		expect(fetcher).toHaveBeenCalledWith('/api/legal-versions/privacy/2026-12-15', { credentials: 'include' });
	});

	it('says there is no such version', async () => {
		const message = 'There is no such version.';

		expect(await readLegalVersionCheck('cookies', '2026-12-15', answering(404, { code: 'NOT_FOUND', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});
});

describe('sendLegalPublication', () => {
	it('sends no fields under its key and hands back what changed', async () => {
		const publication = {
			document: 'privacy',
			documentName: 'Privacy Policy',
			version: '2026-12-15',
			material: true,
			publishedAt: '2026-11-02T14:00:00Z',
			queued: 4
		};
		const fetcher = answering(200, publication);

		expect(await sendLegalPublication('privacy', '2026-12-15', 'key-1', fetcher)).toEqual({
			kind: 'ok',
			value: publication
		});
		expect(fetcher).toHaveBeenCalledWith('/api/legal-versions/privacy/2026-12-15/publications', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{}'
		});
	});

	it('asks for a fresh code again', async () => {
		expect(
			await sendLegalPublication(
				'privacy',
				'2026-12-15',
				'key-1',
				answering(403, { code: 'SECOND_FACTOR_REQUIRED', message: 'code' })
			)
		).toEqual({ kind: 'step-up' });
	});

	it('says why the Terms refuse it', async () => {
		const message = 'A material version has to take effect at least 30 days after the emails are sent.';

		expect(
			await sendLegalPublication('terms', '2026-11-20', 'key-1', answering(422, { code: 'FAILED_PRECONDITION', message }))
		).toEqual({ kind: 'refused', errors: [{ message }] });
	});
});
