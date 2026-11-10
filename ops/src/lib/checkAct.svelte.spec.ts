import { describe, expect, it, vi } from 'vitest';
import { STEP_UP_AGAIN } from './act.ts';
import { CheckAct } from './checkAct.svelte.ts';
import { SERVICE_PROBLEM } from './session.ts';
import { STEP_UP_PASSWORD_FIELD } from './stepUp.ts';

function stepUpAnswering(status: number) {
	return vi.fn<typeof fetch>(async () => new Response(undefined, { status }));
}

function confirming(fetcher = stepUpAnswering(204)) {
	return { email: () => 'mark@doula.cloud', freshIdToken: vi.fn(async () => 'fresh-token'), fetcher };
}

describe('CheckAct', () => {
	it('is sending until the act answers, then done with what it answered', async () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn() });
		const answer = Promise.withResolvers<{ kind: 'ok'; value: string }>();

		const sending = act.run(() => answer.promise);
		expect(act.state).toEqual({ status: 'sending' });
		expect(act.isSending).toBe(true);
		expect(act.value).toBeUndefined();
		answer.resolve({ kind: 'ok', value: 'given' });

		expect(await sending).toBe('given');
		expect(act.state).toEqual({ status: 'done', value: 'given' });
		expect(act.value).toBe('given');
		expect(act.errors).toEqual([]);
	});

	it('sends a second press under the same key, and a renewed act under a new one', async () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn() });
		const send = vi.fn<(key: string) => Promise<{ kind: 'refused'; errors: { message: string }[] }>>(async () => ({
			kind: 'refused',
			errors: [{ message: 'Try again.' }]
		}));

		await act.run(send);
		await act.run(send);
		act.renew();
		await act.run(send);

		const [first, second, third] = send.mock.calls.map(([key]) => key);
		expect(second).toBe(first);
		expect(third).not.toBe(first);
	});

	it('does nothing when pressed again while sending', async () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn() });
		const answer = Promise.withResolvers<{ kind: 'ok'; value: string }>();
		const send = vi.fn(() => answer.promise);

		const sending = act.run(send);
		expect(await act.run(send)).toBeUndefined();
		answer.resolve({ kind: 'ok', value: 'once' });
		await sending;

		expect(send).toHaveBeenCalledOnce();
	});

	it('is refused with the refusal, pointed where the page says', async () => {
		const act = new CheckAct<string>({
			onsignedout: vi.fn(),
			locate: (errors) => errors.map((error) => ({ ...error, targetId: 'owner-1' }))
		});

		await act.run(async () => ({ kind: 'refused', errors: [{ message: 'Choose the Owner', targetId: 'requested-by' }] }));

		expect(act.state).toEqual({ status: 'refused', errors: [{ message: 'Choose the Owner', targetId: 'owner-1' }] });
	});

	// A thrown act was silent before: the spinner stopped and nothing was said.
	it('says the service has a problem when the act throws', async () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn() });

		expect(
			await act.run(async () => {
				throw new TypeError('Failed to fetch');
			})
		).toBeUndefined();

		expect(act.errors).toEqual([{ message: SERVICE_PROBLEM }]);
		expect(act.isSending).toBe(false);
	});

	it('asks for the code again when doula-ops does', async () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn() });

		await act.run(async () => ({ kind: 'step-up' }));

		expect(act.errors).toEqual([STEP_UP_AGAIN]);
	});

	it('hands back once the session has ended', async () => {
		const onsignedout = vi.fn();
		const act = new CheckAct<string>({ onsignedout });

		await act.run(async () => ({ kind: 'signed-out' }));

		expect(act.state).toEqual({ status: 'signed-out' });
		expect(onsignedout).toHaveBeenCalledOnce();
	});

	it('takes a fresh code before an act that asks for one', async () => {
		const stepUp = confirming();
		const act = new CheckAct<string>({ onsignedout: vi.fn(), stepUp });
		act.password = 'pw';
		act.code = '123456';

		await act.run(async () => ({ kind: 'ok', value: 'opened' }));

		expect(stepUp.freshIdToken).toHaveBeenCalledWith('mark@doula.cloud', 'pw', '123456');
		expect(act.value).toBe('opened');
	});

	it('sends nothing when the fresh code is refused', async () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn(), stepUp: confirming() });
		const send = vi.fn();

		await act.run(send);

		expect(send).not.toHaveBeenCalled();
		expect(act.errors).toContainEqual({ message: 'Enter your password', targetId: STEP_UP_PASSWORD_FIELD });
	});

	// stepUp's own last request is not caught where it is made; this is where it is.
	it('says the service has a problem when taking the fresh code throws', async () => {
		const fetcher = vi.fn<typeof fetch>(async () => {
			throw new TypeError('Failed to fetch');
		});
		const act = new CheckAct<string>({ onsignedout: vi.fn(), stepUp: confirming(fetcher) });
		act.password = 'pw';
		act.code = '123456';

		await act.run(vi.fn());

		expect(act.errors).toEqual([{ message: SERVICE_PROBLEM }]);
	});

	it('takes a refusal the page found itself, and a renewed act forgets it and the code', () => {
		const act = new CheckAct<string>({ onsignedout: vi.fn() });
		act.password = 'pw';
		act.code = '123456';

		act.refuse([{ message: 'Enter the email address' }]);
		expect(act.errors).toEqual([{ message: 'Enter the email address' }]);

		act.renew();
		expect(act.state).toEqual({ status: 'idle' });
		expect([act.password, act.code]).toEqual(['', '']);
	});
});
