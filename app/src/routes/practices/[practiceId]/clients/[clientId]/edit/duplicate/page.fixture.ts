/*
 * Gate two's question, on the edit path (ADR-0017's amendment, #814), as
 * the continuum check sees it (#595).
 *
 * Seeded through `editMergeDraft` directly rather than through
 * `respond()`, the same reason `clients/new/duplicate/page.fixture.ts`
 * seeds `intakeDraft`: the sweep mounts this route on its own, without
 * the edit page's 409 that would normally fill the draft.
 *
 * Two matches, not one (`.claude/rules/svelte-tests.md`'s row rule): one
 * with `wouldSurvive: true` and every optional column filled in at its
 * longest, one with `wouldSurvive: false` and every optional column
 * blank -- so both merge directions and both of `matchDescription`'s
 * "on file" / "nothing on file" branches render on the swept screen.
 */
import type { ClientEditFields, CollisionMatch } from '#lib/client.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import type { RouteFixture } from '../../../../../../routeFixture.js';
import Page from './+page.svelte';

export const practiceId = 'practice-1';
export const clientId = 'client-1';

export const fields: ClientEditFields = {
	givenName: 'Anne-Marie',
	familyName: 'Ochieng-Whitfield',
	preferredName: '',
	email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
	phone: '+1 (585) 555-0142',
	addressLine1: '4827 Pittsford-Mendon Center Road',
	addressLine2: '',
	addressLocality: 'Honeoye Falls',
	addressRegion: 'NY',
	addressPostalCode: '14472',
	dateOfBirth: '1988-02-09',
	fieldValues: {}
};

export const matches: CollisionMatch[] = [
	{
		id: 'client-2',
		givenName: 'Anne-Marie',
		familyName: 'Ochieng-Whitfield',
		preferredName: '',
		email: 'anne-marie.ochieng-whitfield@finger-lakes-midwifery.example.com',
		phone: '+1 (585) 555-0199',
		addressLine1: '12 Highland Park Boulevard',
		addressLine2: 'Apartment 4B',
		addressLocality: 'Rochester',
		addressRegion: 'NY',
		addressPostalCode: '14620',
		dateOfBirth: '1988-02-09',
		wouldSurvive: true,
		engagements: [
			{ engagementId: 'e1', kind: 'birth', status: 'completed', createdAt: '2024-03-01T00:00:00Z' },
			{ engagementId: 'e2', kind: 'postpartum', status: 'active', createdAt: '2026-01-04T00:00:00Z' }
		]
	},
	{
		id: 'client-3',
		givenName: 'Anne-Marie',
		familyName: 'Ochieng-Whitfield',
		preferredName: '',
		email: '',
		phone: '',
		addressLine1: '',
		addressLine2: '',
		addressLocality: '',
		addressRegion: '',
		addressPostalCode: '',
		dateOfBirth: '',
		wouldSurvive: false,
		engagements: []
	}
];

/**
 * Fills `editMergeDraft`, the same module state gate two's 409 would have
 * filled -- called at import time so a `+page.svelte` mounted on its own
 * renders the screen a reader would actually meet.
 */
export function seedEditMergeDraft(): void {
	editMergeDraft.open(clientId, fields, matches, true);
}

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the edit page's 409 that would have filled it, and the module has no other moment to do it in.
seedEditMergeDraft();

export const fixture: RouteFixture = {
	name: 'The Client edit duplicate check',
	component: Page,
	params: { practiceId, clientId },
	url: 'https://example.test/practices/practice-1/clients/client-1/edit/duplicate',
	readyText: 'Is this the same person?'
};
