/**
 * A check page's act, as one value: not yet sent, sending, done with what
 * doula-ops answered, refused with why, or signed out.
 *
 * `app/src/lib/formSubmission.svelte.ts`'s `run`, on ops' own `Outcome`.
 * Before this, five check pages each held an `errors` array, an
 * `isSending` flag, a `result`, a password, a code and an Idempotency-Key,
 * and wrote the same `try { … } finally { isSending = false }` with no
 * catch: a dropped connection stopped the spinner and said nothing. They
 * did not read `Outcome` the same way either -- 'step-up' was a sentence
 * on four pages and unhandled on the fifth. `run` is the one reading: a
 * fresh code first when the act asks for one, then the act, and every arm
 * of the answer, or a throw, lands in exactly one state.
 *
 * Focus stays with the page: it knows which heading a done act draws.
 * `ErrorSummary` re-focuses itself whenever `errors` changes (#467).
 */

import { actKey, serviceProblem, STEP_UP_AGAIN, type StepUpOutcome } from './act.ts';
import type { FormError } from './authRefusal.ts';
import { stepUp, type StepUpDependencies } from './stepUp.ts';

/**
The five states, one name each.
*/
export type ActState<T> =
	| { status: 'idle' }
	| { status: 'sending' }
	| { status: 'done'; value: T }
	| { status: 'refused'; errors: FormError[] }
	| { status: 'signed-out' };

export type CheckActOptions = {
	onsignedout: () => unknown;
	/**
	 * Present for an act behind `requireStepUp`: whose sign-in the fresh code
	 * is for, and how to get the token that proves it.
	 */
	stepUp?: { email: () => string } & StepUpDependencies;
	/**
	Points a refusal at the control a page draws for it (refund's Owner radios).
	*/
	locate?: (errors: FormError[]) => FormError[];
};

export class CheckAct<T> {
	#options: CheckActOptions;
	#key = actKey();

	state = $state<ActState<T>>({ status: 'idle' });
	password = $state('');
	code = $state('');

	constructor(options: CheckActOptions) {
		this.#options = options;
	}

	#refused(errors: FormError[]): void {
		this.state = { status: 'refused', errors: this.#options.locate ? this.#options.locate(errors) : errors };
	}

	get isSending(): boolean {
		return this.state.status === 'sending';
	}

	/**
	Why the act was refused, or the empty array, for the page's `ErrorSummary`.
	*/
	get errors(): FormError[] {
		return this.state.status === 'refused' ? this.state.errors : [];
	}

	/**
	What the act answered, once done; `undefined` in every other state.
	*/
	get value(): T | undefined {
		return this.state.status === 'done' ? this.state.value : undefined;
	}

	/**
	A refusal the page found itself, before anything is sent.
	*/
	refuse(errors: FormError[]): void {
		this.#refused(errors);
	}

	/**
	 * A new act: a new Idempotency-Key, no code or password, nothing refused.
	 * A page calls it when the answers the act is about change.
	 */
	renew(): void {
		this.#key = actKey();
		this.password = '';
		this.code = '';
		this.state = { status: 'idle' };
	}

	/**
	 * Sends the act under this visit's key, after a fresh code when the act
	 * asks for one, and resolves with what it answered once done. A second
	 * call while one is sending does nothing.
	 */
	async run(send: (key: string) => Promise<StepUpOutcome<T>>): Promise<T | undefined> {
		if (this.isSending) return undefined;
		this.state = { status: 'sending' };
		let outcome: StepUpOutcome<T>;
		try {
			const confirm = this.#options.stepUp;
			if (confirm) {
				const errors = await stepUp(
					{ email: confirm.email(), password: this.password, code: this.code },
					{ freshIdToken: confirm.freshIdToken, fetcher: confirm.fetcher }
				);
				if (errors.length > 0) {
					this.#refused(errors);
					return undefined;
				}
			}
			outcome = await send(this.#key);
		} catch {
			this.#refused(serviceProblem());
			return undefined;
		}
		switch (outcome.kind) {
			case 'ok': {
				this.state = { status: 'done', value: outcome.value };
				return outcome.value;
			}
			case 'refused': {
				this.#refused(outcome.errors);
				return undefined;
			}
			case 'step-up': {
				this.#refused([STEP_UP_AGAIN]);
				return undefined;
			}
			case 'signed-out': {
				this.state = { status: 'signed-out' };
				await this.#options.onsignedout();
				return undefined;
			}
		}
	}
}
