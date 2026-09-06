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

import { loadTemplate, type Field } from './clientFieldTemplate.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { intakeSections, intakeStepList, type IntakeSection, type IntakeStep } from './intakeJourney.js';

export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

async function loadPracticeName(fetcher: Fetcher, practiceId: string): Promise<string> {
	const response = await fetcher(`/api/practices/${practiceId}/session`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { practiceName: string } = await response.json();
	return body.practiceName;
}

export class IntakeFlow {
	practiceId = $state('');
	status = $state<'idle' | 'loading' | 'ready' | 'error'>('idle');
	loadError = $state('');
	practiceName = $state('');
	fields = $state<Field[]>([]);

	sections = $derived<IntakeSection[]>(intakeSections(this.fields, this.practiceName));
	steps = $derived<IntakeStep[]>(intakeStepList(this.sections));

	/**
	 * Reads the Practice's name and its Client Field Template.
	 *
	 * A second call for the same Practice is a no-op once it is ready, so
	 * a layout effect that re-runs on every step navigation costs one
	 * comparison rather than a round trip.
	 */
	async load(fetcher: Fetcher, practiceId: string): Promise<void> {
		if (this.practiceId === practiceId && this.status !== 'idle' && this.status !== 'error') {
			return;
		}
		this.practiceId = practiceId;
		this.status = 'loading';
		this.loadError = '';
		try {
			const [practiceName, template] = await Promise.all([
				loadPracticeName(fetcher, practiceId),
				loadTemplate(fetcher, practiceId)
			]);
			this.practiceName = practiceName;
			this.fields = template.fields;
			this.status = 'ready';
		} catch (error) {
			this.loadError = error instanceof Error && error.message ? error.message : 'Failed to load';
			this.status = 'error';
		}
	}
}

/**
The one flow the `clients/new` layout and its steps share.
*/
export const intakeFlow = new IntakeFlow();
