import { page } from '#lib/appState.svelte.js';
import { cannotBeReachedDraft } from '#lib/cannotBeReachedDraft.svelte.js';
import {
	STEP_LABELS,
	journeyAddressFrom,
	journeyHref,
	rosterHrefFor,
	type JourneyStepSlug
} from '#lib/cannotBeReached.js';
import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
import { canNarrowToMine, isOwnerOrAdmin } from '#lib/roles.js';
import type { PracticeSession } from '../../+layout.js';

export const JOURNEY_NAME = 'Recording a time you cannot be reached';

/**
 * What every page of the act reads about where it is: the Practice, who the
 * reader is, the roster to come back to, and the steps she walks. An Owner or
 * Admin is asked which Doula first; a Doula is only ever asking about herself.
 */
export class Journey {
	practiceId = $derived(page.params.practiceId!);
	session = $derived((page.data as { session: PracticeSession }).session);
	isOwnerView = $derived(isOwnerOrAdmin(this.session));
	address = $derived(journeyAddressFrom(page.url.searchParams));
	staffId = $derived(this.isOwnerView ? cannotBeReachedDraft.answers.staffId : this.session.staffId);
	firstSlug: JourneyStepSlug = $derived(this.isOwnerView ? 'doula' : 'when');
	slugs: Exclude<JourneyStepSlug, 'check'>[] = $derived(this.isOwnerView ? ['doula', 'when', 'cover'] : ['when', 'cover']);
	rosterHref = $derived(
		rosterHrefFor(
			this.practiceId,
			this.address.back,
			canNarrowToMine(this.session),
			this.isOwnerView ? undefined : { staffId: this.session.staffId }
		)
	);

	href(step: JourneyStepSlug, extra: Record<string, string> = {}): string {
		return journeyHref(this.practiceId, step, { back: this.address.back, doula: '' }, extra);
	}

	steps(current: JourneyStepSlug): JourneyStep[] {
		const at = current === 'check' ? this.slugs.length : this.slugs.indexOf(current);
		return this.slugs.map((slug, index) => ({
			label: STEP_LABELS[slug],
			href: this.href(slug),
			status: index < at ? 'completed' : (index === at ? 'current' : 'todo')
		}));
	}

	/**
	The page before this one: the step before it, or the roster at the first.
	*/
	backFrom(step: JourneyStepSlug): string {
		const order: JourneyStepSlug[] = [...this.slugs, 'check'];
		const before = order[order.indexOf(step) - 1];
		return before ? this.href(before) : this.rosterHref;
	}

	nextAfter(step: JourneyStepSlug): string {
		const order: JourneyStepSlug[] = [...this.slugs, 'check'];
		return this.href(order[order.indexOf(step) + 1]!);
	}
}
