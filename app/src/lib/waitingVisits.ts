/**
 * The Visits waiting for a new Doula, Practice-wide (#2874): the dated ones
 * the Schedule lists and the undated ones only an Engagement shows. Mirrors
 * the Go BFF's WaitingVisitsResponse (api/internal/visit/waiting.go).
 */
import type { Fetcher } from './fetcher.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { DEFAULT_SCHEDULE_DAYS, addDays, scheduleHref } from './visitSchedule.js';
import { dayIn } from './zonedTime.js';

/**
An Engagement holding Visits with no date that wait for a new Doula.
*/
export interface WaitingEngagement {
	engagementId: string;
	clientName: string;
	undatedCount: number;
}

export interface WaitingVisits {
	total: number;
	datedCount: number;
	undatedCount: number;
	/**
	The last of the dated ones, so a link can name days that hold them all.
	*/
	latestScheduledAt?: string;
	items: WaitingEngagement[];
	hasMore: boolean;
}

export async function loadWaitingVisits(fetcher: Fetcher, practiceId: string): Promise<WaitingVisits> {
	const response = await fetcher(`/api/practices/${practiceId}/visits/waiting`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
Drawn where a Visit waits, or where the read failed and the line says so.
*/
export function isWaitingLineShown(waiting: WaitingVisits | 'unavailable' | undefined): waiting is WaitingVisits | 'unavailable' {
	return waiting === 'unavailable' || (waiting !== undefined && waiting.total > 0);
}

function visitCount(count: number): string {
	return `${count} ${count === 1 ? 'Visit' : 'Visits'}`;
}

export function datedWaitingLabel(waiting: WaitingVisits): string {
	return `${visitCount(waiting.datedCount)} on the Schedule`;
}

export function undatedWaitingLabel(waiting: WaitingVisits): string {
	return `${visitCount(waiting.undatedCount)} with no date yet`;
}

export function waitingEngagementLabel(item: WaitingEngagement): string {
	return item.undatedCount === 1 ? item.clientName : `${item.clientName}, ${visitCount(item.undatedCount)}`;
}

/** The Schedule narrowed to the Visits waiting for a new Doula, over days that
 * hold every one of them: today through the day of the last. */
export function datedWaitingHref(basePath: string, waiting: WaitingVisits, now: Date, timeZone: string): string {
	const today = dayIn(now, timeZone);
	const last = waiting.latestScheduledAt ? dayIn(new Date(waiting.latestScheduledAt), timeZone) : today;
	const chosen = last > addDays(today, DEFAULT_SCHEDULE_DAYS - 1) ? { from: today, to: last } : undefined;
	return scheduleHref(basePath, { preset: DEFAULT_SCHEDULE_DAYS, chosen, isMine: false, doula: '', needsDoula: true, type: '', order: '' });
}
