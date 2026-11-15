/**
The characters a table's Doula cell holds on one line.
*/
export const DOULA_CELL_FITS = 16;

export interface DoulaWords {
	/**
	Every name, the reader first as "You", for a card and a screen reader.
	*/
	full: string;
	/**
	What a one-line cell draws: all the names where they fit, else the first and a count.
	*/
	lead: string;
	/**
	How many names the one-line cell leaves out of `lead`.
	*/
	more: number;
}

/**
 * How a row names its Doulas (#2958). The reader is "You" wherever she is
 * one, and first, so she finds herself; two names are both drawn where they
 * fit the cell, and a longer list is its first name and how many more.
 */
export function doulaWords(names: readonly string[], ids: readonly string[] = [], readerId = ''): DoulaWords {
	const isReader = (index: number) => readerId !== '' && ids[index] === readerId;
	const readerIndex = names.findIndex((_, index) => isReader(index));
	const named = names.map((name, index) => (isReader(index) ? 'You' : name));
	const ordered = readerIndex > 0 ? [named[readerIndex]!, ...named.filter((_, index) => index !== readerIndex)] : named;
	const full = ordered.join(', ');
	if (full.length <= DOULA_CELL_FITS) return { full, lead: full, more: 0 };
	return { full, lead: ordered[0]!, more: ordered.length - 1 };
}
