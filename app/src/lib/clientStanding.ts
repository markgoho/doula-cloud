/**
 * A Client record's standing, as the BFF names it (`clientstanding` in
 * `api/internal`, derived once by the `clients.standing` column, 00171).
 *
 * A screen asks one of the three questions below of the `standing` field
 * the detail read carries, rather than testing `erasedAt` or
 * `mergedInto` itself, so two screens cannot answer "can this record
 * take a write?" two different ways. `erasedAt` and `mergedInto` stay on
 * the read for what they say beyond the standing: the date an erasure
 * ran, and the record a tombstone points at.
 */

/**
Mirrors clientstanding.Standing.
*/
export type ClientStanding = 'live' | 'merged' | 'erased' | 'merged_then_erased';

/**
Something that carries a standing, such as a ClientDetail.
*/
export interface HasStanding {
	standing: ClientStanding;
}

/** Whether the record can take a write: neither merged nor erased. Every
 * write the BFF accepts on or under a Client asks the same question. */
export function isLive(record: HasStanding | undefined): boolean {
	return record?.standing === 'live';
}

/** Whether the record's details were erased, as a record of its own or
 * as the tombstone of one. */
export function isErased(record: HasStanding | undefined): boolean {
	return record?.standing === 'erased' || record?.standing === 'merged_then_erased';
}

/** Whether the record was merged into another, erased since or not. A
 * tombstone has no screen of its own: a reader goes to its survivor. */
export function isTombstone(record: HasStanding | undefined): boolean {
	return record?.standing === 'merged' || record?.standing === 'merged_then_erased';
}
