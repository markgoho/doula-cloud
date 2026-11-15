import type { AwaitingVoidRequest, ContractListItem, ContractsListData, VoidRequestsPage } from '#lib/contract.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS, TYPICAL_DOULAS } from '../../../typicalPilot.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './ContractsScreen.svelte';

export const NOW = new Date('2026-11-14T15:00:00Z');

const longName = 'Anne-Marie Ochieng-Whitfield';
const urlName = 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

export const items: ContractListItem[] = [
	{
		contractId: 'contract-1',
		engagementId: 'eng-1',
		clientId: 'client-1',
		clientName: longName,
		engagementKind: 'birth',
		status: 'draft',
		steppedOn: '2026-11-08',
		dueDate: '2027-01-13',
		doulaNames: ['Marisol Terrazas'],
		amountCents: 150_000,
		hasSignedPdf: false,
		voidRequested: false
	},
	{
		contractId: 'contract-2',
		engagementId: 'eng-2',
		clientId: 'client-2',
		clientName: urlName,
		engagementKind: 'postpartum',
		status: 'sent',
		steppedOn: '2026-11-02',
		daysWaiting: 12,
		sentByName: 'Dee Whitlock',
		doulaNames: ['Kimiko Nakashima', 'Joss Adeyemi'],
		amountCents: 95_000,
		hasSignedPdf: false,
		voidRequested: false
	},
	{
		contractId: 'contract-3',
		engagementId: 'eng-3',
		clientId: 'client-3',
		clientName: 'Quinn Bishop',
		engagementKind: 'birth',
		status: 'signed',
		steppedOn: '2025-12-30',
		sentByName: 'Dee Whitlock',
		pregnancyEndedOn: '2026-10-30',
		birthOutcome: 'live_birth',
		doulaNames: ['Priya Raman'],
		amountCents: 150_000,
		hasSignedPdf: true,
		voidRequested: true
	},
	{
		contractId: 'contract-4',
		engagementId: 'eng-3',
		clientId: 'client-3',
		clientName: 'Quinn Bishop',
		engagementKind: 'birth',
		status: 'voided',
		steppedOn: '2026-11-09',
		sentByName: 'Renata Alvarez',
		doulaNames: [],
		amountCents: 150_000,
		hasSignedPdf: true,
		voidRequested: false
	},
	{
		contractId: 'contract-5',
		engagementId: 'eng-5',
		clientId: 'client-5',
		clientName: 'Rowan Okafor',
		engagementKind: 'birth',
		status: 'sent',
		steppedOn: '2026-11-14',
		daysWaiting: 0,
		sentByName: 'Renata Alvarez',
		pregnancyEndedOn: '2026-10-01',
		birthOutcome: 'loss',
		amountCents: 150_000,
		hasSignedPdf: false,
		voidRequested: false
	}
];

export const voidRequests: AwaitingVoidRequest[] = [
	{
		requestId: 'request-1',
		contractId: 'contract-3',
		engagementId: 'eng-3',
		clientId: 'client-3',
		clientName: 'Priya Okonkwo-Fitzgerald',
		requestedByName: 'Jamie Doula',
		reason: 'https://portal.highland-midwifery-group.example.org/notes/2027/reschedule?source=client-call',
		daysAgo: 11,
		createdAt: '2026-11-03T15:00:00Z'
	},
	{
		requestId: 'request-2',
		contractId: 'contract-2',
		engagementId: 'eng-2',
		clientId: 'client-2',
		clientName: 'Jamie Dunmore',
		requestedByName: 'Joss Adeyemi',
		reason: 'Jamie changed from birth to postpartum-only care.',
		daysAgo: 0,
		createdAt: '2026-11-14T10:00:00Z'
	}
];

export const queue: VoidRequestsPage = { items: voidRequests, hasMore: false, total: 2 };

export const data: ContractsListData = {
	items,
	hasMore: false,
	total: items.length,
	view: 'all',
	order: '',
	isMine: false,
	doula: '',
	now: NOW
};

export const doulas = [
	{ value: 'staff-1', label: 'Marisol Terrazas' },
	{ value: 'staff-2', label: 'Priya Raman' }
];

const owner = practiceSession(['owner']);
const properties = { data, isQueueExpected: true, queue, doulas, isRosterLanded: true };

export const typical: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, with typical pilot data',
	typical: true,
	url: 'https://example.test/practices/practice-1/contracts',
	props: {
		...properties,
		data: {
			...data,
			view: 'not-signed',
			items: [4, 5, 6].map((index) => {
				const person = nth(TYPICAL_CLIENTS, index);
				return {
					contractId: `contract-${index}`,
					engagementId: `eng-${index}`,
					clientId: person.clientId,
					clientName: person.name,
					engagementKind: 'birth' as const,
					status: index === 4 ? ('draft' as const) : ('sent' as const),
					steppedOn: `2026-10-0${index}`,
					...(index !== 4 && { daysWaiting: 40 + index, sentByName: nth(TYPICAL_DOULAS, 2).name }),
					dueDate: '2027-01-13',
					doulaNames: [nth(TYPICAL_DOULAS, 1).name],
					amountCents: 150_000,
					hasSignedPdf: false,
					voidRequested: false
				};
			}),
			total: 3
		},
		queue: {
			items: [
				{
					requestId: 'request-1',
					contractId: 'contract-4',
					engagementId: 'eng-4',
					clientId: nth(TYPICAL_CLIENTS, 7).clientId,
					clientName: nth(TYPICAL_CLIENTS, 7).name,
					requestedByName: nth(TYPICAL_DOULAS, 2).name,
					reason: 'Her due date moved, so the fee changed',
					daysAgo: 4,
					createdAt: '2026-10-08T15:00:00Z'
				}
			],
			hasMore: false,
			total: 1
		}
	}
};

export const loading: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, while its read runs',
	props: { isQueueExpected: true, doulas, isRosterLanded: true }
};

export const queueLoading: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, while the void requests read runs',
	props: { data, isQueueExpected: true, doulas, isRosterLanded: true }
};

export const queueFailed: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, with its void requests read failed',
	props: { ...properties, queue: undefined, hasQueueFailed: true }
};

export const noRequests: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, with no void request waiting',
	props: { ...properties, queue: { items: [], hasMore: false, total: 0 } }
};

export const readFailed: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, with its read failed',
	props: { ...properties, data: undefined, hasReadFailed: true }
};

export const contractorItems = items.map((contract) => ({ ...contract, amountCents: undefined, hasSignedPdf: false }));

export const asContractor: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, as a contractor Doula',
	props: { data: { ...data, items: contractorItems }, isQueueExpected: false, doulas: [], isRosterLanded: true },
	pageData: { session: { ...practiceSession(['doula']).session, isContractor: true } }
};

export const asDoula: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts list, as an employed Doula on Mine',
	url: 'https://example.test/practices/practice-1/contracts?show=all&mine=true',
	props: { data: { ...data, isMine: true }, isQueueExpected: false, doulas: [], isRosterLanded: true },
	pageData: { session: { ...practiceSession(['doula']).session, hasOtherDoulas: true } }
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide Contracts list',
	variants: [typical, loading, queueLoading, queueFailed, noRequests, readFailed, asContractor, asDoula],
	component: Page,
	params: { practiceId: 'practice-1' },
	pageData: owner,
	url: 'https://example.test/practices/practice-1/contracts?show=all',
	props: properties,
	readyText: 'Contracts'
};
