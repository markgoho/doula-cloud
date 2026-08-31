import { describe, expect, it, vi } from 'vitest';

// #539: every branch of the gate itself -- session-read shape,
// owner/admin carve-out, fail-open on error -- is covered once in
// #lib/practiceContractorGate.spec.ts. This route's own `load` is a thin
// call-through, so its test only has to prove the wiring: the right
// practiceId goes in, and whatever the gate returns comes back untouched.
const loadContractorGate = vi.hoisted(() => vi.fn());
vi.mock('#lib/practiceContractorGate.js', () => ({ loadContractorGate }));

describe('clients/+page.ts load (#539)', () => {
	it("reads this Practice's contractor gate and forwards its result", async () => {
		const { load } = await import('./+page.js');
		loadContractorGate.mockResolvedValue({ isContractor: true });

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(loadContractorGate).toHaveBeenCalledWith('practice-1');
		expect(result).toEqual({ isContractor: true });
	});
});
