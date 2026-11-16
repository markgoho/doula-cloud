import { describe, expect, it, vi } from 'vitest';
import {
	contractStepper,
	nameWithInitial,
	contractStatusText,
	createContract,
	declineContractVoidRequest,
	downloadClientSignedContractPdf,
	downloadSignedContractPdf,
	editableMergeFields,
	editableValues,
	fillProse,
	loadClientContract,
	loadContract,
	CONTRACTS_RESULT_RULER,
	CONTRACT_VIEW_TAGS,
	contractOrderOf,
	contractPeriodOf,
	contractOrderOptions,
	contractOrderedBy,
	contractStatusWord,
	contractViewOrder,
	contractsQueryString,
	contractsResultSummary,
	contractsViewOf,
	daysWaitingWords,
	loadPracticeContracts,
	loadPracticeVoidRequests,
	mergeFieldLabel,
	missingMergeFieldKeys,
	orderForView,
	practiceContractsPath,
	practiceVoidRequestsPath,
	requestContractVoid,
	saveContractValues,
	openContract,
	remindContract,
	reminderDeliveryWords,
	sendContract,
	setMergeFieldValue,
	signContract,
	signedContractFilename,
	signedPdfDownloadedWords,
	voidContract,
	voidRequestAsked,
	voidRequestsHeading,
	isVoidRequestReasonTooLong,
	VOID_REQUEST_REASON_LIMIT
} from './contract.js';
import { jsonResponse } from './testResponse.js';

describe('contractStatusText', () => {
	it('says each status in the words the Staff page reads (#2004)', () => {
		expect(['draft', 'sent', 'signed', 'voided'].map((status) => contractStatusText(status))).toEqual([
			'Draft, not sent yet',
			'Sent, waiting for the Client to sign',
			'Signed',
			'Voided, no longer active'
		]);
	});

	it('reads an unknown status as itself rather than blank', () => {
		expect(contractStatusText('archived')).toBe('archived');
	});
});

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

describe('remindContract', () => {
	it('POSTs to the contract remind path and returns the decoded contract', async () => {
		const contract = { engagementId: 'eng-1', status: 'sent', prose: '', mergeFields: [], values: {} };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await remindContract(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/remind', {
			method: 'POST'
		});
		expect(result).toEqual(contract);
	});

	it('throws with the refusal on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('This Client was already emailed about a Contract today.', 409));

		await expect(remindContract(fetcher, 'practice-1', 'eng-1')).rejects.toThrow(
			'This Client was already emailed about a Contract today.'
		);
	});
});

describe('reminderDeliveryWords', () => {
	it('says what became of the mail, and nothing once it was sent', () => {
		expect(reminderDeliveryWords('queued')).toBe('on its way');
		expect(reminderDeliveryWords('not_delivered')).toBe('did not arrive');
		expect(reminderDeliveryWords('sent')).toBe('');
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

describe('requestContractVoid', () => {
	it('POSTs the reason to the void-request path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'signed',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: { client_name: 'Jamie' },
			voidRequests: [
				{
					id: 'request-1',
					requestedBy: 'staff-1',
					requestedByName: 'Jamie Doula',
					reason: 'the client rescheduled',
					status: 'open',
					createdAt: '2026-01-01T00:00:00Z'
				}
			]
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract, 201));

		const result = await requestContractVoid(fetcher, 'practice-1', 'eng-1', 'the client rescheduled');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/void-request', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason: 'the client rescheduled' })
		});
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('a void may only be requested for a signed contract: it is draft', 409));

		await expect(requestContractVoid(fetcher, 'practice-1', 'eng-1', 'asking')).rejects.toThrow(
			'a void may only be requested for a signed contract: it is draft'
		);
	});
});

