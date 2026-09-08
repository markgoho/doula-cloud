/**
 * The one way this dashboard reads one of its own `+server.ts` routes.
 *
 * Every route here fails the same way — a status the browser can see and a
 * `{ message }` body — so every read recovers the same way too.
 */

interface FailureBody {
	readonly message?: unknown;
}

async function readFailureMessage(response: Response, fallbackMessage: string): Promise<string> {
	try {
		const body = (await response.json()) as FailureBody;
		if (typeof body.message === 'string') return body.message;
	} catch {
		// A failure that is not even JSON explains itself no better than the
		// fallback does.
	}

	return fallbackMessage;
}

/**
 * Reads one route, or throws with whatever it said went wrong.
 *
 * `fetch` is a parameter rather than the global so a spec can drive every
 * branch without a network. Nothing is cached: each call is a new read, which
 * is the whole sync model for this tool.
 *
 * `fallbackMessage` names the upstream this route depends on, because a sync
 * pulls from more than one and a banner has to say which one failed.
 */
export async function readRoute<T>(
	fetcher: typeof fetch,
	endpoint: string,
	fallbackMessage: string
): Promise<T> {
	const response = await fetcher(endpoint, { cache: 'no-store' });

	if (!response.ok) throw new Error(await readFailureMessage(response, fallbackMessage));

	return (await response.json()) as T;
}
