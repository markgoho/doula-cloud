import { describe, expect, it, vi } from 'vitest';
import {
	createContract,
	fillProse,
	loadClientContract,
	loadContract,
	mergeFieldLabel,
	saveContractValues,
	sendContract,
	setMergeFieldValue,
	signContract,
	voidContract
} from './contract.js';

function jsonResponse(body: unknown, status = 200): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}

describe('loadContract', () => {
	it('fetches the practice+engagement contract path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'draft',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: {}
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await loadContract(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract');
		expect(result).toEqual(contract);
	});

	it('returns undefined on a 404 (no contract created yet)', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		const result = await loadContract(fetcher, 'practice-1', 'eng-1');

		expect(result).toBeUndefined();
	});

	it('throws with the response body text on any other non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('server error', 500));

		await expect(loadContract(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('server error');
	});
});

describe('createContract', () => {
	it('POSTs to the practice+engagement contract path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'draft',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: {}
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract, 201));

		const result = await createContract(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract', {
			method: 'POST'
		});
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('no contract template found for this practice', 404));

		await expect(createContract(fetcher, 'practice-1', 'eng-1')).rejects.toThrow(
			'no contract template found for this practice'
		);
	});
});

describe('saveContractValues', () => {
	it('PUTs the full values map as JSON to the practice+engagement contract path', async () => {
		const values = { client_name: 'Jamie', price: '$1,200' };
		const contract = { engagementId: 'eng-1', status: 'draft', prose: '', mergeFields: [], values };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await saveContractValues(fetcher, 'practice-1', 'eng-1', values);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ values })
		});
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('contract is no longer a draft', 409));

		await expect(saveContractValues(fetcher, 'practice-1', 'eng-1', {})).rejects.toThrow(
			'contract is no longer a draft'
		);
	});
});

describe('sendContract', () => {
	it('POSTs to the practice+engagement contract send path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'sent',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: { client_name: 'Jamie' }
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await sendContract(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/send', {
			method: 'POST'
		});
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('contract is not a draft', 409));

		await expect(sendContract(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('contract is not a draft');
	});
});

describe('voidContract', () => {
	it('POSTs to the practice+engagement contract void path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'voided',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: { client_name: 'Jamie' }
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await voidContract(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/void', {
			method: 'POST'
		});
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('contract is not signed', 409));

		await expect(voidContract(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('contract is not signed');
	});
});

describe('loadClientContract', () => {
	it('fetches the portal engagement contract path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'sent',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: { client_name: 'Jamie' }
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await loadClientContract(fetcher, 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/eng-1/contract');
		expect(result).toEqual(contract);
	});

	it('returns null on a 404 (no Contract sent yet, or still a Draft)', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		const result = await loadClientContract(fetcher, 'eng-1');

		expect(result).toBeNull();
	});

	it('throws with the response body text on any other non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('server error', 500));

		await expect(loadClientContract(fetcher, 'eng-1')).rejects.toThrow('server error');
	});
});

describe('signContract', () => {
	it('POSTs the typed name and attestation to the portal engagement sign path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'signed',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: { client_name: 'Jamie' }
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await signContract(fetcher, 'eng-1', 'Jamie Doe', true);

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/eng-1/contract/sign', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ fullLegalName: 'Jamie Doe', attestation: true })
		});
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('contract is not awaiting signature', 409));

		await expect(signContract(fetcher, 'eng-1', 'Jamie Doe', true)).rejects.toThrow(
			'contract is not awaiting signature'
		);
	});
});

describe('fillProse', () => {
	it('substitutes every merge field placeholder with its filled value', () => {
		const result = fillProse('Agreement for {{client_name}} at {{price}}.', {
			client_name: 'Jamie',
			price: '$1,200'
		});
		expect(result).toBe('Agreement for Jamie at $1,200.');
	});

	it('leaves an unfilled placeholder blank', () => {
		const result = fillProse('Agreement for {{client_name}}.', {});
		expect(result).toBe('Agreement for .');
	});

	it('substitutes every occurrence of a repeated placeholder', () => {
		const result = fillProse('{{client_name}}, meet {{client_name}}.', { client_name: 'Jamie' });
		expect(result).toBe('Jamie, meet Jamie.');
	});

	it('leaves prose with no placeholders unchanged', () => {
		const result = fillProse('Plain prose, no merge fields.', {});
		expect(result).toBe('Plain prose, no merge fields.');
	});
});

describe('setMergeFieldValue', () => {
	it('sets the value for the given merge field key', () => {
		const result = setMergeFieldValue({}, 'client_name', 'Jamie');
		expect(result).toEqual({ client_name: 'Jamie' });
	});

	it('does not mutate the input object', () => {
		const original = { client_name: 'Jamie' };
		setMergeFieldValue(original, 'price', '$1,200');
		expect(original).toEqual({ client_name: 'Jamie' });
	});

	it('overwrites an existing value for the same key', () => {
		const result = setMergeFieldValue({ client_name: 'Jamie' }, 'client_name', 'Alex');
		expect(result).toEqual({ client_name: 'Alex' });
	});
});

describe('mergeFieldLabel', () => {
	it('returns the friendly label for a known merge field key', () => {
		expect(mergeFieldLabel('client_name')).toBe('Client name');
		expect(mergeFieldLabel('scope_of_service')).toBe('Scope of service');
	});

	it('falls back to the raw key for an unknown merge field', () => {
		expect(mergeFieldLabel('some_ad_hoc_token')).toBe('some_ad_hoc_token');
	});
});
