import { refusal, type Outcome } from './act.ts';

/**
One version of the Terms of Service or the Privacy Policy, named by the date it takes effect.
*/
export type LegalVersion = {
	document: string;
	documentName: string;
	version: string;
	change: string;
	material: boolean;
	publishedAt: string | null;
};

/**
What the check page states before a version is published.
*/
export type LegalVersionCheck = LegalVersion & { owners: number };

/**
What a publication changed.
*/
export type LegalPublication = {
	document: string;
	documentName: string;
	version: string;
	material: boolean;
	publishedAt: string;
	queued: number;
};

type Fetch = typeof fetch;

/**
Who a version emails, in words.
*/
export function owners(isMaterial: boolean, count: number): string {
	if (!isMaterial) return 'None. A version that is not material emails no one.';
	return `${count} ${count === 1 ? 'Owner' : 'Owners'}`;
}

/**
Every version in the history, each document's newest first.
*/
export async function readLegalVersions(fetcher: Fetch = fetch): Promise<Outcome<LegalVersion[]>> {
	const response = await fetcher('/api/legal-versions', { credentials: 'include' });
	if (response.ok) {
		const list: { items: LegalVersion[] } = await response.json();
		return { kind: 'ok', value: list.items };
	}
	return refusal(response);
}

function versionPath(document: string, version: string): string {
	return `/api/legal-versions/${encodeURIComponent(document)}/${encodeURIComponent(version)}`;
}

/**
The version the check page names, and how many Owners publishing it emails.
*/
export async function readLegalVersionCheck(
	document: string,
	version: string,
	fetcher: Fetch = fetch
): Promise<Outcome<LegalVersionCheck>> {
	const response = await fetcher(versionPath(document, version), { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
 * Publishes a version under key, the name of this one request: a retry
 * with the same key is answered with the publication already made. The
 * body names no one; doula-ops takes the operator from the session.
 */
export async function sendLegalPublication(
	document: string,
	version: string,
	key: string,
	fetcher: Fetch = fetch
): Promise<Outcome<LegalPublication>> {
	const response = await fetcher(`${versionPath(document, version)}/publications`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
		body: '{}'
	});
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
