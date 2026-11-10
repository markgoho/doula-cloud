import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
import { PageRead } from '#lib/pageRead.svelte.js';
import { FIELD_TYPES, isSelectType, type FieldType } from '#lib/planTemplate.js';
import { practiceQuestionIds } from '#lib/components/organisms/PracticeQuestionForm.svelte';
import {
	activeQuestionCount,
	optionsFromText,
	placeQuestion,
	questionErrors,
	questionPosition,
	withQuestion
} from '#lib/practiceQuestions.js';

export interface Listed {
	id: string;
	type: FieldType;
	label: string;
	options?: string[];
	order: number;
	archived?: boolean;
}

export interface Draft {
	id: string;
	type: FieldType;
	label: string;
	options?: string[];
}

export interface PracticeQuestionOptions<F extends Listed> {
	/**
	The stem of every control's id (`practiceQuestionIds`).
	*/
	name: string;
	/**
	The question's id, or `new` for one not yet saved.
	*/
	questionId: string;
	/**
	The kind of answer a new question was started with (`?kind=`).
	*/
	kind: string | undefined;
	/**
	The list as it stands. A rejection carries the reason the page shows.
	*/
	load: () => Promise<F[]>;
	loadFailure: string;
	notOnList: string;
	/**
	A question the list holds but this page does not change.
	*/
	isEditable?: (question: F) => boolean;
	/**
	A saved question keeps its kind of answer.
	*/
	locksKind?: boolean;
	refuseLabel?: (label: string) => string | undefined;
	toField: (draft: Draft) => F;
	/**
	Sends the whole list back and leaves the page.
	*/
	onSave: (fields: F[]) => Promise<void>;
	retire: {
		verb: string;
		consequence: string;
		apply: (fields: F[], id: string) => F[];
		onDone: (fields: F[], label: string) => Promise<void>;
	};
}

export class PracticeQuestion<F extends Listed> {
	readonly #options: PracticeQuestionOptions<F>;
	readonly submission = new FormSubmission();
	fields = $state.raw<F[]>([]);
	id = $state('');
	type = $state<FieldType>('short_text');
	label = $state('');
	optionsText = $state('');
	position = $state<number | undefined>();
	/**
	The list's read, and whether the question it names can be asked:
	one value, failed with why when it cannot.
	*/
	readonly read = new PageRead();
	isConfirmingRetire = $state(false);

	constructor(options: PracticeQuestionOptions<F>) {
		this.#options = options;
	}

	// Opens the question the route names on the list just read, or throws
	// the sentence that says why it cannot be.
	#open(): void {
		const { kind, questionId, notOnList, isEditable } = this.#options;
		if (this.isNew) {
			const started = FIELD_TYPES.find((candidate) => candidate === kind);
			if (!started) throw new Error('Choose the kind of answer first, from Add a question.');
			this.id = crypto.randomUUID();
			this.type = started;
		} else {
			const question = this.fields.find((field) => field.id === questionId && (isEditable?.(field) ?? true));
			if (!question) throw new Error(notOnList);
			this.id = question.id;
			this.type = question.type;
			this.label = question.label;
			this.optionsText = (question.options ?? []).join('\n');
			this.position = questionPosition(this.fields, this.id);
		}
	}

	get ids() {
		return practiceQuestionIds(this.#options.name);
	}

	get name() {
		return this.#options.name;
	}

	get isNew() {
		return this.#options.questionId === 'new';
	}

	get isLoaded() {
		return this.read.isLoaded;
	}

	get title() {
		return this.isNew ? 'Add a question' : 'Change a question';
	}

	get frame() {
		return {
			journey: 'Adding or changing a question',
			steps: [],
			question: { as: 'legend' as const, text: this.title }
		};
	}

	get waiting() {
		return {
			title: this.title,
			fieldsets: [],
			read: this.read.frame('Loading the question')
		};
	}

	get shownLabel() {
		return this.label || 'Untitled question';
	}

	get isKindLocked() {
		return Boolean(this.#options.locksKind) && !this.isNew;
	}

	get positionCount() {
		return activeQuestionCount(this.fields);
	}

	get retireVerb() {
		return this.#options.retire.verb;
	}

	get retireConsequence() {
		return this.#options.retire.consequence;
	}

	async load() {
		const { load, loadFailure } = this.#options;
		await this.read.load(async () => {
			this.fields = await load();
			this.#open();
		}, loadFailure);
	}

	async save(event: SubmitEvent) {
		event.preventDefault();
		await this.submission.run(async () => {
			const options = optionsFromText(this.optionsText);
			const errors = questionErrors(
				{ type: this.type, label: this.label, options },
				this.ids,
				this.#options.refuseLabel
			);
			if (errors.length > 0) {
				return errors;
			}
			const draft: Draft = { id: this.id, type: this.type, label: this.label.trim() };
			if (isSelectType(this.type)) {
				draft.options = options;
			}
			const placed = withQuestion(this.fields, this.#options.toField(draft));
			await this.#options.onSave(
				this.position === undefined ? placed : placeQuestion(placed, this.id, this.position)
			);
		}, orThrownMessage);
	}

	async retire() {
		this.isConfirmingRetire = false;
		const { retire } = this.#options;
		await this.submission.run(
			() => retire.onDone(retire.apply(this.fields, this.id), this.shownLabel),
			orThrownMessage
		);
	}
}
