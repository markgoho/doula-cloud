import { describe, expect, it } from 'vitest';
import {
	CARE_HEADING,
	NO_CARE_MESSAGE,
	contractStatusLabel,
	contractVoidedNotice,
	engagementLabel,
	engagementStatusLabel
} from './clientRegister';

describe('engagementStatusLabel', () => {
	it.each([
		['intake', 'Getting started'],
		['active', 'Ongoing'],
		['completed', 'Care ended']
	])('labels "%s" as "%s"', (status, label) => {
		expect(engagementStatusLabel(status)).toBe(label);
	});

	it('throws for a status this build has not labeled, rather than falling back to the raw value', () => {
		expect(() => engagementStatusLabel('postpartum')).toThrow(/no Client wording/);
	});
});

describe('engagementLabel', () => {
	it('carries the Practice name and when the Engagement began', () => {
		expect(
			engagementLabel({ practiceName: 'Riverside Doulas', createdAt: '2026-03-12T20:00:00Z' })
		).toBe('Riverside Doulas, started Mar 12, 2026');
	});

	// #310's own reason to exist: two Engagements at the same Practice, the
	// same status even, must still read apart -- ADR-0015 lifted the
	// one-Engagement-per-Practice assumption, and Practice name alone (or
	// Practice name plus status) can no longer be trusted to differ.
	it('tells two Engagements at the same Practice, in the same status, apart', () => {
		const first = engagementLabel({ practiceName: 'Riverside Doulas', createdAt: '2024-06-01T20:00:00Z' });
		const second = engagementLabel({ practiceName: 'Riverside Doulas', createdAt: '2026-03-12T20:00:00Z' });

		expect(first).not.toBe(second);
	});

	// The register is narrow on purpose (CONTEXT.md's Engagement entry):
	// kind, the birth outcome and the ending reason have no Client word at
	// all. The input type only holds what the register allows, so this
	// asserts none of those staff-only facts, even passed in by a caller
	// that ignores the type, reach the words a Client reads.
	it('never carries a staff-only fact, even if a caller ignores the input type', () => {
		const staffLeaking = {
			practiceName: 'Riverside Doulas',
			createdAt: '2026-03-12T20:00:00Z',
			kind: 'birth',
			birthOutcome: 'stillbirth',
			endingReason: 'family moved away'
		};

		const label = engagementLabel(staffLeaking);

		expect(label).not.toMatch(/birth|stillbirth|family moved away/i);
	});
});

describe('contractStatusLabel', () => {
	it.each([
		['draft', 'Being prepared'],
		['sent', 'Ready for your signature'],
		['signed', 'Signed'],
		['voided', 'No longer active']
	])('labels "%s" as "%s"', (status, label) => {
		expect(contractStatusLabel(status)).toBe(label);
	});

	it('throws for an unlabeled contract status', () => {
		expect(() => contractStatusLabel('archived')).toThrow(/no Client wording/);
	});
});

describe('contractVoidedNotice', () => {
	it('names the Practice and makes no claim about an Invoice', () => {
		expect(contractVoidedNotice('Rooted Birth Collective')).toBe('Rooted Birth Collective ended this Contract.');
	});
});

describe('the register nouns', () => {
	it('CARE_HEADING is the heading form of "my care"', () => {
		expect(CARE_HEADING).toBe('Your care');
	});

	it('NO_CARE_MESSAGE never says Engagement', () => {
		expect(NO_CARE_MESSAGE).not.toMatch(/engagement/i);
	});
});