describe('declineContractVoidRequest', () => {
	it('POSTs the reason to the decline path and returns the decoded contract', async () => {
		const contract = {
			engagementId: 'eng-1',
			status: 'signed',
			prose: 'Agreement for {{client_name}}.',
			mergeFields: ['client_name'],
			values: { client_name: 'Jamie' },
			voidRequests: [
				{
					id: 'request-1',
					requestedBy: 'staff-1',
					requestedByName: 'Jamie Doula',
					reason: 'the client rescheduled',
					status: 'declined',
					declineReason: 'not yet',
					createdAt: '2026-01-01T00:00:00Z'
				}
			]
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(contract));

		const result = await declineContractVoidRequest(fetcher, 'practice-1', 'eng-1', 'request-1', 'not yet');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/eng-1/contract/void-request/request-1/decline',
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ reason: 'not yet' })
			}
		);
		expect(result).toEqual(contract);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('no open void request found with this id for this contract', 404));

		await expect(
			declineContractVoidRequest(fetcher, 'practice-1', 'eng-1', 'request-1', 'not yet')
		).rejects.toThrow('no open void request found with this id for this contract');
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
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('no contract on this work has ever been signed', 404));

		await expect(downloadClientSignedContractPdf(fetcher, 'eng-1')).rejects.toThrow(
			'no contract on this work has ever been signed'
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

describe('openContract', () => {
	it('POSTs to the portal engagement open path', async () => {
		const fetcher = vi.fn().mockResolvedValue({ ok: true });

		await openContract(fetcher, 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/eng-1/contract/open', { method: 'POST' });
	});

	it('throws the response message on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ error: 'Contract is signed' }, 409));

		await expect(openContract(fetcher, 'eng-1')).rejects.toThrow('Contract is signed');
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

describe('signedContractFilename', () => {
	it.each([
		['Quinn Grier', 'signed-contract-quinn-grier.pdf'],
		['Zoë  O’Brien-Núñez', 'signed-contract-zoe-o-brien-nunez.pdf'],
		['  ', 'signed-contract.pdf']
	])('names %j as %s', (name, filename) => {
		expect(signedContractFilename(name)).toBe(filename);
	});
});

describe('signedPdfDownloadedWords', () => {
	it('names the Client', () => {
		expect(signedPdfDownloadedWords('Quinn Grier')).toBe('Downloaded the Signed PDF for Quinn Grier.');
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

	// #2222: the same values and the same strings as the BFF's
	// TestFillProse_PrintsADateFieldAsACalendarDay, so the Staff preview,
	// the portal page and the signed PDF agree.
	it('prints a date field as a calendar day', () => {
		const result = fillProse('Dates of care: {{engagement_start_date}} through {{engagement_end_date}}', {
			engagement_start_date: '2026-10-27',
			engagement_end_date: '2027-04-05'
		});
		expect(result).toBe('Dates of care: Oct 27, 2026 through Apr 5, 2027');
	});

	it('passes a date field through as typed when it is not a real day', () => {
		expect(fillProse('From {{engagement_start_date}}', { engagement_start_date: 'next spring' })).toBe(
			'From next spring'
		);
		expect(fillProse('From {{engagement_start_date}}', { engagement_start_date: '2026-02-30' })).toBe(
			'From 2026-02-30'
		);
	});

	it('leaves a field that is not a date untouched', () => {
		expect(fillProse('Scope: {{scope_of_service}}', { scope_of_service: '2026-10-27' })).toBe('Scope: 2026-10-27');
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

describe('editableMergeFields', () => {
	it('removes the reserved price key', () => {
		expect(editableMergeFields(['client_name', 'price', 'scope_of_service'])).toEqual([
			'client_name',
			'scope_of_service'
		]);
	});

	it('returns every key unchanged when price is absent', () => {
		expect(editableMergeFields(['client_name', 'scope_of_service'])).toEqual(['client_name', 'scope_of_service']);
	});

	it('returns an empty array unchanged', () => {
		expect(editableMergeFields([])).toEqual([]);
	});
});

describe('editableValues', () => {
	it('removes the reserved price key', () => {
		expect(editableValues({ client_name: 'Jamie', price: '$150.00' })).toEqual({ client_name: 'Jamie' });
	});

	it('returns every entry unchanged when price is absent', () => {
		expect(editableValues({ client_name: 'Jamie' })).toEqual({ client_name: 'Jamie' });
	});

	it('returns an empty object unchanged', () => {
		expect(editableValues({})).toEqual({});
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

const parameters = (query: string) => new URLSearchParams(query);
const summary = (overrides: Partial<Parameters<typeof contractsResultSummary>[0]> = {}) =>
	contractsResultSummary({
		view: 'not-signed',
		order: '',
		shown: 12,
		hasMore: false,
		total: 12,
		isMine: false,
		doulaName: '',
		search: '',
		...overrides
	});

describe('contractsViewOf', () => {
	it.each([
		['', 'not-signed'],
		['show=not-signed', 'not-signed'],
		['show=nonsense', 'not-signed'],
		['show=draft', 'draft'],
		['show=sent', 'sent'],
		['show=signed', 'signed'],
		['show=voided', 'voided'],
		['show=all', 'all']
	] as const)('reads %j as the %s view', (query, view) => {
		expect(contractsViewOf(parameters(query))).toBe(view);
	});
});

describe('the order a view reads in', () => {
	it.each([
		['not-signed', 'oldest'],
		['draft', 'oldest'],
		['sent', 'oldest'],
		['signed', 'newest'],
		['voided', 'newest'],
		['all', 'newest']
	] as const)("is %s's own: %s", (view, own) => {
		expect(contractViewOrder(view)).toBe(own);
		expect(orderForView(own, view)).toBe('');
	});

	it('keeps an order that is not the view own', () => {
		expect(orderForView('newest', 'sent')).toBe('newest');
		expect(orderForView('client', 'all')).toBe('client');
		expect(orderForView('', 'all')).toBe('');
	});

	it('reads the address order, leaving out the view own and an unknown one', () => {
		expect(contractOrderOf(parameters('order=newest'), 'sent')).toBe('newest');
		expect(contractOrderOf(parameters('order=oldest'), 'sent')).toBe('');
		expect(contractOrderOf(parameters('order=client'), 'signed')).toBe('client');
		expect(contractOrderOf(parameters('order=sideways'), 'signed')).toBe('');
		expect(contractOrderOf(parameters(''), 'signed')).toBe('');
	});

	it.each([
		['not-signed', ['Longest wait first', 'Newest first']],
		['draft', ['Made, oldest first', 'Made, newest first']],
		['sent', ['Longest wait first', 'Shortest wait first']],
		['signed', ['Signed, newest first', 'Signed, oldest first']],
		['voided', ['Voided, newest first', 'Voided, oldest first']],
		['all', ['Newest first', 'Oldest first']]
	] as const)('offers the %s view its own order first, then the other and by Client', (view, [own, other]) => {
		const options = contractOrderOptions(view);
		expect(options.map((option) => option.label)).toEqual([own, other, 'Client, A to Z']);
		expect(options[0]!.value).toBe('');
		expect(options.map((option) => option.value)).toContain('client');
	});

	it('names the column an order sorts the table by', () => {
		expect(contractOrderedBy('', 'not-signed')).toEqual({ column: 'Status', direction: 'ascending' });
		expect(contractOrderedBy('', 'signed')).toEqual({ column: 'Status', direction: 'descending' });
		expect(contractOrderedBy('oldest', 'signed')).toEqual({ column: 'Status', direction: 'ascending' });
		expect(contractOrderedBy('client', 'sent')).toEqual({ column: 'Client', direction: 'ascending' });
	});
});

describe('contractsQueryString', () => {
	it('is empty for the default view in its own order', () => {
		expect(contractsQueryString({ view: 'not-signed', order: '' })).toBe('');
		expect(contractsQueryString({ view: 'not-signed', order: 'oldest' })).toBe('');
	});

	it('names each narrowing and leaves the Client search out', () => {
		expect(
			contractsQueryString({ view: 'sent', order: 'newest', isMine: true, doula: 'staff-1' })
		).toBe('?show=sent&mine=true&doula=staff-1&order=newest');
	});
});

describe('practiceContractsPath', () => {
	it('is the bare list for the default view', () => {
		expect(practiceContractsPath('practice-1', { view: 'not-signed', order: '', isMine: false, doula: '' })).toBe(
			'/api/practices/practice-1/contracts'
		);
	});

	it('carries the view, order, narrowing and cursor', () => {
		expect(
			practiceContractsPath('practice-1', { view: 'signed', order: 'client', isMine: true, doula: 'staff-1' }, 'a+b')
		).toBe('/api/practices/practice-1/contracts?view=signed&order=client&mine=true&doula=staff-1&cursor=a%2Bb');
	});
});

describe('loadPracticeContracts', () => {
	const query = { view: 'sent', order: '', isMine: false, doula: '' } as const;

	it('reads a page by GET and returns it', async () => {
		const page = { items: [], hasMore: false, total: 0 };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(page));

		await expect(loadPracticeContracts(fetcher, 'practice-1', query, 'c1')).resolves.toEqual(page);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/contracts?view=sent&cursor=c1');
	});

	it('sends a search in a POST body, never in the address', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false, total: 0 }));

		await loadPracticeContracts(fetcher, 'practice-1', { ...query, view: 'not-signed', search: 'quinn', isMine: true });

		const [path, init] = fetcher.mock.calls[0]!;
		expect(path).toBe('/api/practices/practice-1/contracts/list');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body)).toEqual({ view: '', order: '', doula: '', mine: true, search: 'quinn', cursor: '' });
	});

	it('carries the cursor of a searched page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false, total: 0 }));

		await loadPracticeContracts(fetcher, 'practice-1', { ...query, search: 'q' }, 'next');

		expect(JSON.parse(fetcher.mock.calls[0]![1].body)).toMatchObject({ view: 'sent', cursor: 'next' });
	});

	it('throws with the response text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted', 403));

		await expect(loadPracticeContracts(fetcher, 'practice-1', query)).rejects.toThrow('not permitted');
	});
});

describe('the void requests read', () => {
	it('addresses the queue, with a cursor when given', () => {
		expect(practiceVoidRequestsPath('practice-1')).toBe('/api/practices/practice-1/contracts/void-requests');
		expect(practiceVoidRequestsPath('practice-1', 'a+b')).toBe(
			'/api/practices/practice-1/contracts/void-requests?cursor=a%2Bb'
		);
	});

	it('returns the page', async () => {
		const page = { items: [], hasMore: false, total: 0 };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(page));

		await expect(loadPracticeVoidRequests(fetcher, 'practice-1', 'c1')).resolves.toEqual(page);
		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/contracts/void-requests?cursor=c1');
	});

	it('throws with the response text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted', 403));

		await expect(loadPracticeVoidRequests(fetcher, 'practice-1')).rejects.toThrow('not permitted');
	});

	it('counts the requests and says when each was asked', () => {
		expect(voidRequestsHeading(3)).toBe('3 void requests to decide');
		expect(voidRequestsHeading(1)).toBe('1 void request to decide');
		expect(voidRequestAsked(11)).toBe('asked 11 days ago');
		expect(voidRequestAsked(1)).toBe('asked 1 day ago');
		expect(voidRequestAsked(0)).toBe('asked today');
	});
});

