import { beforeNavigate, snapshot } from '$app/navigation';
import { tick } from 'svelte';

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

/**
 * Puts a list back where it was left: scrolled to the held offset, then
 * focus on the row she left from. The row may sit under a stuck table header
 * (#2839); `nearest` moves it clear only if it does.
 */
export function putListBack(back: ListReturn, rowLink: (rowId: string) => HTMLElement | undefined): void {
	window.scrollTo(0, back.scrollY);
	const link = back.rowId ? rowLink(back.rowId) : undefined;
	link?.focus({ preventScroll: true });
	link?.scrollIntoView({ block: 'nearest' });
}

/**
 * Reads a list as many pages again as it held when she left, then puts it
 * back where it was (`putListBack`). `navigations` counts the moves the list
 * has seen since, so a move while its pages load stops it short of
 * scrolling: it is never put on another view's rows. `loadMore` answers
 * whether it added any, so the pages running out ends the reading.
 */
export async function restoreList(
	back: ListReturn,
	list: {
		pagesLoaded: () => number;
		navigations: () => number;
		loadMore: () => Promise<boolean>;
		rowLink: (rowId: string) => HTMLElement | undefined;
	}
): Promise<void> {
	const at = list.navigations();
	while (at === list.navigations() && list.pagesLoaded() < back.pages && (await list.loadMore()));
	await tick();
	if (at !== list.navigations()) return;
	putListBack(back, list.rowLink);
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
