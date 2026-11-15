import { tick } from 'svelte';
import type { SentPeriod, SentPeriodChoice } from './sentPeriod.js';
import { sentPeriodChoiceOf } from './sentPeriod.js';
import { scheduleRangeErrors, type ScheduleRangeErrors } from './visitSchedule.js';

/**
 * What the "Sent" select and its two typed days hold before Apply (#2871,
 * #2954), shared by the lists that narrow to a period: put back in step with
 * the address after every navigation, read as the period they would send, and
 * checked for two days that are not a range.
 */
export class SentPeriodDraft {
	choice = $state<SentPeriodChoice>('');
	from = $state('');
	to = $state('');
	errors = $state<ScheduleRangeErrors>({});

	get isChoosingDays(): boolean {
		return this.choice === 'choose';
	}

	get period(): SentPeriod | undefined {
		if (this.choice === '') return undefined;
		return this.choice === 'choose' ? { days: { from: this.from, to: this.to } } : { preset: this.choice };
	}

	seed(period: SentPeriod | undefined) {
		this.choice = sentPeriodChoiceOf(period);
		this.from = period && 'days' in period ? period.days.from : '';
		this.to = period && 'days' in period ? period.days.to : '';
		this.errors = {};
	}

	clear() {
		this.seed(undefined);
	}

	/**
	 * Two typed days that are not a range are said beside them; the others send as drawn.
	 */
	hasProblem(): boolean {
		const problems = this.isChoosingDays ? scheduleRangeErrors(this.from, this.to) : {};
		this.errors = problems;
		return Boolean(problems.from || problems.to);
	}

	async focusProblem() {
		await tick();
		document.querySelector<HTMLInputElement>(this.errors.from ? 'input[name="from"]' : 'input[name="to"]')?.focus();
	}

	/**
	 * Apply's own check: true after moving focus to the day that is wrong.
	 */
	async refusesToApply(): Promise<boolean> {
		if (!this.hasProblem()) return false;
		await this.focusProblem();
		return true;
	}

	/**
	 * "Choose days" alone only opens its fields: nothing is sent until Apply.
	 */
	isOnlyOpeningDays(changed: readonly string[]): boolean {
		return this.isChoosingDays && changed.length === 1 && changed[0] === 'sent';
	}
}
