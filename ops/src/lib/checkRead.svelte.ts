/**
 * A check page's own read, as one value: still loading, loaded with what
 * doula-ops answered, failed with why, or signed out.
 *
 * `app/src/lib/pageRead.svelte.ts`'s shape, on ops' own `Outcome`. Before
 * this, each check page held the read as a nullable `check` beside an
 * `errors` array, set them in an `onMount` with no catch, and drew
 * `{#if check} … {/if}`: a refused read put its reason in the summary with
 * no heading under it, and a dropped connection drew nothing at all. Here
 * a thrown read is a failure like a refused one, and a page that draws
 * 'failed' always has its heading and the reason.
 */

import { serviceProblem, type Outcome } from './act.ts';
import type { FormError } from './authRefusal.ts';

/**
The four states, one name each.
*/
export type CheckState<T> =
	| { status: 'loading' }
	| { status: 'loaded'; value: T }
	| { status: 'failed'; errors: FormError[] }
	| { status: 'signed-out' };

export class CheckRead<T> {
	#onsignedout: () => unknown;

	state = $state<CheckState<T>>({ status: 'loading' });

	constructor(onsignedout: () => unknown) {
		this.#onsignedout = onsignedout;
	}

	/**
	What was read, once loaded; `undefined` in every other state.
	*/
	get value(): T | undefined {
		return this.state.status === 'loaded' ? this.state.value : undefined;
	}

	/**
	Why the read failed, or the empty array, for the page's `ErrorSummary`.
	*/
	get errors(): FormError[] {
		return this.state.status === 'failed' ? this.state.errors : [];
	}

	/**
	A read the page made again later (refund's quote), written as what it now holds.
	*/
	set(value: T): void {
		this.state = { status: 'loaded', value };
	}

	async load(read: () => Promise<Outcome<T>>): Promise<void> {
		this.state = { status: 'loading' };
		let outcome: Outcome<T>;
		try {
			outcome = await read();
		} catch {
			this.state = { status: 'failed', errors: serviceProblem() };
			return;
		}
		switch (outcome.kind) {
			case 'ok': {
				this.state = { status: 'loaded', value: outcome.value };
				return;
			}
			case 'refused': {
				this.state = { status: 'failed', errors: outcome.errors };
				return;
			}
			case 'signed-out': {
				this.state = { status: 'signed-out' };
				await this.#onsignedout();
			}
		}
	}
}
