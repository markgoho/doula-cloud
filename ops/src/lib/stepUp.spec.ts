import { describe, expect, it, vi } from 'vitest';
import { STEP_UP_CODE_FIELD, STEP_UP_PASSWORD_FIELD, stepUp } from './stepUp.ts';
import { SERVICE_PROBLEM } from './session.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

const firebaseRefusal = (code: string) => Object.assign(new Error(code), { code });

describe('stepUp', () => {
	it('signs in again with the password and code and stamps the session with the fresh token', async () => {
		const fetcher = answering(204);
		const freshIdToken = vi.fn(async () => 'fresh-token');

		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '123 456' },
			{ freshIdToken, fetcher }
		);

		expect(errors).toEqual([]);
		expect(freshIdToken).toHaveBeenCalledWith('mark@doula.cloud', 'pw', '123456');
		expect(fetcher).toHaveBeenCalledWith('/api/session/step-up', {
			method: 'POST',
			credentials: 'include',
			headers: { Authorization: 'Bearer fresh-token' }
		});
	});

	it('says what to enter when the password and code are empty, and asks nobody', async () => {
		const fetcher = answering(204);
		const freshIdToken = vi.fn(async () => 'fresh-token');

		const errors = await stepUp({ email: 'mark@doula.cloud', password: '', code: '' }, { freshIdToken, fetcher });

		expect(errors).toEqual([
			{ message: 'Enter your password', field: STEP_UP_PASSWORD_FIELD },
			{ message: 'Enter the 6-digit code from your authenticator app', field: STEP_UP_CODE_FIELD }
		]);
		expect(freshIdToken).not.toHaveBeenCalled();
		expect(fetcher).not.toHaveBeenCalled();
	});

	it('shows a refused code in plain words beside the code, and tells doula-ops of the try', async () => {
		const fetcher = answering(401, { code: 'UNAUTHORIZED', message: 'Enter your password and the code.' });
		const freshIdToken = vi.fn(async () => {
			throw firebaseRefusal('auth/invalid-verification-code');
		});

		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '000000' },
			{ freshIdToken, fetcher }
		);

		expect(errors).toEqual([
			{ message: 'Enter the 6-digit code your authenticator app shows now', field: STEP_UP_CODE_FIELD }
		]);
		expect(fetcher).toHaveBeenCalledWith('/api/session/step-up', { method: 'POST', credentials: 'include' });
	});

	it('shows a refused password beside the password, and tells doula-ops of the try', async () => {
		const fetcher = answering(401, { code: 'UNAUTHORIZED', message: 'Enter your password and the code.' });
		const freshIdToken = vi.fn(async () => {
			throw firebaseRefusal('auth/wrong-password');
		});

		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'wrong', code: '123456' },
			{ freshIdToken, fetcher }
		);

		expect(errors).toEqual([{ message: 'Enter the correct password', field: STEP_UP_PASSWORD_FIELD }]);
		expect(fetcher).toHaveBeenCalledOnce();
	});

	it('says to wait after too many tries', async () => {
		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '123456' },
			{
				freshIdToken: async () => {
					throw firebaseRefusal('auth/too-many-requests');
				},
				fetcher: answering(401, {})
			}
		);

		expect(errors).toEqual([
			{ message: 'Too many tries. Wait a few minutes, then try again', field: STEP_UP_PASSWORD_FIELD }
		]);
	});

	it('says the service has a problem for a refusal that is not about the password or code', async () => {
		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '123456' },
			{
				freshIdToken: async () => {
					throw new Error('network');
				},
				fetcher: answering(401, {})
			}
		);

		expect(errors).toEqual([{ message: SERVICE_PROBLEM }]);
	});

	it("shows doula-ops's own refusal of the fresh token", async () => {
		const fetcher = answering(401, { code: 'UNAUTHORIZED', message: 'Enter your password and the code your app shows now.' });

		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '123456' },
			{ freshIdToken: async () => 'stale-token', fetcher }
		);

		expect(errors).toEqual([{ message: 'Enter your password and the code your app shows now.' }]);
	});

	it('says the service has a problem when doula-ops fails', async () => {
		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '123456' },
			{ freshIdToken: async () => 'fresh-token', fetcher: answering(500, { message: 'boom' }) }
		);

		expect(errors).toEqual([{ message: SERVICE_PROBLEM }]);
	});

	it('still shows the refused code when doula-ops cannot be told of the try', async () => {
		const fetcher = vi.fn<typeof fetch>(async () => {
			throw new TypeError('offline');
		});

		const errors = await stepUp(
			{ email: 'mark@doula.cloud', password: 'pw', code: '000000' },
			{
				freshIdToken: async () => {
					throw firebaseRefusal('auth/code-expired');
				},
				fetcher
			}
		);

		expect(errors).toEqual([
			{
				message: 'That code has expired. Enter the code your authenticator app shows now',
				field: STEP_UP_CODE_FIELD
			}
		]);
	});
});
