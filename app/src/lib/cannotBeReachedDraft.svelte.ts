/**
 * What has been answered so far in the act that records a time a Doula
 * cannot be reached (#2934), across its per-step routes.
 *
 * Held as `intakeDraft` holds an intake: module state the journey's pages
 * share, mirrored to sessionStorage per tab and per Practice, and cleared on
 * save. The reason is free text about a person's evening and is held here
 * only, never in an address.
 */

interface Answers {
	staffId: string;
	staffName: string;
	startsAt: string;
	endsAt: string;
	reason: string;
	cover: Record<string, string>;
}

function blank(): Answers {
	return { staffId: '', staffName: '', startsAt: '', endsAt: '', reason: '', cover: {} };
}

function readStored(key: string): Answers | undefined {
	try {
		const raw = sessionStorage.getItem(key);
		return raw === null ? undefined : { ...blank(), ...(JSON.parse(raw) as Partial<Answers>) };
	} catch {
		return undefined;
	}
}

function writeStored(key: string, value: Answers): void {
	try {
		sessionStorage.setItem(key, JSON.stringify(value));
	} catch {
		// The draft still lives in module state.
	}
}

function clearStored(key: string): void {
	try {
		sessionStorage.removeItem(key);
	} catch {
		// As above.
	}
}

export class CannotBeReachedDraft {
	scope = $state('');
	answers = $state<Answers>(blank());

	get #key(): string {
		return `doula-cloud:cannot-be-reached:${this.scope}`;
	}

	start(scope: string): void {
		if (this.scope === scope) return;
		this.scope = scope;
		this.answers = readStored(this.#key) ?? blank();
	}

	update(changes: Partial<Answers>): void {
		this.answers = { ...this.answers, ...changes };
		writeStored(this.#key, this.answers);
	}

	setCover(engagementId: string, value: string): void {
		this.update({ cover: { ...this.answers.cover, [engagementId]: value } });
	}

	/**
	Keeps only the answers for births still asked about.
	*/
	keepCover(engagementIds: readonly string[]): void {
		const kept = Object.fromEntries(Object.entries(this.answers.cover).filter(([id]) => engagementIds.includes(id)));
		this.update({ cover: kept });
	}

	clear(): void {
		clearStored(this.#key);
		this.answers = blank();
	}
}

export const cannotBeReachedDraft = new CannotBeReachedDraft();