describe('contract words', () => {
	it('says each state and a wait', () => {
		expect(contractStatusWord('draft')).toBe('Draft');
		expect(contractStatusWord('voided')).toBe('Voided');
		expect(daysWaitingWords(12)).toBe('12 days');
		expect(daysWaitingWords(1)).toBe('1 day');
	});

	it('tags the four states and not the default view or All', () => {
		expect(CONTRACT_VIEW_TAGS).toEqual({
			draft: 'Draft Contracts',
			sent: 'Sent Contracts',
			signed: 'Signed Contracts',
			voided: 'Voided Contracts'
		});
	});
});

describe('contractsResultSummary', () => {
	it('says a count, the view and the order', () => {
		expect(summary()).toBe('Showing 12 Contracts not signed, longest wait first, then Drafts.');
		expect(summary({ shown: 1, total: 1 })).toBe('Showing 1 Contract not signed, longest wait first, then Drafts.');
		expect(summary({ view: 'signed', shown: 2, total: 2 })).toBe('Showing 2 Signed Contracts, newest first.');
		expect(summary({ view: 'all', order: 'client' })).toBe('Showing 12 Contracts, Client A to Z.');
	});

	it('says how many of how many while more exist', () => {
		expect(summary({ view: 'signed', shown: 30, hasMore: true, total: 41 })).toBe(
			'Showing the first 30 of 41 Signed Contracts, newest first.'
		);
		expect(summary({ shown: 30, hasMore: true, total: 41, isMine: true })).toBe(
			'Showing the first 30 of your 41 Contracts not signed, longest wait first, then Drafts.'
		);
		expect(summary({ shown: 30, hasMore: true, total: 3 })).toContain('the first 30 of 30');
	});

	it('says whose Contracts and which search', () => {
		expect(summary({ shown: 4, total: 4, isMine: true })).toBe(
			'Showing 4 of your Contracts not signed, longest wait first, then Drafts.'
		);
		expect(summary({ view: 'sent', shown: 2, total: 2, doulaName: 'Priya Raman', search: 'quinn' })).toBe(
			`Showing 2 Sent Contracts on Priya Raman's Engagements matching "quinn", longest wait first.`
		);
	});

	it('says the Sent view by its wait', () => {
		expect(summary({ view: 'sent', order: 'newest' })).toContain('shortest wait first');
		expect(summary({ view: 'sent', order: 'client' })).toContain('Client A to Z');
	});

	it.each([
		['not-signed', 'No Contract is waiting to be signed.'],
		['draft', 'No Draft Contract is waiting to be sent.'],
		['sent', 'No Sent Contract is waiting on a Client.'],
		['signed', 'No Contract is signed.'],
		['voided', 'No Contract has been voided.']
	] as const)('says nothing in the %s view', (view, words) => {
		expect(summary({ view, shown: 0 })).toBe(words);
	});

	it('says nothing in the whole book, and in a narrowed one', () => {
		expect(summary({ view: 'all', shown: 0 })).toBe(
			'No Contracts yet. One appears here when a Contract is made on an Engagement.'
		);
		expect(summary({ view: 'all', shown: 0, isMine: true })).toBe('No Contract of yours.');
		expect(summary({ view: 'sent', shown: 0, isMine: true, doulaName: 'Priya Raman', search: 'q' })).toBe(
			`No Sent Contract of yours on Priya Raman's Engagements matching "q" is waiting on a Client.`
		);
	});

	it('holds room for the longest line it can say', () => {
		expect(CONTRACTS_RESULT_RULER.length).toBeGreaterThan(summary().length);
		expect(CONTRACTS_RESULT_RULER).toContain('D'.repeat(30));
	});
});

