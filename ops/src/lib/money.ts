const dollars = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

/**
An amount in cents as a person reads it: $42.74.
*/
export function money(cents: number): string {
	return dollars.format(cents / 100);
}
