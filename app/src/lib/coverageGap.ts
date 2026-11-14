/**
 * The words for when a coverage gap runs. A gap is the one on-call fact that
 * carries a clock (GLOSSARY: "a few drinks on Saturday night"), so it is
 * said as ADR-0022's Visit form, in the Practice's zone (ADR-0036).
 */
import { scheduledVisitParts } from './dates.js';
import { dayIn } from './zonedTime.js';

/**
 * A gap's two ends as a reader says them: "Sat Nov 14, 6:00pm" and "Sun Nov
 * 15, 9:00am". An end on the day the gap starts says its clock alone,
 * "11:00pm". The year joins a day only when it is not the year `now` is in.
 */
export function gapTimes(
	startsAt: string,
	endsAt: string,
	timeZone: string,
	now: Date
): { start: string; end: string } {
	const start = scheduledVisitParts(startsAt, timeZone, now);
	const end = scheduledVisitParts(endsAt, timeZone, now);
	const isSameDay = dayIn(new Date(startsAt), timeZone) === dayIn(new Date(endsAt), timeZone);
	return {
		start: `${start.day} ${start.clock}`,
		end: isSameDay ? end.clock : `${end.day} ${end.clock}`
	};
}

/**
 * The whole phrase: "Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am", or "Sat Nov
 * 14, 6:00pm to 9:00pm" within one day.
 */
export function formatGapTimes(startsAt: string, endsAt: string, timeZone: string, now: Date): string {
	const { start, end } = gapTimes(startsAt, endsAt, timeZone, now);
	return `${start} to ${end}`;
}
