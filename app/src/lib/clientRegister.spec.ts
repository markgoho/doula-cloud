import { describe, expect, it } from 'vitest';
import {
	CARE_HEADING,
	NO_CARE_MESSAGE,
	contractStatusLabel,
	contractVoidedNotice,
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

	it('throws for a status this build has not labelled, rather than falling back to the raw value', () => {
		expect(() => engagementStatusLabel('postpartum')).toThrow(/no Client label/);
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

	it('throws for an unlabelled contract status', () => {
		expect(() => contractStatusLabel('archived')).toThrow(/no Client label/);
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
