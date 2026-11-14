/**
 * The name `practices/[practiceId]/+layout.ts`'s `load` declares with
 * `depends`, so an act that changes what the Practice session says can
 * have it read again on the navigation that follows (#2183). The load
 * otherwise reruns only when the Practice changes. Saving a Practice's
 * first Client is such an act: the session's `hasClients` turns true and
 * the Staff top bar offers the sections that were waiting for it.
 *
 * It lives here rather than in `+layout.ts`, which may export only what
 * SvelteKit reads from it.
 */
export const PRACTICE_SESSION_DEPENDENCY = 'practice:session';

const lastRead: { session?: { practiceId: string } } = {};

/**
 * The Practice session the last successful read returned, kept in this
 * document so a read that cannot reach the service (no connection) leaves
 * the screen where it is instead of replacing it with the error page.
 * Forgotten at sign-out, so no one is handed another's Practice.
 */
export function noteSessionRead(session: { practiceId: string }): void {
	lastRead.session = session;
}

export function sessionHeldFor<Session extends { practiceId: string }>(practiceId: string): Session | undefined {
	return lastRead.session?.practiceId === practiceId ? (lastRead.session as Session) : undefined;
}

export function forgetSessionRead(): void {
	lastRead.session = undefined;
}
