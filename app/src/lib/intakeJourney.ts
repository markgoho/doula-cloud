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
 *
 * ## One table, four readers
 *
 * `STRUCTURAL_STEPS` below is the only place a structural step's slug,
 * its name and the columns it asks are written down. The rail reads it
 * for labels and hrefs, `intakeAnswers.ts` reads it for the summary's
 * section headings, rows and Change links, `proposedChanges.ts` reads it for
 * what to call a proposed change, and each route names its own step by
 * the same slug. They were three separate lists until a review of this
 * ticket pointed out that "Email address" was typed out in two of them
 * and the slug `sections/N` composed in three.
 */

import type { ClientMatch } from './client.js';
import type { Field } from './clientFieldTemplate.js';
import type { IntakeAnswers } from './intakeDraft.svelte.js';
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

/**
 * A structural column, keyed on the ones that are a string on both the
 * draft and a match -- so the two readers that compare them need no
 * runtime guard for a shape the types already rule out.
 */
export type TextColumn = {
	[Key in keyof IntakeAnswers & keyof ClientMatch]: IntakeAnswers[Key] extends string
		? ClientMatch[Key] extends string
			? Key
			: never
		: never;
}[keyof IntakeAnswers & keyof ClientMatch];

export interface StructuralQuestion {
	key: TextColumn;
	label: string;
	/** What a Change link on this row changes, in words -- GOV.UK's
	 * visually-hidden text, so a screen-reader user listing the page's
	 * links hears more than "Change" eleven times. */
	changes: string;
}

export interface StructuralStep {
	slug: string;
	label: string;
	questions: StructuralQuestion[];
}

/** The five structural steps, in the order #466's body asks them. The
 * name comes first so every page after it can say the Client's own
 * name (#463's no-pronoun rule). */
export const STRUCTURAL_STEPS: StructuralStep[] = [
	{
		slug: 'name',
		label: 'Name',
		questions: [
			{ key: 'givenName', label: 'Given name', changes: 'given name' },
			{ key: 'familyName', label: 'Family name', changes: 'family name' },
			{ key: 'preferredName', label: 'Preferred name', changes: 'preferred name' }
		]
	},
	{
		slug: 'date-of-birth',
		label: 'Date of birth',
		questions: [{ key: 'dateOfBirth', label: 'Date of birth', changes: 'date of birth' }]
	},
	{
		slug: 'email',
		label: 'Email address',
		questions: [{ key: 'email', label: 'Email address', changes: 'email address' }]
	},
	{
		slug: 'phone',
		label: 'Phone number',
		questions: [{ key: 'phone', label: 'Phone number', changes: 'phone number' }]
	},
	{
		slug: 'address',
		label: 'Address',
		questions: [
			{ key: 'addressLine1', label: 'Address line 1', changes: 'address line 1' },
			{ key: 'addressLine2', label: 'Address line 2', changes: 'address line 2' },
			{ key: 'addressLocality', label: 'City', changes: 'city' },
			{ key: 'addressRegion', label: 'State', changes: 'state' },
			{ key: 'addressPostalCode', label: 'ZIP code', changes: 'ZIP code' }
		]
	}
];

/** The structural steps a Client's details journey asks (#1610): every
 * one but the name, which intake asked and the Edit form corrects. */
export const DETAILS_STEPS: StructuralStep[] = STRUCTURAL_STEPS.filter(
	(step) => step.slug !== 'name'
);

/** Every structural column a reader types, flat -- what `proposedChanges.ts`
 * walks to say what a save to an existing Client would change. */
export const STRUCTURAL_QUESTIONS: StructuralQuestion[] = STRUCTURAL_STEPS.flatMap(
	(step) => step.questions
);

/*
 * The query a Change link carries, so a question page reached from the
 * summary sends Continue back there instead of on. Written once: it is
 * read as a name and a value on the way in and composed as a string on
 * the way out, and those were two literals that had to agree.
 */
export const CHANGE_PARAMETER = 'from';
export const CHANGE_VALUE = 'check';
export const CHANGE_QUERY = `${CHANGE_PARAMETER}=${CHANGE_VALUE}`;

/**
 * The screens that open the name question directly, with no search in
 * front of it (#1609, ADR-0017's amendment of 2026-10-02): the empty
 * Practice's overview, and the Clients list of a Practice that holds no
 * Client record. Anywhere else, intake is reached from the search.
 */
export type IntakeOrigin = 'overview' | 'clients';

const INTAKE_ORIGINS: readonly IntakeOrigin[] = ['overview', 'clients'];

