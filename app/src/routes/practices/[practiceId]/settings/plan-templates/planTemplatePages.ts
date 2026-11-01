/**
 * What the Plan Templates list and its question pages share (#2193): the
 * two plan types, which one an address names, where the list is for one
 * of them, and what a save says when it lands back there.
 */
import { resolve } from '$app/paths';

export const PLAN_TYPES = [
	{ value: 'care_plan', label: 'Care Plan' },
	{ value: 'birth_plan', label: 'Birth Plan' }
] as const;

export type PlanType = (typeof PLAN_TYPES)[number]['value'];

export function isPlanType(value: string | undefined): value is PlanType {
	return PLAN_TYPES.some((planType) => planType.value === value);
}

/**
 * The plan type the list's `?plan=` names; the Care Plan otherwise.
 */
export function planTypeFromQuery(url: { readonly searchParams: Pick<URLSearchParams, 'get'> }): PlanType {
	const named = url.searchParams.get('plan') ?? undefined;
	return isPlanType(named) ? named : 'care_plan';
}

export function planLabel(planType: PlanType): string {
	return PLAN_TYPES.find((type) => type.value === planType)!.label;
}

/**
 * The list, on the plan type's tab.
 */
export function planTemplatesHref(practiceId: string, planType: PlanType): string {
	const list = resolve('/practices/[practiceId]/settings/plan-templates', { practiceId });
	return planType === 'care_plan' ? list : `${list}?plan=${planType}`;
}

/**
 * What a save says on the list (#2194's words, ADR-0001's snapshot): the
 * plan type saved, and which plans it reaches.
 */
export function planTemplateSavedMessage(planType: PlanType): string {
	const label = planLabel(planType);
	return `${label} questions saved. Every ${label} filled in from now on asks them, and a ${label} already filled in keeps the questions it had.`;
}

/**
 * What a removal says on the list (#2572): the question taken off, and the
 * dialog's own promise about plans already filled in.
 */
export function planTemplateRemovedMessage(planType: PlanType, question: string): string {
	const label = planLabel(planType);
	return `"${question}" removed. Every ${label} filled in from now on will not ask it, and a ${label} already filled in keeps it, with its answer.`;
}
