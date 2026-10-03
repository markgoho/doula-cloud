import { describe, expect, it } from 'vitest';
import type { Field } from './clientFieldTemplate.js';
import {
	DETAILS_STEPS,
	intakeSections,
	knownAsFrom,
	detailsSavedMessage,
	clientSavedMessage,
	intakeStepList,
	journeySteps,
	nextStepHref,
	previousStepHref,
	intakeOrigin,
	originQuery
} from './intakeJourney.js';

function field(partial: Partial<Field> & { id: string }): Field {
	return {
		type: 'short_text',
		label: partial.id,
		order: 0,
		archived: false,
		...partial
	};
}

const basePath = '/practices/p1/clients/new';

describe('intakeSections', () => {
	it('has no sections for a Practice that has added no fields', () => {
		expect(intakeSections([], 'Highland Midwifery')).toEqual([]);
	});

	it('heads fields before the first section header with the Practice name', () => {
		const sections = intakeSections(
			[field({ id: 'a', label: 'Allergies', order: 0 })],
			'Highland Midwifery'
		);

		expect(sections).toEqual([
			{ heading: 'Highland Midwifery', fields: [expect.objectContaining({ id: 'a' })] }
		]);
	});

	it('starts a new section at each section header', () => {
		const sections = intakeSections(
			[
				field({ id: 'h1', type: 'section_header', label: 'Health', order: 0 }),
				field({ id: 'a', label: 'Allergies', order: 1 }),
				field({ id: 'h2', type: 'section_header', label: 'Birth', order: 2 }),
				field({ id: 'b', label: 'Birth plan', order: 3 })
			],
			'Highland Midwifery'
		);

		expect(sections.map((section) => section.heading)).toEqual(['Health', 'Birth']);
		expect(sections[1].fields.map((entry) => entry.id)).toEqual(['b']);
	});

	// #432: a Practice that has added nothing gets five steps, not six
	// with an empty one -- which starts here, with a header that asks
	// nothing not becoming a section.
	it('drops a section that asks nothing', () => {
		const sections = intakeSections(
			[
				field({ id: 'h1', type: 'section_header', label: 'Health', order: 0 }),
				field({ id: 'h2', type: 'section_header', label: 'Birth', order: 1 }),
				field({ id: 'b', label: 'Birth plan', order: 2 })
			],
			'Highland Midwifery'
		);

		expect(sections.map((section) => section.heading)).toEqual(['Birth']);
	});

	it('leaves archived fields out of the form', () => {
		const sections = intakeSections(
			[field({ id: 'a', label: 'Allergies', order: 0, archived: true })],
			'Highland Midwifery'
		);

		expect(sections).toEqual([]);
	});

	it('reads the fields in the order the Practice arranged them, not array order', () => {
		const sections = intakeSections(
			[
				field({ id: 'b', label: 'Second', order: 1 }),
				field({ id: 'a', label: 'First', order: 0 })
			],
			'Highland Midwifery'
		);

		expect(sections[0].fields.map((entry) => entry.id)).toEqual(['a', 'b']);
	});
});

describe('knownAsFrom', () => {
	it.each([
		['her preferred name', { preferredName: ' Bex ', givenName: 'Rebecca' }, 'Bex'],
		['her given name', { preferredName: '', givenName: 'Rebecca' }, 'Rebecca'],
		['the Client before a name is given', { preferredName: '', givenName: '' }, 'the Client'],
		['the Client before a record is read', undefined, 'the Client']
	])('calls her %s', (_case, names, expected) => {
		expect(knownAsFrom(names)).toBe(expected);
	});
});

// #1710: what the screen a save lands on says.
describe('detailsSavedMessage and clientSavedMessage', () => {
	it('names her the way every other line does', () => {
		const names = { preferredName: 'Bex', givenName: 'Rebecca' };

		expect(detailsSavedMessage(names)).toBe("Bex's details saved.");
		expect(clientSavedMessage(names)).toBe('Bex saved as a Client.');
	});

	it('starts with a capital where she has no name to give', () => {
		const names = { preferredName: '', givenName: '' };

		expect(detailsSavedMessage(names)).toBe("The Client's details saved.");
	});
});

