/*
 * The Client every screen under her record is drawn for: her own details,
 * as the Client hub and the new-work request both read them (#595).
 */
import type { ClientDetail } from '#lib/clientDetail.js';

export const persephone = {
	id: 'client-1',
	standing: 'live',
	givenName: 'Persephone',
	familyName: 'Ochieng-Whitfield',
	preferredName: '',
	email: 'persephone@example.test',
	phone: '585-555-0101',
	addressLine1: '100 Highland Ave',
	addressLine2: '',
	addressLocality: 'Rochester',
	addressRegion: 'NY',
	addressPostalCode: '14620',
	dateOfBirth: '1994-03-01',
} satisfies Partial<ClientDetail>;
