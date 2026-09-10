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
 * One session is all this screen has (#928), and now the route says so
 * itself. It reads no role at all: who may be here is ADR-0008's Credit
 * row, the balance read and the purchase both declare
 * `staffauth.OwnerAndAdmin` (`billing/mount.go`), and the balance arrives
 * through `+page.ts`'s own `load` -- so a Doula meets `refuseRead` and
 * `practices/+error.svelte` instead of this component, and an Owner and
 * an Admin draw the identical screen with the Buy credits button plainly
 * enabled.
 * [#1162](https://github.com/markgoho/doula-cloud/issues/1162) removed the
 * disabled second tree this comment used to have to explain away.
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
