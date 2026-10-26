// `bun run seed:demo` -- seeds the local demo Practice (#1994) against a
// stack `bun run dev:full` already started, or finds it there and changes
// nothing, then prints the accounts and a fresh portal sign-in link for the
// Client. dev:full runs the same seed on every start; this is the way to
// run it again, or to get a new link once the last one has expired.
// demo-practice.ts holds the seed and says what it makes.
import { request } from '@playwright/test';
import { demoSummary, mintPortalLink, seedDemoPractice } from './demo-practice';

const context = await request.newContext();
try {
	const practice = await seedDemoPractice(context);
	console.log(demoSummary(practice, await mintPortalLink(context)));
} finally {
	await context.dispose();
}
