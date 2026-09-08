/*
 * The Rates screen, as the continuum check sees it (#966).
 *
 * A postpartum rate left unset, alongside a birth rate that is set, is
 * the hostile case ADR-0025 asks a fixture to realize: #966's AC that a
 * Practice with no rate set is a valid state, so the continuum sweep
 * measures the screen with one field blank rather than only the tidy
 * both-filled case.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RatesResponse } from '#lib/rates.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const rates: RatesResponse = {
	rates: [
		{ kind: 'birth', amountCents: 250_000 },
		// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
		{ kind: 'postpartum', amountCents: null }
	]
};

export const fixture: RouteFixture = {
	name: 'Rates',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/rates',
	respond: () => jsonResponse(rates),
	readyText: 'Rates'
};