describe('intakeStepList', () => {
	// #1610: a Client's details journey asks everything but the name.
	it('is four steps for a details journey at a Practice with no added fields', () => {
		expect(intakeStepList([], DETAILS_STEPS).map((step) => step.slug)).toEqual([
			'date-of-birth',
			'email',
			'phone',
			'address'
		]);
	});

	it('is five steps for a Practice with no added fields', () => {
		const steps = intakeStepList([]);

		expect(steps.map((step) => step.label)).toEqual([
			'Name',
			'Date of birth',
			'Email address',
			'Phone number',
			'Address'
		]);
	});

	it('adds one step per Practice-named section', () => {
		const steps = intakeStepList([
			{ heading: 'Health', fields: [] },
			{ heading: 'Birth', fields: [] }
		]);

		expect(steps).toHaveLength(7);
		expect(steps[5]).toEqual({ id: 'section-0', label: 'Health', slug: 'sections/0' });
		expect(steps[6]).toEqual({ id: 'section-1', label: 'Birth', slug: 'sections/1' });
	});
});

describe('journeySteps', () => {
	const steps = intakeStepList([{ heading: 'Health', fields: [] }]);

	it('marks the step being asked as current and gives each its own href', () => {
		const rail = journeySteps(steps, basePath, 'email', ['name', 'date-of-birth']);

		expect(rail[2]).toEqual({
			label: 'Email address',
			href: `${basePath}/email`,
			status: 'current'
		});
		expect(rail[0].status).toBe('completed');
		expect(rail[3].status).toBe('todo');
		expect(rail[5].href).toBe(`${basePath}/sections/0`);
	});

	// The Change round trip: standing on step two with steps three to six
	// already answered has to report those as completed.
	it('reads completion from where the reader has been, not from position', () => {
		const rail = journeySteps(steps, basePath, 'name', ['email', 'address']);

		expect(rail.map((step) => step.status)).toEqual([
			'current',
			'todo',
			'completed',
			'todo',
			'completed',
			'todo'
		]);
	});

	it('has no current step when the reader is off the question sequence', () => {
		const rail = journeySteps(steps, basePath, undefined, ['name']);

		expect(rail.some((step) => step.status === 'current')).toBe(false);
	});
});

describe('nextStepHref', () => {
	const steps = intakeStepList([{ heading: 'Health', fields: [] }]);

	it('is the following step', () => {
		expect(nextStepHref(steps, basePath, 'name')).toBe(`${basePath}/date-of-birth`);
	});

	it('is the summary once there is no following step', () => {
		expect(nextStepHref(steps, basePath, 'section-0')).toBe(`${basePath}/check`);
	});
});

describe('previousStepHref', () => {
	const steps = intakeStepList([]);

	it('is the preceding step', () => {
		expect(previousStepHref(steps, basePath, 'email', '/search')).toBe(`${basePath}/date-of-birth`);
	});

	it('is the search that fronts intake from the first step', () => {
		expect(previousStepHref(steps, basePath, 'name', '/search')).toBe('/search');
	});
});

describe('intakeOrigin (#1609)', () => {
	it.each(['overview', 'clients'] as const)('reads %s as the screen that opened intake', (origin) => {
		const search = new URLSearchParams(originQuery(origin));

		expect(intakeOrigin(search)).toBe(origin);
	});

	it.each([
		['no origin at all', ''],
		['a Change round trip', 'from=check'],
		['an address', 'from=https://example.test/']
	])('names no screen for %s', (_case, query) => {
		expect(intakeOrigin(new URLSearchParams(query))).toBeUndefined();
	});
});
