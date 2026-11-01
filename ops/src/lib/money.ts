const dollars = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

export function money(cents: number): string {
	return dollars.format(cents / 100);
}