describe('who took a Contract\u{2019}s step', () => {
	it.each([
		[{ status: 'draft', madeByName: 'Dee Whitlock' }, 'Made', 'Dee Whitlock'],
		[{ status: 'sent', sentByName: 'Dee Whitlock' }, 'Sent', 'Dee Whitlock'],
		[{ status: 'signed', sentByName: 'Dee Whitlock' }, 'Sent', 'Dee Whitlock'],
		[{ status: 'voided', sentByName: 'Dee Whitlock', voidedByName: 'Renata Alvarez' }, 'Voided', 'Renata Alvarez']
	] as const)('names %j', (contract, verb, name) => {
		expect(contractStepper(contract)).toEqual({ verb, name });
	});

	it('names no one where the row holds no name for its step', () => {
		expect(contractStepper({ status: 'draft' })).toBeUndefined();
	});

	it('shortens a name to its first words and a last initial', () => {
		expect(nameWithInitial('Anne-Marie Ochieng-Whitfield')).toBe('Anne-Marie O.');
		expect(nameWithInitial('Mary Ann Smith')).toBe('Mary Ann S.');
		expect(nameWithInitial('Cher')).toBe('Cher');
	});
});

describe('isVoidRequestReasonTooLong', () => {
	it('counts characters, not bytes, after trimming', () => {
		expect(isVoidRequestReasonTooLong('\u{E9}'.repeat(VOID_REQUEST_REASON_LIMIT))).toBe(false);
		expect(isVoidRequestReasonTooLong(` ${'a'.repeat(VOID_REQUEST_REASON_LIMIT)} `)).toBe(false);
		expect(isVoidRequestReasonTooLong('a'.repeat(VOID_REQUEST_REASON_LIMIT + 1))).toBe(true);
	});
});

