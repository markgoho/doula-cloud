/**
The browser-tab title every route sets (#487). `serviceName` defaults to the
product name for the Staff side; the Client portal passes the Practice's own
name instead, matching #431's Practice-named chrome. `isError` prefixes
`Error: `, GOV.UK's own convention for a genuinely refused page (#467).
*/
export function formatPageTitle(
	page: string,
	options: { serviceName?: string; isError?: boolean } = {}
): string {
	const serviceName = options.serviceName ?? 'Doula Cloud';
	const prefix = options.isError ? 'Error: ' : '';
	return `${prefix}${page} — ${serviceName}`;
}
