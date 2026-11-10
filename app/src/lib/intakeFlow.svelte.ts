/**
 * The shape intake takes at one Practice (#466): its own name, and the
 * Client Field Template that decides how many steps the journey has.
 *
 * Loaded once by the `clients/new` layout and read by every step, rather
 * than once per page: the sequence is six or more navigations, and
 * re-asking for the same template on each of them would put a skeleton
 * in front of a reader who is three questions in. The layout shows the
 * skeleton the first time and nothing after it -- ADR-0020's rule is
 * that a wait is disclosed, not that a wait is manufactured.
 *
 * Separate from `intakeDraft.svelte.ts` because these are two different
 * lifetimes: the draft is what one reader typed and is cleared on save,
 * this is what the Practice has configured and outlives every Client
 * added under it.
 */

import type { Fetcher } from './fetcher.js';

import { loadTemplate, type Field } from './clientFieldTemplate.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { intakeSections, intakeStepList, type IntakeSection, type IntakeStep } from './intakeJourney.js';
import { PageRead } from './pageRead.svelte.js';

async function loadPracticeName(fetcher: Fetcher, practiceId: string): Promise<string> {
	const response = await fetcher(`/api/practices/${practiceId}/session`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { practiceName: string } = await response.json();
	return body.practiceName;
}

interface PracticeShape {
	practiceName: string;
	fields: Field[];
}

export class IntakeFlow {
	practiceId = $state('');
	/**
	The Practice's name and template, read as one: loading, loaded or failed.
	*/
	readonly read = new PageRead<PracticeShape>();
	practiceName = $derived(this.read.value?.practiceName ?? '');
	fields = $derived<Field[]>(this.read.value?.fields ?? []);

	sections = $derived<IntakeSection[]>(intakeSections(this.fields, this.practiceName));
	steps = $derived<IntakeStep[]>(intakeStepList(this.sections));

	/**
	 * Reads the Practice's name and its Client Field Template.
	 *
	 * A second call for the same Practice is a no-op once it is under way
	 * or loaded, so a layout effect that re-runs on every step navigation
	 * costs one comparison rather than a round trip. A read for a Practice
	 * that answers after another's was asked for is dropped (`PageRead`).
	 */
	async load(fetcher: Fetcher, practiceId: string): Promise<void> {
		if (this.practiceId === practiceId && this.read.state.status !== 'failed') return;
		this.practiceId = practiceId;
		await this.read.load(async () => {
			const [practiceName, template] = await Promise.all([
				loadPracticeName(fetcher, practiceId),
				loadTemplate(fetcher, practiceId)
			]);
			return { practiceName, fields: template.fields };
		}, 'Failed to load');
	}
}

/**
The one flow the `clients/new` layout and its steps share.
*/
export const intakeFlow = new IntakeFlow();