describe('the Contracts list narrowed to a period (#2954)', () => {
	const october = { from: '2026-10-01', to: '2026-10-31' };

	it('reads the period from the address, and none in the Draft view', () => {
		expect(contractPeriodOf(parameters('sent=last-month'), 'signed')).toEqual({ preset: 'last-month' });
		expect(contractPeriodOf(parameters('sent=last-month'), 'draft')).toBeUndefined();
	});

	it('names the period in the address, and drops it in the Draft view', () => {
		expect(contractsQueryString({ view: 'signed', order: '', period: { preset: 'last-month' } })).toBe('?show=signed&sent=last-month');
		expect(contractsQueryString({ view: 'draft', order: '', period: { preset: 'last-month' } })).toBe('?show=draft');
	});

	it('asks the BFF for the two days, by GET and by POST', async () => {
		expect(practiceContractsPath('p1', { view: 'sent', order: '', isMine: false, doula: '', days: october })).toBe(
			'/api/practices/p1/contracts?view=sent&sentFrom=2026-10-01&sentTo=2026-10-31'
		);
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false, total: 0 }));
		await loadPracticeContracts(fetcher, 'p1', { view: 'sent', order: '', isMine: false, doula: '', search: 'q', days: october });
		expect(JSON.parse(fetcher.mock.calls[0]![1].body)).toMatchObject({ sentFrom: '2026-10-01', sentTo: '2026-10-31' });
	});

	it('says the period in the result line, and that Drafts no longer follow', () => {
		expect(summary({ days: october })).toBe('Showing 12 Contracts not signed sent in October 2026, longest wait first.');
		expect(summary({ shown: 0, total: 0, view: 'signed', days: october })).toBe('No Contract sent in October 2026 is signed.');
	});
});
