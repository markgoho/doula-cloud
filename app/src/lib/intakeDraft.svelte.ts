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

import type { ClientCreateFields, ClientMatch, ClientSearchFields } from './client.js';
import { carriedGivenName, searchedName, type IntakeOrigin } from './intakeJourney.js';

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

/*
 * Storage is best-effort at every touch point. Private browsing, a
 * quota, and a browser configured to refuse site data all throw on
 * access rather than returning null, and none of them is a reason for
 * intake to stop working.
 */
function readStored(key: string): IntakeAnswers | undefined {
	try {
		const raw = sessionStorage.getItem(key);
		if (raw === null) return undefined;
		return { ...blankAnswers(), ...(JSON.parse(raw) as Partial<IntakeAnswers>) };
	} catch {
		return undefined;
	}
}

function readStoredSearch(key: string): ClientSearchFields | undefined {
	try {
		const raw = sessionStorage.getItem(key);
		if (raw === null) return undefined;
		return JSON.parse(raw) as ClientSearchFields;
	} catch {
		return undefined;
	}
}

function writeStored(key: string, value: IntakeAnswers | ClientSearchFields): void {
	try {
		sessionStorage.setItem(key, JSON.stringify(value));
	} catch {
		// Nothing to recover: the draft still lives in module state.
	}
}

function clearStored(key: string): void {
	try {
		sessionStorage.removeItem(key);
	} catch {
		// As above.
	}
}

export class IntakeDraft {
	/**
	 * Which journey the draft belongs to, so two journeys' drafts never
	 * share a stored copy: `intake` for a new Client, `details` for the
	 * details added from her record (#1610).
	 */
	readonly #namespace: string;
	/** What the draft is for: the Practice in intake, the Client in her
	 * details journey. Half of the stored copy's key. */
	scope = $state('');
	answers = $state<IntakeAnswers>(blankAnswers());
	/**
	 * Which steps the reader has been through, in the order they were
	 * walked. An array rather than a Set so it serializes and so Svelte's
	 * proxy reports a push; `journeySteps` asks it with `includes`, which
	 * is why no Set is needed at all.
	 */
	visitedSteps = $state<string[]>([]);
	/**
	 * The matches a refused save named (#466's duplicate-check page).
	 * Held here rather than in the page's own state because the page that
	 * shows them is a route of its own, reached by navigation.
	 */
	matches = $state<ClientMatch[]>([]);
	/**
	 * The screen that opened the name question directly (#1609), which
	 * is where its Back goes. Undefined when intake was reached from the
	 * search, which is then where Back goes. Module state only, not
	 * mirrored: the name question's own URL carries it, so a reload of
	 * that page reads it again. It names a screen, never a Client, so it
	 * may sit in the URL.
	 */
	origin = $state<IntakeOrigin | undefined>();
	/**
	 * The four values the search typed, as typed, when the search opened
	 * intake. Outside `answers` on purpose: `answers` is what the save
	 * sends and what the name question edits, and this is neither. Two
	 * screens read it. The name question shows a name of two or more
	 * words from it (`searchedName`), and the search fills its boxes
	 * from it again when the name question's Back returns there (#1835),
	 * so nothing has to be retyped. Mirrored under a key of its own
	 * (#1829): the search hands it over in module memory, not the URL,
	 * so the mirror is the only place a reload can read it back from.
	 */
	searched = $state<ClientSearchFields | undefined>();

	constructor(namespace = 'intake') {
		this.#namespace = namespace;
	}

	get #storageKey(): string {
		return `doula-cloud:${this.#namespace}:${this.scope}`;
	}

	get #searchedKey(): string {
		return `${this.#storageKey}:searched`;
	}

	/**
	 * A name of two or more words the search typed, which carries into
	 * neither name field (#1716) and which the name question shows
	 * (#1758). It is never saved.
	 */
	get searchedName(): string | undefined {
		return searchedName(this.searched?.name);
	}

	/** Whether the one fact ADR-0017 requires is here. Read by `start`,
	 * to decide whether the search's carried values are seeding an empty
	 * draft or would overwrite a typed one, and by `givenNameRefusal`,
	 * which is what every page's save asks before sending anything. */
	get hasGivenName(): boolean {
		return this.answers.givenName.trim() !== '';
	}

	/**
	 * Opens the draft for a Practice, or in the details journey for a
	 * Client.
	 *
	 * `carried` seeds a draft that has nothing in it yet, and never
	 * overwrites one that does: the details journey opens it with the
	 * answers already on her record (#1610). Intake seeds through `carry`
	 * instead, as its layout mounts.
	 */
	start(scope: string, carried: Partial<IntakeAnswers> = {}): void {
		if (this.scope !== scope) {
			this.scope = scope;
			this.answers = readStored(this.#storageKey) ?? blankAnswers();
			this.searched = readStoredSearch(this.#searchedKey);
			this.visitedSteps = [];
			this.matches = [];
		}
		if (!this.hasGivenName) {
			this.answers = { ...this.answers, ...carried };
		}
	}

	/**
	 * Seeds the draft with what the search that fronts intake typed
	 * (#498, ADR-0017), handed over in module memory (#1829,
	 * `#lib/intakeHandoff.ts`). A one-word name fills Given name; a name of
	 * two or more words fills no field and is shown from `searched`
	 * (#1716, #1758). A key the search left blank is left off, since a
	 * blank would overwrite. Mirrored like any other answer, so a reload of
	 * the name question keeps it.
	 */
	carry(fields: ClientSearchFields | undefined): void {
		if (!fields) return;
		const seeded: Partial<IntakeAnswers> = {};
		const givenName = carriedGivenName(fields.name);
		if (givenName) seeded.givenName = givenName;
		for (const key of ['dateOfBirth', 'email', 'phone'] as const) {
			const value = fields[key].trim();
			if (value) seeded[key] = value;
		}
		this.update(seeded);
		this.searched = { ...fields };
		writeStored(this.#searchedKey, this.searched);
	}

	/** Records an answer and mirrors the draft, so the next page and the
	 * next page load both see it. */
	update(changes: Partial<IntakeAnswers>): void {
		this.answers = { ...this.answers, ...changes };
		writeStored(this.#storageKey, this.answers);
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
		clearStored(this.#storageKey);
		clearStored(this.#searchedKey);
		this.answers = blankAnswers();
		this.visitedSteps = [];
		this.matches = [];
		this.origin = undefined;
		this.searched = undefined;
	}
}

/**
The one draft the `clients/new` layout and its steps share.
*/
export const intakeDraft = new IntakeDraft();
