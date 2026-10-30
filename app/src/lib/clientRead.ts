/*
 * The files a Client reads (#2341): the one list of the screens and modules
 * written in the Client register, the "Client-read screens" of Voice rules 4
 * and 8 in `docs/design/brief.md`. `copy.secondPerson.usage.spec.ts` reads
 * them for "owed" and "Client", and `copy.domainNoun.usage.spec.ts` reads
 * them for a capitalized contract, invoice or birth plan. A spec file may not
 * export, so the list lives here and both import it. Widen it when a new
 * Client-facing file falls outside these globs.
 *
 * Globs relative to `app/`. Each spec walks only the files it reads anyway
 * (never a spec, a route fixture or a type declaration), so a glob here that
 * reaches one of those does not put it in front of either rule.
 */
export const CLIENT_READ: readonly string[] = [
	// Every portal route, and the portal's own `+layout.ts`.
	'src/routes/portal/**/*.{svelte,ts}',
	'src/lib/components/**/Portal*.svelte',
	// Rendered only by the portal's contract page.
	'src/lib/components/organisms/SignContract.svelte',
	// The Client register's modules. `yourCare.ts` is read only by the portal
	// hub, and `portalDoor.ts` is the line under the portal's door on the
	// signed-out landing.
	'src/lib/{clientRegister,clientInvoice,clientPayment,portalDoor,portalLanding,portalVisits,yourCare}.ts'
];
