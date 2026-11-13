import { beforeNavigate, snapshot } from '$app/navigation';

/*
 * Coming back to a paged list (#2746, T3): the list keeps how many pages it
 * held, how far down she was and the row she left for, and a Back to it
 * reads as many pages again. Capped, so a reader who paged a long way gets
 * a sane read rather than the whole book. The Clients list and the
 * Invoices list both read it.
 */
export const MAX_RESTORED_PAGES = 5;

export function pagesToRestore(held: number | undefined): number {
	return Math.min(held ?? 1, MAX_RESTORED_PAGES);
}

export interface ListReturn {
	pages: number;
	scrollY: number;
	rowId?: string;
}

/**
 * Keeps a list's place in its own history entry, through SvelteKit's
 * `snapshot`, which captures synchronously before the navigation writes
 * the next entry. A `replaceState` called on the way out resolves the route
 * first and so could land on the entry being opened, which then carried the
 * list's address and its place (#2746). Called during the list's
 * initialization; `rowOf` names the row a navigation leaves for, if any.
 */
export function keepListReturn(options: {
	id: string;
	pagesLoaded: () => number;
	rowOf: (to: URL) => string | undefined;
	restore: (back: ListReturn) => void;
}): void {
	let rowId: string | undefined;
	beforeNavigate(({ to }) => {
		rowId = to ? options.rowOf(to.url) : undefined;
	});
	snapshot<ListReturn>({
		id: options.id,
		capture: () => ({ pages: options.pagesLoaded(), scrollY: window.scrollY, rowId }),
		restore: (held) => options.restore({ ...held, pages: pagesToRestore(held.pages) })
	});
}
