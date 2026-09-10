/*
 * The Practice-wide Billing screen, as the continuum check sees it
 * (#595).
 *
 * The balance and the ledger's first page come from `+page.ts`'s own
 * `load`, so both arrive as a component prop the same way the invoice
 * list fixture's `data` does. The ledger carries no Practice-typed free
 * text (origin is a fixed enum, quantity a number), so this measures a
 * large realistic balance and the widest origin label rather than any
 * hostile string.
 *
 * One session is all this screen has (#928), and it takes a moment to
 * see why. The route reads `isOwnerOrAdmin` to decide whether the Buy
 * credits button is usable, which reads like two trees -- but the only
 * session that would answer `false` never reaches the screen: the balance
 * arrives through `+page.ts`'s own `load`, the endpoint behind it is
 * `staffauth.OwnerAndAdmin` (`billing/mount.go`), and a Doula meets
 * `refuseRead` and `practices/+error.svelte` instead of this component.
 * An Owner and an Admin both answer `true` and draw the identical screen.
 * So the disabled button and the sentence under it are a tree no session
 * can reach, and declaring a variant for it would measure a screen nobody
 * ever sees -- the same argument #537 makes about polite content.
 * [#1162](https://github.com/markgoho/doula-cloud/issues/1162) holds the
 * unreachable branch itself.
 */
import type { Balance } from '#lib/billing.js';
import type { RouteFixture } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

export const data: Balance = {
	balance: 42,
	ledger: {
		items: [
			{ origin: 'founding_grant', quantity: 20, createdAt: '2026-01-01T00:00:00Z' },
			{ origin: 'purchase', quantity: 30, createdAt: '2026-02-01T00:00:00Z' },
			{ origin: 'consumption', quantity: -8, createdAt: '2026-03-01T00:00:00Z' }
		],
		hasMore: false
	},
	price: { unitAmountCents: 2000, currency: 'usd' }
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide Billing screen',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/billing',
	props: { data },
	pageData: {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	readyText: 'Credits'
};
