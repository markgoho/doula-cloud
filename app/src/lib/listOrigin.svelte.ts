import { afterNavigate, goto } from '$app/navigation';

/*
 * The list an Engagement was opened from (#2864), for the Engagement page's
 * Back link. The Schedule and the Invoices list each link their rows to an
 * Engagement; the page reads where the reader came from and names it. The
 * typed search is in no URL (#1829): the list keeps it in its history
 * entry's state, so the Back link either goes back through history, which
 * restores it, or, when the page has moved on since, hands the held search
 * back through `goto`'s state. Memory only, as `clientsReturn.ts` is.
 */
export type OriginList = 'schedule' | 'invoices' | 'on-call';

export interface ListOrigin {
	list: OriginList;
	address: string;
}

const LABEL: Record<OriginList, string> = {
	schedule: 'Back to the Schedule',
	invoices: 'Back to Invoices',
	'on-call': 'Back to On call'
};

const held: Record<OriginList, string> = { schedule: '', invoices: '', 'on-call': '' };

export function holdListSearch(list: OriginList, search: string): void {
	held[list] = search;
}

/**
 * The list `from` is, with the address's filters kept, or nothing when it is
 * neither list. `paths` holds each list's own path for this Practice.
 */
export function originFrom(from: URL | undefined, paths: Partial<Record<OriginList, string>>): ListOrigin | undefined {
	if (!from) return undefined;
	const list = (Object.keys(paths) as OriginList[]).find((key) => paths[key] === from.pathname);
	return list ? { list, address: `${from.pathname}${from.search}` } : undefined;
}

/**
 * Where the page was opened from. Made while the route's own navigation
 * runs, before any read has landed, so the loading frame and the page it
 * becomes name the same list.
 */
export class ListBack {
	#arrivedAt = '';
	origin = $state<ListOrigin>();

	/*
	 * The browser's Back restores the list's place and its search with its
	 * history entry, so straight from the list this goes back through
	 * history. Once the address has moved (a "Jump to" link adds an entry
	 * with no navigation event), the entry before it is not the list:
	 * the link goes to the list's address and hands the held search back as
	 * the entry's state, never in the address.
	 */
	go = (event: MouseEvent): void => {
		const origin = this.origin;
		if (!origin || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
		event.preventDefault();
		if (location.href === this.#arrivedAt) {
			history.back();
			return;
		}
		const search = held[origin.list];
		const states = { schedule: { scheduleSearch: search }, invoices: { invoicesSearch: search }, 'on-call': {} };
		void goto(origin.address, { state: states[origin.list] });
	};

	constructor(paths: () => Partial<Record<OriginList, string>>, here: () => string) {
		afterNavigate(({ from }) => {
			const found = originFrom(from?.url, paths());
			if (found) {
				this.origin = found;
				this.#arrivedAt = location.href;
			} else if (from?.url.pathname !== here()) {
				this.origin = undefined;
			}
		});
	}

	href(otherwise: string): string {
		return this.origin?.address ?? otherwise;
	}

	label(otherwise: string): string {
		return this.origin ? LABEL[this.origin.list] : otherwise;
	}
}
