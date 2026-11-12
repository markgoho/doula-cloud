/*
 * Coming back to a paged list (#2746, T3): the list keeps how many pages it
 * held in its history entry's state, and a Back to it reads as many again.
 * Capped, so a reader who paged a long way gets a sane read rather than the
 * whole book. The Clients list and the Invoices list both read it.
 */
export const MAX_RESTORED_PAGES = 5;

export function pagesToRestore(held: number | undefined): number {
	return Math.min(held ?? 1, MAX_RESTORED_PAGES);
}
