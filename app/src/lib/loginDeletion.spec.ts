import { describe, expect, it, vi } from 'vitest';
import { deleteOwnLogin } from './loginDeletion.js';
import { jsonResponse as response } from './testResponse.js';

describe('deleteOwnLogin', () => {
	it('sends a DELETE carrying X-Confirmed, and names nobody', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('', 204));

		await deleteOwnLogin(fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/staff/account', {
			method: 'DELETE',
			headers: { 'X-Confirmed': 'true' }
		});
	});

	it('throws with the Practices in the way named, on the last-Owner refusal', async () => {
		const refusal =
			'cannot delete your login while you are the only Owner of Rochester Birth Collective: hand ownership to someone else, or delete the practice first';
		const fetcher = vi.fn().mockResolvedValue(response(refusal, 409));

		await expect(deleteOwnLogin(fetcher)).rejects.toThrow('Rochester Birth Collective');
	});
});
