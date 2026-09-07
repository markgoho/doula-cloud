import { describe, expect, it, vi } from 'vitest';
import {
	awaitingContractStatusLabel,
	createContract,
	downloadClientSignedContractPdf,
	downloadSignedContractPdf,
	fillProse,
	loadClientContract,
	loadContract,
	loadPracticeAwaitingContracts,
	mergeFieldLabel,
	missingMergeFieldKeys,
	practiceAwaitingContractsPath,
	saveContractValues,
	sendContract,
	setMergeFieldValue,
	signContract,
	voidContract
} from './contract.js';
import { jsonResponse } from './testResponse.js';

function blobResponse(): Response {
	return new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 });
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

	// #258: an Owner/Admin reader's GET response splits money-tagged merge
	// fields into a separate moneyValues field (ADR-0008), so the Staff
	// Engagement page and its Send-completeness check would otherwise read
	// a filled money field as missing.
	it('folds moneyValues into values for an Owner/Admin reader', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'draft',
			prose: 'Agreement for {{client_name}} at {{money_price}}.',
			mergeFields: ['client_name', 'money_price'],
			values: { client_name: 'Jamie' },
			moneyValues: { money_price: '$1,200' }
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await loadContract(fetcher, 'practice-1', 'eng-1');

		expect(result?.values).toEqual({ client_name: 'Jamie', money_price: '$1,200' });
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

describe('downloadClientSignedContractPdf', () => {
	it('fetches the portal engagement contract pdf path and returns a Blob', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());

		const blob = await downloadClientSignedContractPdf(fetcher, 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/eng-1/contract/pdf');
		expect(blob).toBeInstanceOf(Blob);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('no signed contract found for this engagement', 404));

		await expect(downloadClientSignedContractPdf(fetcher, 'eng-1')).rejects.toThrow(
			'no signed contract found for this engagement'
		);
	});
});

describe('downloadSignedContractPdf', () => {
	it('fetches the practice+engagement contract pdf path and returns a Blob', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());

		const blob = await downloadSignedContractPdf(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/pdf');
		expect(blob).toBeInstanceOf(Blob);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('signed PDF not found', 404));

		await expect(downloadSignedContractPdf(fetcher, 'practice-1', 'eng-1')).rejects.toThrow(
			'signed PDF not found'
		);
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

describe('missingMergeFieldKeys', () => {
	it('returns an empty array when every merge field has a value', () => {
		const result = missingMergeFieldKeys(['client_name', 'price'], { client_name: 'Jamie', price: '$1,200' });
		expect(result).toEqual([]);
	});

	it('names a key with no entry at all in values', () => {
		const result = missingMergeFieldKeys(['client_name', 'price'], { client_name: 'Jamie' });
		expect(result).toEqual(['price']);
	});

	it('names a key mapped to an empty string', () => {
		const result = missingMergeFieldKeys(['client_name'], { client_name: '' });
		expect(result).toEqual(['client_name']);
	});

	it('names a key mapped to a whitespace-only string', () => {
		const result = missingMergeFieldKeys(['client_name'], { client_name: ' '.repeat(3) });
		expect(result).toEqual(['client_name']);
	});

	it('names every missing key, not only the first', () => {
		const result = missingMergeFieldKeys(['practice_name', 'client_name', 'price'], { client_name: 'Jamie' });
		expect(result).toEqual(['practice_name', 'price']);
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

describe('practiceAwaitingContractsPath', () => {
	it('addresses the Practice-wide awaiting-signature roll-up', () => {
		expect(practiceAwaitingContractsPath('practice-1')).toBe(
			'/api/practices/practice-1/contracts/awaiting-signature'
		);
	});

	it('carries an encoded cursor when there is one', () => {
		expect(practiceAwaitingContractsPath('practice-1', 'a+b/c=')).toBe(
			'/api/practices/practice-1/contracts/awaiting-signature?cursor=a%2Bb%2Fc%3D'
		);
	});
});

describe('loadPracticeAwaitingContracts', () => {
	const row = {
		engagementId: 'eng-1',
		contractId: 'contract-1',
		clientId: 'client-1',
		clientName: 'Ada',
		status: 'draft',
		createdAt: '2026-01-01T00:00:00Z'
	};

	it('fetches the Practice-wide path and returns the page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [row], hasMore: false }));

		const result = await loadPracticeAwaitingContracts(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/contracts/awaiting-signature');
		expect(result).toEqual({ items: [row], hasMore: false });
	});

	it('carries the cursor to the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPracticeAwaitingContracts(fetcher, 'practice-1', 'cursor-1');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/contracts/awaiting-signature?cursor=cursor-1'
		);
	});

	it('throws with the response body text on a non-2xx response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted', 403));

		await expect(loadPracticeAwaitingContracts(fetcher, 'practice-1')).rejects.toThrow('not permitted');
	});
});

describe('awaitingContractStatusLabel', () => {
	it('labels a draft Contract as work the Practice still owes', () => {
		expect(awaitingContractStatusLabel('draft')).toBe('Draft');
	});

	it('labels a sent Contract as work the Client still owes', () => {
		expect(awaitingContractStatusLabel('sent')).toBe('Sent');
	});

	it('falls back to the raw status for anything unrecognized', () => {
		expect(awaitingContractStatusLabel('signed')).toBe('signed');
	});
});
