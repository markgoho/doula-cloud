/**
 * What has been typed into intake so far, across its per-step routes
 * (#466).
 *
 * ## Why a module and not a load function
 *
 * Intake is one question per page, each page a real route with a real
 * `href` -- that is what `StepRail` and the Change links on
 * check-answers both need. The answers, though, belong to the sequence
 * rather than to any one page, and there is no server record to hang
 * them off until the save. So they live here, in module state the
 * `clients/new` layout owns, and every step reads and writes the same
 * draft.
 *
 * ## Why sessionStorage as well
 *
 * Module state survives a client-side navigation and nothing else. A
 * doula typing on a phone who pulls to refresh, or follows a link and
 * comes back, would otherwise lose a half-typed record. The mirror is
 * per-tab and per-Practice, is cleared on a successful save, and every
 * read of it is guarded: a browser with storage denied gets a draft
 * that works exactly as it did before, one navigation at a time.
 */

import type { ClientCreateFields, ClientMatch } from './client.js';

/** One answer to a Practice-defined question. The three shapes cover
 * the five field types intake collects: text and single-select are a
 * string, multi-select is a list, and a checkbox is a boolean. */
export type FieldValue = string | string[] | boolean;

/** Every structural column but `id`, plus the Practice-defined layer --
 * the full `client.Record` shape the create call sends (#466). */
export interface IntakeAnswers extends ClientCreateFields {
	addressLine1: string;
	addressLine2: string;
	addressLocality: string;
	addressRegion: string;
	addressPostalCode: string;
	fieldValues: Record<string, FieldValue>;
}

export function blankAnswers(): IntakeAnswers {
	return {
		givenName: '',
		familyName: '',
		preferredName: '',
		email: '',
		phone: '',
		addressLine1: '',
		addressLine2: '',
		addressLocality: '',
		addressRegion: '',
		addressPostalCode: '',
		dateOfBirth: '',
		fieldValues: {}
	};
}

function storageKey(practiceId: string): string {
	return `doula-cloud:intake:${practiceId}`;
}

/*
 * Storage is best-effort at every touch point. Private browsing, a
 * quota, and a browser configured to refuse site data all throw on
 * access rather than returning null, and none of them is a reason for
 * intake to stop working.
 */
function readStored(practiceId: string): IntakeAnswers | undefined {
	try {
		const raw = sessionStorage.getItem(storageKey(practiceId));
		if (raw === null) return undefined;
		return { ...blankAnswers(), ...(JSON.parse(raw) as Partial<IntakeAnswers>) };
	} catch {
		return undefined;
	}
}

function writeStored(practiceId: string, answers: IntakeAnswers): void {
	try {
		sessionStorage.setItem(storageKey(practiceId), JSON.stringify(answers));
	} catch {
		// Nothing to recover: the draft still lives in module state.
	}
}

function clearStored(practiceId: string): void {
	try {
		sessionStorage.removeItem(storageKey(practiceId));
	} catch {
		// As above.
	}
}

export class IntakeDraft {
	practiceId = $state('');
	answers = $state<IntakeAnswers>(blankAnswers());
	/**
	 * Which steps the reader has been through. An array rather than a Set
	 * so it serialises and so Svelte's proxy reports a push -- membership
	 * is asked through `visited`, which hands out the Set the rail wants.
	 */
	visitedSteps = $state<string[]>([]);
	/**
	 * Set while the reader is inside a Change link's round trip from
	 * check-answers, so Continue returns there instead of walking the
	 * rest of the sequence again.
	 */
	isReturningToCheck = $state(false);
	/**
	 * The matches a refused save named (#466's duplicate-check page).
	 * Held here rather than in the page's own state because the page that
	 * shows them is a route of its own, reached by navigation.
	 */
	matches = $state<ClientMatch[]>([]);

	/** Whether anything has been typed at all -- what a step past the
	 * first checks before rendering a form nobody filled in. */
	get hasStarted(): boolean {
		return this.answers.givenName.trim() !== '';
	}

	/**
	 * Opens the draft for a Practice.
	 *
	 * `carried` is what the search that fronts intake put in the query
	 * string (#498, ADR-0017): it seeds a draft that has nothing in it
	 * yet, and never overwrites one that does -- a reader who walked back
	 * to the first question keeps what was typed over what was searched.
	 */
	start(practiceId: string, carried: Partial<IntakeAnswers> = {}): void {
		if (this.practiceId !== practiceId) {
			this.practiceId = practiceId;
			this.answers = readStored(practiceId) ?? blankAnswers();
			this.visitedSteps = [];
			this.isReturningToCheck = false;
			this.matches = [];
		}
		if (!this.hasStarted) {
			this.answers = { ...this.answers, ...carried };
		}
	}

	/** Records an answer and mirrors the draft, so the next page and the
	 * next page load both see it. */
	update(changes: Partial<IntakeAnswers>): void {
		this.answers = { ...this.answers, ...changes };
		writeStored(this.practiceId, this.answers);
	}

	setFieldValue(fieldId: string, value: FieldValue): void {
		this.update({ fieldValues: { ...this.answers.fieldValues, [fieldId]: value } });
	}

	/** Marks a step answered. Idempotent: a reader who walks the same
	 * question twice has still answered it once. */
	visit(stepId: string): void {
		if (!this.visitedSteps.includes(stepId)) {
			this.visitedSteps = [...this.visitedSteps, stepId];
		}
	}

	/** Everything the draft holds, gone -- what a save that went through
	 * leaves behind, so the next Client starts blank. */
	clear(): void {
		clearStored(this.practiceId);
		this.answers = blankAnswers();
		this.visitedSteps = [];
		this.isReturningToCheck = false;
		this.matches = [];
	}
}

/**
The one draft the `clients/new` layout and its steps share.
*/
export const intakeDraft = new IntakeDraft();
