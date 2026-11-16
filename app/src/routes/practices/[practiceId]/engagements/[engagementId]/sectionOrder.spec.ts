import { describe, expect, it } from 'vitest';
import { dayIn } from '#lib/zonedTime.js';
import { carePhaseOf, sectionOrder, type SectionHeading, type SectionOrderInput } from './sectionOrder.js';

const ZONE = 'America/New_York';
const NOW = new Date('2026-11-15T15:00:00Z');

function base(overrides: Partial<SectionOrderInput>): SectionOrderInput {
	return {
		status: 'active',
		kind: 'birth',
		today: dayIn(NOW, ZONE),
		hasDoula: true,
		hasVisits: true,
		hasMessages: true,
		hasInvoices: true,
		hasOffers: true,
		hasContract: true,
		hasCarePlan: true,
		hasBirthPlan: true,
		isOffersVisible: true,
		isDoulaFeesVisible: false,
		hasStartNote: false,
		...overrides
	};
}

describe('a birth with no Doula on it (#2755 closing review)', () => {
	it('is not on call though the grant-day rule opens its days today', () => {
		const window = { from: dayIn(NOW, ZONE), to: '2027-05-30' };

		expect(carePhaseOf(base({ status: 'intake', hasDoula: false, onCallWindow: window }))).toBe('intake');
		expect(carePhaseOf(base({ hasDoula: false, onCallWindow: window }))).toBe('before-on-call');
		expect(carePhaseOf(base({ hasDoula: true, onCallWindow: window }))).toBe('on-call');
	});
});

describe('sectionOrder', () => {
	const cases: { name: string; input: SectionOrderInput; phase: string; order: SectionHeading[] }[] = [
		{
			name: 'intake (Grace Whitfield: Contract sent, nothing else begun)',
			input: base({
				status: 'intake',
				dueDate: '2027-05-30',
				onCallWindow: { from: '2027-05-16', to: '2027-06-13' },
				hasVisits: false,
				hasInvoices: false,
				hasOffers: false,
				hasCarePlan: false,
				hasBirthPlan: false,
				hasMessages: true
			}),
			phase: 'intake',
			order: ['Contract', 'Messages', 'Visits', 'Invoices', 'Birth Plan', 'Care Plan', 'On call', 'Offers', 'Birth outcome', 'Activity']
		},
		{
			name: 'before on call (Tamsin Reyes: no Doula yet, days begin in 25 days)',
			input: base({
				dueDate: '2026-12-10',
				onCallWindow: { from: '2026-12-10', to: '2026-12-24' },
				hasVisits: false,
				hasMessages: false,
				hasInvoices: false,
				hasOffers: false,
				hasCarePlan: false,
				hasBirthPlan: false
			}),
			phase: 'before-on-call',
			order: ['Contract', 'Visits', 'Birth Plan', 'Messages', 'Care Plan', 'Invoices', 'On call', 'Offers', 'Birth outcome', 'Activity']
		},
		{
			name: 'on call (Camille Boyd: due in 20 days, days began)',
			input: base({
				dueDate: '2026-12-05',
				onCallWindow: { from: '2026-11-14', to: '2026-12-19' },
				hasOffers: false,
				isDoulaFeesVisible: true,
				hasStartNote: true
			}),
			phase: 'on-call',
			order: ['On call', 'Birth Plan', 'Visits', 'Messages', 'Care Plan', 'Contract', 'Invoices', 'Doula fees', 'Offers', 'Birth outcome', 'Note when work started', 'Activity']
		},
		{
			name: 'on call from the due date: Birth outcome second (Camille, due today)',
			input: base({
				dueDate: '2026-11-15',
				onCallWindow: { from: '2026-10-25', to: '2026-11-29' },
				hasOffers: false
			}),
			phase: 'on-call',
			order: ['On call', 'Birth outcome', 'Birth Plan', 'Visits', 'Messages', 'Care Plan', 'Contract', 'Invoices', 'Offers', 'Activity']
		},
		{
			name: 'postpartum (Imani Brooks: live birth, Invoice overdue)',
			input: base({ birthOutcome: 'live_birth', hasBirthPlan: false, hasOffers: false }),
			phase: 'postpartum',
			order: ['Visits', 'Care Plan', 'Messages', 'Invoices', 'Birth outcome', 'Contract', 'Birth Plan', 'Offers', 'On call', 'Activity']
		},
		{
			name: 'completed (Jamie Bishop: birth outcome, finished)',
			input: base({ status: 'completed', birthOutcome: 'live_birth', hasOffers: false }),
			phase: 'completed',
			order: ['Invoices', 'Birth outcome', 'Visits', 'Care Plan', 'Birth Plan', 'Contract', 'Messages', 'Offers', 'On call', 'Activity']
		},
		{
			name: 'after a loss (Mira Castellanos: no Birth Plan nag, On call one line)',
			input: base({ birthOutcome: 'loss', hasBirthPlan: false, hasInvoices: false, hasOffers: false }),
			phase: 'loss',
			order: ['Messages', 'Visits', 'Care Plan', 'Birth outcome', 'Contract', 'Invoices', 'Offers', 'Birth Plan', 'On call', 'Activity']
		}
	];

	for (const { name, input, phase, order } of cases) {
		it(name, () => {
			expect(carePhaseOf(input)).toBe(phase);
			expect(sectionOrder(input)).toEqual(order);
		});
	}

	it('leaves Offers and Doula fees to the readers who may have them', () => {
		const order = sectionOrder(base({ isOffersVisible: false, isDoulaFeesVisible: false, dueDate: '2026-12-05', onCallWindow: { from: '2026-11-14', to: '2026-12-19' } }));
		expect(order).not.toContain('Offers');
		expect(order).not.toContain('Doula fees');
	});

	it('keeps Activity last in every phase, and has no Details section (the template draws that)', () => {
		for (const { input } of cases) {
			const order = sectionOrder(input);
			expect(order.at(-1)).toBe('Activity');
			expect(order).not.toContain('Details' as SectionHeading);
		}
	});

	it('sinks On call below the filled sections until its on-call days begin', () => {
		const before = sectionOrder(base({ dueDate: '2026-12-10', onCallWindow: { from: '2026-12-10', to: '2026-12-24' } }));
		expect(before.indexOf('On call')).toBeGreaterThan(before.indexOf('Invoices'));
		const during = sectionOrder(base({ dueDate: '2026-12-05', onCallWindow: { from: '2026-11-14', to: '2026-12-19' } }));
		expect(during[0]).toBe('On call');
	});

	it('reads today in the Practice zone: the same instant is before on call in New York and on call in Tokyo', () => {
		const instant = new Date('2026-11-16T03:00:00Z');
		const window = { from: '2026-11-16', to: '2026-12-20' };
		const inNewYork = base({ today: dayIn(instant, 'America/New_York'), dueDate: '2026-12-06', onCallWindow: window });
		const inTokyo = base({ today: dayIn(instant, 'Asia/Tokyo'), dueDate: '2026-12-06', onCallWindow: window });
		expect(carePhaseOf(inNewYork)).toBe('before-on-call');
		expect(carePhaseOf(inTokyo)).toBe('on-call');
	});

	it('treats active birth care with no due date as before on call', () => {
		expect(carePhaseOf(base({ dueDate: undefined, onCallWindow: undefined }))).toBe('before-on-call');
	});

	it('treats postpartum care as postpartum, and an unknown outcome as postpartum', () => {
		expect(carePhaseOf(base({ kind: 'postpartum' }))).toBe('postpartum');
		expect(carePhaseOf(base({ birthOutcome: 'unknown' }))).toBe('postpartum');
	});
});
