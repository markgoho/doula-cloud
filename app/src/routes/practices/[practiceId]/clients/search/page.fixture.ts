/*
 * Finding a Client, as the continuum check sees it (#595).
 *
 * `isContractor: false` from `+page.ts`'s own load takes the real search
 * form rather than the contractor-Doula explain-only door. No fetch
 * fires until a search is submitted, which the sweep never simulates, so
 * there is no Practice-typed free text on screen at mount.
 *
 * Two sessions (#928), and this route is the clearest case of the pair
 * being two screens rather than one screen twice: `isAmbientContractor`
 * chooses between two whole templates, with different `<h1>`s, different
 * document titles, and not one element in common below the layout
 * primitives. Neither is a subset of the other.
 */
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import Page from './+page.svelte';

/*
 * ADR-0017's "a contractor originates nothing" (#501): in place of the
 * search form she would be refused from, one paragraph explaining why
 * and one link out to setting up a Practice of her own.
 *
 * `isAmbientContractor` arrives through `+page.ts`'s own `load` as
 * `data.isContractor`, so this restates `props` -- setting a session in
 * `pageData` would mount the base tree a second time and the sweep would
 * stay green about it. `readyText` moves with the branch: the heading
 * really is different, so the inherited one would never appear and the
 * mount would wait for a heading that is not coming.
 *
 * Nothing here is Practice-typed -- both the paragraph and the link are
 * this repo's copy -- so this branch has no hostile value to choose. What
 * it has to survive at 320px is one long unbroken sentence, which is the
 * measurement.
 */
export const asContractor: RouteVariant = {
	name: 'Adding a Client, as a contractor Doula',
	props: { data: { isContractor: true } },
	readyText: 'Add a Client'
};

export const fixture: RouteFixture = {
	name: 'Finding a Client',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/clients/search',
	props: { data: { isContractor: false } },
	readyText: 'Find a Client',
	variants: [asContractor]
};
