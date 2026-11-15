import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import { load, type ContractsPageData } from './+page.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async (path: string) =>
		path.endsWith('/staff')
			? jsonResponse(
					{
						members: [
							{ staffId: 'staff-1', name: 'Marisol Terrazas', roles: ['doula'], employmentType: 'employee' },
							{ staffId: 'staff-2', name: 'Dee Whitlock', roles: ['admin'], employmentType: 'employee' }
						]
					},
					200
				)
			: jsonResponse(body, status)
	);
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

const emptyPage = { items: [], hasMore: false, total: 0 };
const asOwner = { roles: ['owner'], isContractor: false, hasOtherDoulas: true };
const asDoula = { roles: ['doula'], isContractor: false, hasOtherDoulas: true };
const asContractor = { roles: ['doula'], isContractor: true, hasOtherDoulas: true };

async function run(session: object, query = ''): Promise<ContractsPageData> {
	return (await load({
		depends: () => {},
		parent: async () => ({ session }),
		params: { practiceId: 'practice-1' },
		url: new URL(`https://example.test/practices/practice-1/contracts${query}`)
	} as Parameters<typeof load>[0])) as ContractsPageData;
}

const calledPaths = (fetchMock: ReturnType<typeof vi.fn>) => fetchMock.mock.calls.map(([path]) => path);

describe('contracts/+page.ts load', () => {
	it('reads the Contracts and the void requests for an Owner, and the roster', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const data = await run(asOwner);

		expect(calledPaths(fetchMock)).toEqual(
			expect.arrayContaining([
				'/api/practices/practice-1/contracts',
				'/api/practices/practice-1/contracts/void-requests',
				'/api/practices/practice-1/staff'
			])
		);
		await expect(loadedValue(data)).resolves.toEqual({
			...emptyPage,
			view: 'not-signed',
			order: '',
			isMine: false,
			doula: '',
			now: expect.any(Date)
		});
		await expect(loadedValue({ read: data.voidRequests })).resolves.toEqual(emptyPage);
		await expect(data.doulas).resolves.toEqual([{ value: 'staff-1', label: 'Marisol Terrazas' }]);
	});

	it.each([
		['an employed Doula', asDoula],
		['a contractor Doula', asContractor]
	])('makes no void requests read and no roster read for %s', async (_who, session) => {
		const { fetchMock } = setup(200, emptyPage);

		const data = await run(session);

		expect(data.voidRequests).toBeUndefined();
		await expect(data.doulas).resolves.toEqual([]);
		expect(calledPaths(fetchMock)).toEqual(['/api/practices/practice-1/contracts']);
	});

	it.each([
		['show=sent', '?view=sent', 'sent', ''],
		['show=signed&order=oldest', '?view=signed&order=oldest', 'signed', 'oldest'],
		['show=signed&order=newest', '?view=signed', 'signed', ''],
		['show=all&order=client', '?view=all&order=client', 'all', 'client'],
		['show=bogus&order=bogus', '', 'not-signed', ''],
		['show=not-signed', '', 'not-signed', '']
	])('reads %s into the BFF read and the view and order the screen holds', async (query, path, view, order) => {
		const { fetchMock } = setup(200, emptyPage);

		const data = await run(asOwner, `?${query}`);

		expect(calledPaths(fetchMock)).toContain(`/api/practices/practice-1/contracts${path}`);
		await expect(loadedValue(data)).resolves.toMatchObject({ view, order });
	});

	it('carries Mine for a Doula who can narrow, and ignores it for one who cannot', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const mine = await run(asDoula, '?mine=true&doula=staff-1');
		const notMine = await run(asContractor, '?mine=true');

		expect(calledPaths(fetchMock)).toContain('/api/practices/practice-1/contracts?mine=true&doula=staff-1');
		await expect(loadedValue(mine)).resolves.toMatchObject({ isMine: true, doula: 'staff-1' });
		await expect(loadedValue(notMine)).resolves.toMatchObject({ isMine: false });
	});

	it('hands a refusal to the screen rather than throwing mid-load, and leaves the other read alone', async () => {
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		const data = await run(asOwner);

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 403 } });
		await expect(settledRead({ read: data.voidRequests })).resolves.toMatchObject({ isLoaded: false, response: { status: 403 } });
	});

	it('answers an unreadable roster with no Doulas rather than a failed load', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async (path: string) => (path.endsWith('/staff') ? jsonResponse({}, 403) : jsonResponse(emptyPage, 200)))
		);

		const data = await run(asOwner);

		await expect(data.doulas).resolves.toEqual([]);
	});

	it('answers a roster that did not arrive with no Doulas', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async (path: string) => {
				if (path.endsWith('/staff')) throw new Error('offline');
				return jsonResponse(emptyPage, 200);
			})
		);

		const data = await run(asOwner);

		await expect(data.doulas).resolves.toEqual([]);
	});
});