/** The query a link that opens intake directly carries, so the name
 * question's Back goes to the screen she came from. The same `from` a
 * Change link uses, because it answers the same question. */
export function originQuery(origin: IntakeOrigin): string {
	return `${CHANGE_PARAMETER}=${origin}`;
}

/** Which screen opened intake, read off the query string. Only the two
 * named values are accepted, so the URL can name a screen but can never
 * name an address for Back to go to. */
export function intakeOrigin(search: Pick<URLSearchParams, 'get'>): IntakeOrigin | undefined {
	const value = search.get(CHANGE_PARAMETER);
	return INTAKE_ORIGINS.find((origin) => origin === value);
}

/**
 * The given name a search's Name value carries onto the name question
 * (#498), or undefined when it carries none. One word carries; a value
 * with any white space inside it carries nothing, because a name is
 * never split on a space -- GOV.UK, the reference under ADR-0021, says
 * not to guess a name's structure (#1716). The person types it into the
 * fields she means.
 */
export function carriedGivenName(value: string | null | undefined): string | undefined {
	const name = value?.trim();
	return name && !/\s/u.test(name) ? name : undefined;
}

/**
 * The name of two or more words a search typed, which `carriedGivenName`
 * carries into no field, or undefined when there is none. The name
 * question shows it, so the person does not have to remember what she
 * typed one screen earlier (#1758). It fills no field and is not saved.
 */
export function searchedName(value: string | null | undefined): string | undefined {
	const name = value?.trim();
	return name && !carriedGivenName(name) ? name : undefined;
}

/**
 * The two names every line about her is made from.
 */
export interface ClientNames {
	preferredName: string;
	givenName: string;
}

/**
 * What the Client is called on a question page: #463's rule with no
 * pronoun in it. Her preferred name if she has one, her given name
 * otherwise, and the domain noun before either is known. Both journeys
 * ask it, each of its own names (#1610).
 */
export function knownAsFrom(names: ClientNames | undefined): string {
	return names?.preferredName.trim() || names?.givenName.trim() || 'the Client';
}

function capitalizeFirst(text: string): string {
	return text.charAt(0).toUpperCase() + text.slice(1);
}

/**
 * What the screen a save of her details lands on says (#1710): the
 * details journey, the edit form and the duplicate pages.
 */
export function detailsSavedMessage(names: ClientNames): string {
	return capitalizeFirst(`${knownAsFrom(names)}'s details saved.`);
}

/**
 * What the screen intake's save lands on says (#1710). Intake always has
 * a given name by then, so she is never "the Client" here.
 */
export function clientSavedMessage(names: ClientNames): string {
	return `${knownAsFrom(names)} saved as a Client.`;
}

/** Every step's own identity: the structural slugs, plus `section-0`,
 * `section-1` and so on for the Practice's own pages. */
export type StepId = string;

export interface IntakeStep {
	id: StepId;
	label: string;
	slug: string;
}

/** The slug a Practice-named section's page lives at. Composed here so
 * the route, the rail and the summary's Change links cannot disagree
 * about it. */
export function sectionSlug(index: number): string {
	return `sections/${index}`;
}

/**
The id a Practice-named section's step is known by.
*/
export function sectionStepId(index: number): StepId {
	return `section-${index}`;
}

/** The whole journey as identities, before any reader has been anywhere
 * -- the list the rail, the Change links and the Continue button all
 * index into. */
export function intakeStepList(
	sections: IntakeSection[],
	structuralSteps: readonly StructuralStep[] = STRUCTURAL_STEPS
): IntakeStep[] {
	const structural = structuralSteps.map((step) => ({
		id: step.slug,
		label: step.label,
		slug: step.slug
	}));
	const practiceDefined = sections.map((section, index) => ({
		id: sectionStepId(index),
		label: section.heading,
		slug: sectionSlug(index)
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
 * more. A Change link's round trip overrides this -- see `intake.ts`'s
 * `checkOr`. */
export function nextStepHref(steps: IntakeStep[], basePath: string, currentId: StepId): string {
	const index = steps.findIndex((step) => step.id === currentId);
	const next = steps[index + 1];
	return next ? `${basePath}/${next.slug}` : `${basePath}/check`;
}

/** Where the back link goes: the previous step, or the screen the
 * journey was opened from once there is no previous one. */
export function previousStepHref(
	steps: IntakeStep[],
	basePath: string,
	currentId: StepId,
	exitHref: string
): string {
	const index = steps.findIndex((step) => step.id === currentId);
	if (index <= 0) return exitHref;
	return `${basePath}/${steps[index - 1]!.slug}`;
}
