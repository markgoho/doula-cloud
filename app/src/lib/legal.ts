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
