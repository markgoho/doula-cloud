/**
 * The shape of intake, computed rather than written down (#466).
 *
 * Two derivations live here, both pure, both off the Practice's own
 * Client Field Template:
 *
 * 1. **Sections.** A section is the run of active, askable fields
 *    between two `section_header` entries. Fields that sit before the
 *    first header are a section too, headed with the Practice's own
 *    name -- #466's un-headed-fields rule.
 * 2. **Steps.** The five structural questions plus one step per section.
 *    #432 requires that a Practice which has added nothing gets five
 *    steps rather than six with an empty one, which is why a section
 *    with no askable field in it is not a step at all.
 *
 * `StepRail` takes `JourneyStep[]` as data and never derives its own --
 * see its module comment. This is the route's half of that contract, in
 * a module rather than in a `.svelte` file so it can be tested directly.
 */

import type { Field } from './clientFieldTemplate.js';
import type { JourneyStep } from './components/organisms/StepRail.svelte';

/** One page of the Practice-defined layer: a heading a reader sees and
 * the fields asked under it. */
export interface IntakeSection {
	heading: string;
	fields: Field[];
}

const SECTION_HEADER = 'section_header';

/**
 * Groups a Practice's template into the pages intake asks it as.
 *
 * Archived fields are gone from the form -- ADR-0017 archives rather
 * than deletes so a Client keeps a value under an old field, but nobody
 * is asked for one again. `order` is the server's own position and is
 * what the array is sorted by, so a template edited out of array order
 * still reads in the order an Owner arranged it.
 */
export function intakeSections(fields: Field[], practiceName: string): IntakeSection[] {
	const active = fields
		.filter((field) => !field.archived)
		.toSorted((left, right) => left.order - right.order);

	const sections: IntakeSection[] = [];
	let current: IntakeSection = { heading: practiceName, fields: [] };

	for (const field of active) {
		if (field.type === SECTION_HEADER) {
			if (current.fields.length > 0) sections.push(current);
			current = { heading: field.label, fields: [] };
			continue;
		}
		current.fields.push(field);
	}
	if (current.fields.length > 0) sections.push(current);

	return sections;
}

/** The five structural steps, in the order #466's body asks them. The
 * name comes first so every page after it can say the Client's own
 * name (#463's no-pronoun rule). */
export const STRUCTURAL_STEPS = [
	{ slug: 'name', label: 'Name' },
	{ slug: 'date-of-birth', label: 'Date of birth' },
	{ slug: 'email', label: 'Email address' },
	{ slug: 'phone', label: 'Phone number' },
	{ slug: 'address', label: 'Address' }
] as const;

/** Every step's own identity: the structural slugs, plus `section-0`,
 * `section-1` and so on for the Practice's own pages. */
export type StepId = string;

export interface IntakeStep {
	id: StepId;
	label: string;
	slug: string;
}

/** The whole journey as identities, before any reader has been anywhere
 * -- the list the rail, the Change links and the Continue button all
 * index into. */
export function intakeStepList(sections: IntakeSection[]): IntakeStep[] {
	const structural = STRUCTURAL_STEPS.map((step) => ({
		id: step.slug,
		label: step.label,
		slug: step.slug
	}));
	const practiceDefined = sections.map((section, index) => ({
		id: `section-${index}`,
		label: section.heading,
		slug: `sections/${index}`
	}));
	return [...structural, ...practiceDefined];
}

/**
 * The rail's data.
 *
 * `visited` is what makes a step "completed" rather than its position:
 * a reader who jumped back from check-answers with a Change link is
 * standing on step two with steps three to six already answered, and
 * position alone would report those as not started.
 */
export function journeySteps(
	steps: IntakeStep[],
	basePath: string,
	currentId: StepId | undefined,
	visited: readonly StepId[]
): JourneyStep[] {
	return steps.map((step) => ({
		label: step.label,
		href: `${basePath}/${step.slug}`,
		status: step.id === currentId ? 'current' : (visited.includes(step.id) ? 'completed' : 'todo')
	}));
}

/** Where Continue goes: the next step, or the summary once there are no
 * more. A Change link's round trip overrides this -- see
 * `intake.ts`'s `continueHref`. */
export function nextStepHref(
	steps: IntakeStep[],
	basePath: string,
	currentId: StepId
): string {
	const index = steps.findIndex((step) => step.id === currentId);
	const next = steps[index + 1];
	return next ? `${basePath}/${next.slug}` : `${basePath}/check`;
}

/** Where the back link goes: the previous step, or the search that
 * fronts intake once there is no previous one (ADR-0017 makes search
 * the only door in). */
export function previousStepHref(
	steps: IntakeStep[],
	basePath: string,
	currentId: StepId,
	searchHref: string
): string {
	const index = steps.findIndex((step) => step.id === currentId);
	if (index <= 0) return searchHref;
	return `${basePath}/${steps[index - 1]!.slug}`;
}
