/**
 * A Practice's timezone (#1166, ADR-0036): the one IANA zone name its
 * calendar-day math happens in, and the short list of them a person is
 * actually asked to choose from.
 *
 * The IANA database carries several hundred names and the API accepts any
 * of them, because the database is what a zone *is*. A person choosing
 * one does not know that list -- she knows she is on Eastern time, or
 * that Arizona does not move its clocks. So the control offers the seven
 * zones that cover the United States, labeled the way she would say them,
 * and the value behind each is the ordinary IANA name for that zone. The
 * US is the whole list because Work State enumerates US states and
 * nothing else (`workStates.ts`); a Practice somewhere else is a product
 * decision the product has not made, not a gap in this file.
 *
 * The list is a convenience on the screen, never the boundary. `ianazone`
 * in the Go BFF refuses anything the IANA database does not name, on both
 * signup and the settings write, so a zone this file has never heard of
 * is still accepted when it is real -- which is what lets
 * `timezoneOptions` show a browser-detected or already-stored zone that
 * is outside the seven.
 */

import type { LabeledValue } from './roles.js';

/**
 * The sentence beneath the timezone question, on both screens that ask
 * it. Named once because the same question asked in two places has to
 * read the same way -- and because what it says is load-bearing rather
 * than decorative: the zone is not a display preference, it is what
 * decides which calendar day a Visit falls on and therefore whether the
 * Visit counts as birth or postpartum work.
 */
export const TIMEZONE_HINT =
	'We use it to work out which day a Visit falls on.';

/**
 * What a person reads when she has chosen no zone at all. GOV.UK's rule
 * for an error message -- start with the field's own noun and say what
 * to do -- and the same sentence `staffauth.MsgTimezoneNeeded` sends
 * from the BFF, so the refusal she meets before the request and the one
 * she meets after it are one sentence.
 *
 * A zone she *did* choose that the IANA database does not name is a
 * different refusal and reads differently (`ianazone.MsgNotRecognized`);
 * only the BFF can tell her that, because only the BFF holds the
 * database.
 */
export const TIMEZONE_NEEDED = 'Choose the timezone this Practice works in';

/**
 * The seven zones the United States keeps. Arizona is its own entry
 * rather than a footnote on Mountain: it does not observe daylight
 * saving, so for eight months of the year a Practice in Phoenix and one
 * in Denver disagree about what time it is, and picking the wrong one
 * moves the boundary a Visit is typed against.
 */
export const US_TIMEZONES: readonly LabeledValue[] = [
	{ value: 'America/New_York', label: 'Eastern time (New York)' },
	{ value: 'America/Chicago', label: 'Central time (Chicago)' },
	{ value: 'America/Denver', label: 'Mountain time (Denver)' },
	{ value: 'America/Phoenix', label: 'Mountain time, no daylight saving (Phoenix)' },
	{ value: 'America/Los_Angeles', label: 'Pacific time (Los Angeles)' },
	{ value: 'America/Anchorage', label: 'Alaska time (Anchorage)' },
	{ value: 'Pacific/Honolulu', label: 'Hawaii time (Honolulu)' }
];

/**
 * The zone this browser believes it is in, or '' when it will not say.
 *
 * Used to pre-select on signup, where GOV.UK's Select guidance allows a
 * default for something the service already knows: the browser has
 * reported an answer, so the founder is confirming rather than being
 * nudged toward a guess. It is only ever a starting point -- the control
 * is a plain select she can change, and nothing is written until she
 * submits.
 */
export function detectTimezone(): string {
	try {
		return new Intl.DateTimeFormat().resolvedOptions().timeZone ?? '';
	} catch {
		/* v8 ignore next: a runtime with no resolvable zone, which no
		   browser the app supports is */
		return '';
	}
}

/**
 * The options a timezone select shows: the seven US zones, plus `current`
 * when it is a real zone outside them.
 *
 * Without that second half, a Practice already stored in
 * `America/Indiana/Indianapolis` -- or a browser reporting
 * `America/Detroit` -- would meet a select whose value matches no option,
 * which renders as blank and silently rewrites her zone the moment she
 * saves. Showing the zone actually held is the honest answer, even
 * though it is not one of the seven -- labeled the way the seven are
 * (`zoneLabel`), with the IANA name as the stored value (#2023).
 */
export function timezoneOptions(current: string): readonly LabeledValue[] {
	if (current === '' || US_TIMEZONES.some((zone) => zone.value === current)) {
		return US_TIMEZONES;
	}
	return [...US_TIMEZONES, { value: current, label: zoneLabel(current) }];
}

/**
 * A zone outside the seven, in the seven's own shape: its common name and
 * its city, "Eastern time (Indianapolis)" for `America/Indiana/Indianapolis`
 * (#2023). The name is `Intl`'s generic one, which does not change with
 * daylight saving, so the label reads the same all year and needs no
 * clock. No offset: a zone's offset does change, and a label that moved
 * twice a year would need the current instant to draw.
 *
 * Falls back to the IANA name itself when `Intl` has no generic name for
 * the zone, or does not know it at all (a `RangeError`): the BFF's own
 * check is what decides whether a zone is real, and a raw name is still
 * the true answer.
 */
export function zoneLabel(zone: string): string {
	let name: string | undefined;
	try {
		name = new Intl.DateTimeFormat('en-US', { timeZone: zone, timeZoneName: 'longGeneric' })
			.formatToParts(0)
			.find((part) => part.type === 'timeZoneName')?.value;
	} catch {
		return zone;
	}
	if (!name || name.startsWith('GMT')) return zone;
	const city = zone.split('/').at(-1)!.replaceAll('_', ' ');
	return `${name.replace(/ Time$/, ' time')} (${city})`;
}
