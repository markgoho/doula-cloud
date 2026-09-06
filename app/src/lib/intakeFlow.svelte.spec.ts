import { describe, expect, it, vi } from 'vitest';
import { IntakeFlow, intakeFlow } from './intakeFlow.svelte.js';

function jsonResponse(body: unknown, status = 200): Response {
	return Response.json(body, {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function fetcherFor(fields: unknown[] = [], practiceName = 'Highland Midwifery') {
	return vi.fn(async (path: string) =>
		jsonResponse(path.endsWith('/session') ? { practiceName } : { fields })
	);
}

describe('IntakeFlow', () => {
	it('is idle until it is asked to load', () => {
		expect(new IntakeFlow().status).toBe('idle');
	});

	it('reads the Practice name and its Client Field Template', async () => {
		const flow = new IntakeFlow();

		await flow.load(fetcherFor([{ id: 'a', type: 'short_text', label: 'Allergies', order: 0, archived: false }]), 'p1');

		expect(flow.status).toBe('ready');
		expect(flow.practiceName).toBe('Highland Midwifery');
		expect(flow.sections).toEqual([
			{ heading: 'Highland Midwifery', fields: [expect.objectContaining({ id: 'a' })] }
		]);
		expect(flow.steps).toHaveLength(6);
	});

	// #432: a Practice that has added nothing gets five steps, not six
	// with an empty one.
	it('is five steps for a Practice that has added no fields', async () => {
		const flow = new IntakeFlow();

		await flow.load(fetcherFor(), 'p1');

		expect(flow.steps).toHaveLength(5);
	});

	it('does not ask again for a Practice it has already read', async () => {
		const flow = new IntakeFlow();
		const fetcher = fetcherFor();

		await flow.load(fetcher, 'p1');
		await flow.load(fetcher, 'p1');

		expect(fetcher).toHaveBeenCalledTimes(2);
	});

	it('reads again when the Practice changes', async () => {
		const flow = new IntakeFlow();
		const fetcher = fetcherFor();

		await flow.load(fetcher, 'p1');
		await flow.load(fetcher, 'p2');

		expect(fetcher).toHaveBeenCalledTimes(4);
		expect(flow.practiceId).toBe('p2');
	});

	it('says what refused it when the template cannot be read', async () => {
		const flow = new IntakeFlow();
		const fetcher = vi.fn(async (path: string) =>
			path.endsWith('/session')
				? jsonResponse({ practiceName: 'Highland Midwifery' })
				: new Response('the template is unreadable', { status: 500 })
		);

		await flow.load(fetcher, 'p1');

		expect(flow.status).toBe('error');
		expect(flow.loadError).toBeTruthy();
	});

	it('says what refused it when the Practice name cannot be read', async () => {
		const flow = new IntakeFlow();
		const fetcher = vi.fn(async () => new Response('', { status: 403 }));

		await flow.load(fetcher, 'p1');

		expect(flow.status).toBe('error');
	});

	it('reports a thrown non-Error as a failure to load', async () => {
		const flow = new IntakeFlow();
		const fetcher = vi.fn(() => {
			throw 'dropped';
		});

		await flow.load(fetcher, 'p1');

		expect(flow.loadError).toBe('Failed to load');
	});

	it('tries again after a failure', async () => {
		const flow = new IntakeFlow();
		const failing = vi.fn(async () => new Response('', { status: 500 }));
		await flow.load(failing, 'p1');

		await flow.load(fetcherFor(), 'p1');

		expect(flow.status).toBe('ready');
	});

	it('exports one flow the whole sequence shares', () => {
		expect(intakeFlow).toBeInstanceOf(IntakeFlow);
	});
});
