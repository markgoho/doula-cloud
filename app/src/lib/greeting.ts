/*
 * The signed-out landing's greeting (#1645), by the hour on the reader's
 * own device. Nobody is signed in on that page, so there is no Practice
 * timezone to read (ADR-0036) and the device clock is the only clock --
 * which is also the correct one, since the greeting is about the reader's
 * own morning, not a Practice's.
 *
 * `now` defaults to the device clock, the same seam `dates.ts` and
 * `onCall.ts` take: a caller reads it once, and a spec passes a `Date`
 * built from local parts, so the answer does not depend on the time zone
 * the suite runs in.
 */
const GREETINGS = {
	morning: 'Good morning.',
	afternoon: 'Good afternoon.',
	evening: 'Good evening.',
	night: 'Up late? Welcome.'
} as const;

export function greetingFor(now: Date = new Date()): string {
	const hour = now.getHours();
	if (hour >= 5 && hour < 12) return GREETINGS.morning;
	if (hour >= 12 && hour < 17) return GREETINGS.afternoon;
	if (hour >= 17 && hour < 22) return GREETINGS.evening;
	return GREETINGS.night;
}
