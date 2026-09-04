import { describe, expect, it, vi } from 'vitest';

// #465, mirroring clients-list-load.spec.ts (#539): every branch of the
// gate itself lives in #lib/practiceContractorGate.spec.ts, so this
// route's own `load` only has to prove the wiring.
const loadContractorGate = vi.hoisted(() => vi.fn());
vi.mock('#lib/practiceContractorGate.js', () => ({ loadContractorGate }));

describe('clients/[clientId]/+page.ts load (#465)', () => {
	it("reads this Practice's contractor gate and forwards its result", async () => {
		const { load } = await import('./+page.js');
		loadContractorGate.mockResolvedValue({ isContractor: true });

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(loadContractorGate).toHaveBeenCalledWith('practice-1');
		expect(result).toEqual({ isContractor: true });
	});
});
