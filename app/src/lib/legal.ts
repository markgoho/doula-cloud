/**
The Terms of Service on the marketing site (#1556). An Owner agrees to it, for her Practice, by the sentence above the signup form's button (#1547, ADR-0053).
*/
export const termsOfServiceUrl = 'https://doula.cloud/terms';

/**
The Privacy Policy on the marketing site (#1556). An Owner agrees to it at signup with the Terms; on every other screen it is a notice, and nobody else agrees to it (ADR-0053).
*/
export const privacyPolicyUrl = 'https://doula.cloud/privacy';

/**
The page of one version of a document on the marketing site, at `/terms/<date>` or `/privacy/<date>` (#1816). The screen that asks an Owner to agree to a material change links the version she agrees to, so she reads its own text and not whatever `/terms` shows that day (#1549).
*/
export function legalVersionUrl(document: 'terms' | 'privacy', version: string): string {
	return `${document === 'terms' ? termsOfServiceUrl : privacyPolicyUrl}/${version}`;
}

/**
One version of the Terms of Service or the Privacy Policy that has not started yet, as `GET /api/legal/coming` answers it (`staffauth.ComingVersion`). A Practice that signs up before the date agrees to it as well, and the signup form names it (#1824, ADR-0053).
*/
export interface ComingVersion {
	document: 'terms' | 'privacy';
	name: string;
	// The date it starts, YYYY-MM-DD.
	version: string;
}

/**
The BFF read the signup form names the coming versions from. It is a read and not a list built into the app, because the `terms_accepted` row the press writes takes its coming versions from the same history at the server's instant, so the form and the record cannot name different versions (#1824).
*/
export const comingVersionsPath = '/api/legal/coming';
