import { describe, expect, it } from 'vitest';
import { errorKindForStatus, refuseRead } from './errorPage.js';
import { jsonResponse } from './testResponse.js';

describe('errorKindForStatus', () => {
	it('maps 404 to notFound', () => {
		expect(errorKindForStatus(404)).toBe('notFound');
	});

	it('maps a 403 that carries no reason to refused', () => {
		expect(errorKindForStatus(403)).toBe('refused');
	});

	it('maps a 403 carrying the plain forbidden code to refused', () => {
		expect(errorKindForStatus(403, 'FORBIDDEN')).toBe('refused');
	});

	// The code the BFF answers for a Practice between initiate and
	// finalize: nothing about the reader's role, so the role refusal
	// would be a false sentence rather than a vague one.
	it('maps a 403 carrying the pending-deletion code to practiceLocked', () => {
		expect(errorKindForStatus(403, 'PRACTICE_PENDING_DELETION')).toBe('practiceLocked');
	});

	// The one 403 here the reader can clear herself.
	it('maps a 403 carrying the MFA code to secondFactor', () => {
		expect(errorKindForStatus(403, 'MFA_REQUIRED')).toBe('secondFactor');
	});

	// A code from a BFF newer than this app, or one added to
	// apierr.ForbiddenCodes without a state here yet: the reader still
	// gets the refusal every 403 said before #918, never a blank page.
	it('falls back to refused for a 403 carrying a code it does not recognize', () => {
		expect(errorKindForStatus(403, 'SOMETHING_ELSE')).toBe('refused');
	});

	// A reason code only ever means anything on a 403 -- the status still
	// decides everything else, so a code riding along on a 404 changes
	// nothing.
	it('ignores a reason code on any status but 403', () => {
		expect(errorKindForStatus(404, 'MFA_REQUIRED')).toBe('notFound');
	});

	it('maps 503 to unavailable', () => {
		expect(errorKindForStatus(503)).toBe('unavailable');
	});

	it('maps 500 to problem', () => {
		expect(errorKindForStatus(500)).toBe('problem');
	});

	it('maps any other unexpected status to problem', () => {
		expect(errorKindForStatus(418)).toBe('problem');
	});
});

describe('refuseRead', () => {
	it('carries the refusal code and message through to the error page', async () => {
		await expect(
			refuseRead(
				jsonResponse({ code: 'PRACTICE_PENDING_DELETION', message: 'this practice is being deleted' }, 403)
			)
		).rejects.toMatchObject({
			status: 403,
			body: { code: 'PRACTICE_PENDING_DELETION', message: 'this practice is being deleted' }
		});
	});

	// The sentence the six loaders threw before #918, kept for a body
	// that names no message at all, so `page.error.message` is never
	// empty.
	it('falls back to its own sentence when the refusal names no message', async () => {
		await expect(refuseRead(jsonResponse('', 403))).rejects.toMatchObject({
			status: 403,
			body: { message: 'not permitted to read this', code: undefined }
		});
	});

	// Not every refusal is a 403: the helper reports whatever the
	// response was, so a loader that reaches for it on some other
	// refusal does not silently relabel it.
	it('reports the response status rather than assuming 403', async () => {
		await expect(refuseRead(jsonResponse({ code: 'NOT_FOUND', message: 'gone' }, 404))).rejects.toMatchObject({
			status: 404,
			body: { code: 'NOT_FOUND' }
		});
	});
});
