import { postAct, refusal, type Outcome } from './act.ts';

export type LegalVersion = {
	document: string;
	documentName: string;
	version: string;
	change: string;
	material: boolean;
	publishedAt: string | null;
};

export type LegalVersionCheck = LegalVersion & { owners: number };

export type LegalPublication = {
	document: string;
	documentName: string;
	version: string;
	material: boolean;
	publishedAt: string;
	queued: number;
};

type Fetch = typeof fetch;

export function owners(isMaterial: boolean, count: number): string {
	if (!isMaterial) return 'None. A version that is not material emails no one.';
	return `${count} ${count === 1 ? 'Owner' : 'Owners'}`;
}

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

export async function readLegalVersionCheck(
	document: string,
	version: string,
	fetcher: Fetch = fetch
): Promise<Outcome<LegalVersionCheck>> {
	const response = await fetcher(versionPath(document, version), { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

export async function sendLegalPublication(
	document: string,
	version: string,
	key: string,
	fetcher: Fetch = fetch
): Promise<Outcome<LegalPublication>> {
	const response = await postAct(`${versionPath(document, version)}/publications`, key, {}, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
