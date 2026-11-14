<script lang="ts">
	import { page } from '#lib/appState.svelte.js';
	import { afterNavigate, beforeNavigate, goto, replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		CLIENT_ORDER_OPTIONS,
		clientOrderFrom,
		clientsResultSummary,
		loadClients,
		NO_CARE_YET,
		UNKNOWN_DOULA,
		type ClientListItem,
		type ClientOrder,
		type OpenEngagement
	} from '#lib/client.js';
	import { RefusalError } from '#lib/formErrors.js';
	import { holdClientsName } from '#lib/clientsReturn.js';
	import { keepListReturn, pagesToRestore, putListBack, type ListReturn } from '#lib/listReturn.js';
	import { onMount, tick, untrack } from 'svelte';
	import { kindLabel, NO_DOULA_YET_LABEL } from '#lib/engagementRequest.js';
	import { engagementStatusText } from '#lib/engagementDetail.js';
	import { startHref } from './new/intake.js';
	import { formatCalendarDay, formatPostpartum } from '#lib/dates.js';
	import { practiceTimeZone } from '#lib/practiceZone.js';
	import { formatAmount, invoiceMoneyState } from '#lib/invoice.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import type { LabeledValue } from '#lib/roles.js';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import MoneyAmount from '#lib/components/molecules/MoneyAmount.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { RouteId } from '$app/types';
import type { PageProps as PageProperties } from './$types';

	// #539 (ADR-0017): +page.ts's load already decided, before this
	// component mounted, whether the caller is a contractor Doula with
	// neither the owner nor the admin role. `data` is optional only so
	// this component's own spec (which renders it directly, bypassing
	// SvelteKit's load cycle) keeps working without a fixture -- every
	// real navigation always supplies it.
	let { data }: { data?: PageProperties['data'] } = $props();
	const isContractor = $derived(data?.isContractor ?? false);
	// #2624: "Only my Clients" is drawn for a Doula who is not a contractor,
	// whose list is already only hers, and where another Doula holds the role
	// (#2675), since at a Practice with one Doula every Client is hers.
	// She must also hold at least one live Client, or the switch could never match.
	let hasMine = $state(false);
	$effect(() => {
		const gate = data?.hasMine ?? Promise.resolve(true);
		let isCurrent = true;
		void gate.then((answer) => {
			if (isCurrent) hasMine = answer;
		});
		return () => {
			isCurrent = false;
		};
	});
	const canNarrowToMine = $derived(!isContractor && (data?.isDoula ?? false) && (data?.hasOtherDoulas ?? true));
	// #1609: whether the Practice holds any Client record, which decides
	// where the header link goes. Defaults to the search, which is right
	// at a Practice of any size. Undefined while +page.ts's read runs
	// (#2249).
	let hasAnyClient = $state<boolean>();
	$effect(() => {
		const gate = data?.hasAnyClient ?? Promise.resolve(true);
		let isCurrent = true;
		void gate.then((answer) => {
			if (isCurrent) hasAnyClient = answer;
		});
		return () => {
			isCurrent = false;
		};
	});

	// The Practice's Doulas for the "Doula" filter, read by +page.ts for an
	// Owner or an Admin and empty for everyone else, so the filter is not
	// drawn for them.
	let doulas = $state.raw<LabeledValue[]>([]);
	$effect(() => {
		const gate = data?.doulas ?? Promise.resolve([]);
		let isCurrent = true;
		void gate.then((options) => {
			if (isCurrent) doulas = options;
		});
		return () => {
			isCurrent = false;
		};
	});

	// What a person reads when a read fails: no status code and no server
	// text, and the first line says nothing is lost.
	const LOAD_FAILED = 'Your Clients did not load. Nothing you saved is lost.';
	const LOAD_MORE_FAILED = 'More Clients did not load. Nothing you saved is lost. Try again.';

	// Starts empty: unlike Billing and Invoices this screen fetches its own
	// first page in the effect below rather than through a load, so every
	// page it holds arrives through this list.
	const clients = new PaginatedList<ClientListItem>({
		first: { items: [], hasMore: false },
		loadPage: async (cursor) => {
			try {
				return await loadClients(apiFetchWithSession, page.params.practiceId!, { cursor, ...appliedView() });
			} catch {
				throw new Error(LOAD_MORE_FAILED);
			}
		},
		failureMessage: LOAD_MORE_FAILED
	});
	let error = $state('');
	let isLoaded = $state(false);
	// A later filter's page is on its way while the rows of the last one stay up.
	let isRefreshing = $state(false);
	// Apply was pressed and the address has not yet changed.
	let isApplying = $state(false);
	const isBusy = $derived(isRefreshing || isApplying);
	// The address names a Doula the Practice does not hold, which the BFF refuses.
	let isDoulaUnknown = $state(false);
	const isReady = $derived(isLoaded && hasAnyClient !== undefined);

	// The default filter is Clients in care (ADR-0017); `?all=true`
	// switches to everyone -- including a Client whose only Request was
	// refused. A URL param, not local state: shareable, survives a
	// reload, and works with the back button, all for free from
	// SvelteKit's own navigation (#499).
	let isShowingEveryone = $derived(page.url.searchParams.get('all') === 'true');

	// #2624: `?mine=true` narrows the list to the Clients she is attached
	// to, in the address for the same reasons. A Client not yet in care has no
	// Doula, so the two switches never both apply.
	let isShowingMine = $derived(canNarrowToMine && page.url.searchParams.get('mine') === 'true');

	// #2724: `?owes=true` and `?noContract=true` narrow it the same way, to
	// the Clients who owe money and the ones with an Engagement under way and
	// no Contract. Money is not shown to a contractor Doula (ADR-0008), so
	// the first never applies to her.
	let isShowingOwing = $derived(!isContractor && page.url.searchParams.get('owes') === 'true');
	let isShowingNoContract = $derived(page.url.searchParams.get('noContract') === 'true');

	// #2724 and after: `?doula=<staffId>` narrows to one Doula's Clients and
	// a typed name to a name. Neither is money, so they apply to everyone. The
	// Doula stays in the address like the rest; the name does not, since a
	// Client's name is in no URL (#1829): it rides in the history entry's
	// state (`goto(url, { state })`), so Back and Forward restore it and a
	// reload drops it. The BFF ANDs them with "Only my Clients", so both
	// together are simply a narrower list.
	let appliedDoula = $derived(page.url.searchParams.get('doula') ?? '');
	let appliedName = $derived((page.state.clientsName ?? '').trim());
	// #2869: `?order=` is in the address too. A contractor Doula is offered no
	// order (her bar is not drawn), and the BFF refuses "owed" to her.
	let appliedOrder = $derived(isContractor ? '' : clientOrderFrom(page.url.searchParams.get('order')));
	// Left for the Client hub's Back link, which hands it back (`clientsReturn.ts`).
	$effect(() => {
		holdClientsName(appliedName);
	});
	const isFiltered = $derived(
		isShowingEveryone || isShowingMine || isShowingOwing || isShowingNoContract || appliedDoula !== '' || appliedName !== '' || appliedOrder !== ''
	);

	// Mine, owing, no Contract and a Doula each imply an Engagement, which is
	// all the switch for Clients with no care could add to them, so it is not
	// drawn beside one. A typed name implies nothing, so it stays.
	const isImplyingCare = $derived(isShowingMine || isShowingOwing || isShowingNoContract || appliedDoula !== '');

	// The result line's id, which the table is described by.
	const RESULT_ID = 'clients-result';

	// Every filter the address names: all but the name (#1829).
	type ClientsView = {
		all?: boolean;
		mine?: boolean;
		owing?: boolean;
		noContract?: boolean;
		doula?: string;
		order?: ClientOrder;
	};

	// The names are the ones the filter bar's own fields carry, so its form
	// reaches this address with no script running. `all` stays in the address
	// beside a narrowing: the BFF ANDs them, and dropping it would untick a box
	// the person ticked.
	function clientsPath({ all, mine, owing, noContract, doula, order }: ClientsView): string {
		const base = resolve('/practices/[practiceId]/clients', {
			practiceId: page.params.practiceId!
		});
		const parts = [
			mine && 'mine=true',
			owing && 'owes=true',
			noContract && 'noContract=true',
			doula && `doula=${encodeURIComponent(doula)}`,
			all && 'all=true',
			order && `order=${order}`
		].filter(Boolean);
		return parts.length > 0 ? `${base}?${parts.join('&')}` : base;
	}

	// The view the address names, as `loadClients` takes it.
	function appliedView() {
		return {
			showAll: isShowingEveryone,
			showMine: isShowingMine,
			showOwing: isShowingOwing,
			showNoContract: isShowingNoContract,
			doula: appliedDoula,
			name: appliedName,
			order: appliedOrder
		};
	}

	/*
	 * Where focus goes once a move to this list has loaded (the GOV.UK
	 * pattern for a page that changes under a keyboard user; `Heading`'s
	 * `focusOnAppearing` is the same idea for a heading). Left alone, focus
	 * falls to the top of the page and Tab has to cross the filter bar to
	 * reach the result.
	 *
	 * - A change of filter in the address (Apply, "Clear filters", back or
	 *   forward) lands on the result line. Never on first arrival, where the
	 *   browser and the route announcer already say where she is.
	 * - Coming back from a Client's hub lands on that Client's row link, so
	 *   she resumes where she left; the result line if the row is not on the
	 *   loaded page.
	 *
	 * It waits for the list that matches the address, so it never lands on
	 * the rows being replaced.
	 */
	const CLIENT_HUB_ROUTE = '/practices/[practiceId]/clients/[clientId]' satisfies RouteId;
	const filterKey = $derived(JSON.stringify(appliedView()));
	let loadedKey = $state('');
	let filterBar = $state<{ focusStatus: () => void }>();
	let pendingFocus = $state<{ clientId?: string }>();
	let lastFilterKey: string | undefined;

	afterNavigate(({ from }) => {
		isLeaving = false;
		seedDrafts();
		if (from?.route.id === CLIENT_HUB_ROUTE) {
			pendingFocus = { clientId: from.params!.clientId };
		} else if (lastFilterKey !== undefined && lastFilterKey !== filterKey) {
			pendingFocus = {};
		}
		lastFilterKey = filterKey;
	});

	const rowDomId = (clientId: string, view: 'table' | 'record') => `client-row-${clientId}-${view}`;

	// Both trees are in the DOM (DataTable's #666); the displayed one wins.
	function rowLink(clientId: string): HTMLElement | undefined {
		for (const view of ['table', 'record'] as const) {
			const link = document.querySelector<HTMLElement>(`#${rowDomId(clientId, view)} a`);
			if (link?.checkVisibility()) return link;
		}
		return undefined;
	}

	/*
	 * Back from a Client also puts her where she was (`keepListReturn`, #2840).
	 * The pages themselves come back through `loadFirstPage` below, so the
	 * place is held until that read lands and is dropped if it fails or any
	 * move comes first: it is never put on another view's rows.
	 */
	let heldReturn: ListReturn | undefined;
	keepListReturn({
		id: 'clients-list',
		pagesLoaded: () => pagesLoaded,
		rowOf: (to) => {
			const base = `${page.url.pathname}/`;
			return to.pathname.startsWith(base) ? to.pathname.slice(base.length).split('/', 1)[0] : undefined;
		},
		restore: (back) => {
			if (error) return;
			heldReturn = back;
			if (isLoaded && !isRefreshing && loadedKey === filterKey) void restorePlace();
		}
	});

	async function restorePlace() {
		const back = heldReturn;
		heldReturn = undefined;
		await tick();
		if (back) putListBack(back, (clientId) => rowLink(clientId));
	}

	$effect(() => {
		if (!pendingFocus || !isReady || loadedKey !== filterKey) return;
		const { clientId } = pendingFocus;
		pendingFocus = undefined;
		const link = clientId === undefined ? undefined : rowLink(clientId);
		if (link) link.focus();
		else filterBar?.focusStatus();
	});

	// What is in the filter bar's controls before it is applied. Seeded from
	// the address and put back in step after every navigation, as the On call
	// screen does with its dates.
	let isAllChecked = $state(false);
	let isMineChecked = $state(false);
	let isOwingChecked = $state(false);
	let isNoContractChecked = $state(false);
	let draftDoula = $state('');
	let draftName = $state('');
	let draftOrder = $state<ClientOrder>('');
	// Once per navigation, never on a re-run of an effect: a box ticked in the
	// moment before the page hydrated was overwritten by the address it had
	// not yet been applied to.
	//
	// A control the person changed since the navigation began is hers: the seed
	// never overwrites it. `beforeNavigate` clears the marks, so the next move
	// seeds every control again.
	let touched: string[] = [];
	function seedDrafts() {
		if (!touched.includes('all')) isAllChecked = isShowingEveryone;
		if (!touched.includes('mine')) isMineChecked = isShowingMine;
		if (!touched.includes('owes')) isOwingChecked = isShowingOwing;
		if (!touched.includes('noContract')) isNoContractChecked = isShowingNoContract;
		if (!touched.includes('doula')) draftDoula = appliedDoula;
		if (!touched.includes('name')) draftName = appliedName;
		if (!touched.includes('order')) draftOrder = appliedOrder;
	}
	seedDrafts();
	// A page that lands once she has left for another screen is not stamped
	// (`loadMore`): `replaceState` resolves the route first, so it would
	// write this list's address and pages onto the entry being opened.
	let isLeaving = false;
	beforeNavigate((leaving) => {
		touched = [];
		heldReturn = undefined;
		isLeaving = leaving?.to?.url.pathname !== page.url.pathname;
	});

	// The field's wrapper, whose form is read when the page takes over.
	let nameField = $state<HTMLElement>();

	// Nothing is disabled before the page takes over, so a name typed or a box
	// ticked in that moment is in the controls but not yet in the drafts. Read
	// them from the DOM before the address seeds anything, and mark each one
	// the person's own so the seed leaves it alone. The Name has no `name`
	// attribute (#1829), so it is read from its input.
	onMount(() => {
		const form = nameField?.closest('form');
		if (!form) return;
		// A switch the bar does not draw has no box to read, and keeps its address.
		const adoptChecked = (key: string, isDraftChecked: boolean, set: (isChecked: boolean) => void) => {
			// The "Also show" box is in the form twice (one for each bar), so read every box of the name.
			const boxes = [...form.querySelectorAll<HTMLInputElement>(`input[name="${CSS.escape(key)}"]`)];
			if (boxes.length === 0) return;
			const isChecked = boxes.some((box) => box.checked);
			if (isChecked === isDraftChecked) return;
			touched.push(key);
			set(isChecked);
		};
		adoptChecked('all', isAllChecked, (value) => (isAllChecked = value));
		adoptChecked('mine', isMineChecked, (value) => (isMineChecked = value));
		adoptChecked('owes', isOwingChecked, (value) => (isOwingChecked = value));
		adoptChecked('noContract', isNoContractChecked, (value) => (isNoContractChecked = value));
		const typed = nameField!.querySelector('input')!.value;
		if (typed !== draftName) {
			touched.push('name');
			draftName = typed;
		}
	});

	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property Checkbox's onChange wraps 1:1 (same exception Checkbox.svelte itself carries)
	const handleToggleAll = (checked: boolean) => {
		touched.push('all');
		isAllChecked = checked;
	};
	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property Checkbox's onChange wraps 1:1 (same exception Checkbox.svelte itself carries)
	const handleToggleMine = (checked: boolean) => {
		touched.push('mine');
		isMineChecked = checked;
	};
	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property Checkbox's onChange wraps 1:1 (same exception Checkbox.svelte itself carries)
	const handleToggleOwing = (checked: boolean) => {
		touched.push('owes');
		isOwingChecked = checked;
	};
	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property Checkbox's onChange wraps 1:1 (same exception Checkbox.svelte itself carries)
	const handleToggleNoContract = (checked: boolean) => {
		touched.push('noContract');
		isNoContractChecked = checked;
	};

	// The filter bar's own form, upgraded to a client-side navigation
	// (Rule of Least Power).
	async function applyFilters(event: SubmitEvent) {
		event.preventDefault();
		// Before the navigation, so a "Load more" already in flight is
		// abandoned rather than merged into the new filter's list. The rows
		// stay up until the new filter's first page arrives.
		clients.abandon();
		// Apply answers the press at once: the navigation has to run its load
		// before the address changes and the list's own request starts.
		isApplying = true;
		try {
			await goto(
				clientsPath({
					all: isAllChecked,
					mine: isMineChecked,
					owing: isOwingChecked,
					noContract: isNoContractChecked,
					doula: draftDoula,
					order: draftOrder
				}),
				{ state: { clientsName: draftName.trim() } }
			);
		} finally {
			isApplying = false;
		}
	}

	const doulaOptions = $derived([{ value: '', label: 'Any Doula' }, ...doulas]);
	// The Doula's name for the result sentence; the id's place if the roster
	// has not landed, or does not list her (a Doula who holds no Client is left
	// out of it).
	function doulaNameOf(id: string): string {
		if (id === '') return '';
		return doulas.find((doula) => doula.value === id)?.label ?? 'that Doula';
	}

	// The GOV.UK "selected filters": one removable tag for each filter the
	// address names, and one for the typed name. Each is a link to the list
	// minus that one filter, so one press removes one. The name is in no
	// address (#1829), so removing any other filter hands it on in the history
	// entry's state, and removing the name hands on an empty one.
	type AppliedTag = { text: string; href: string; name: string };
	const appliedTags = $derived.by(() => {
		const view: ClientsView = {
			all: isShowingEveryone,
			mine: isShowingMine,
			owing: isShowingOwing,
			noContract: isShowingNoContract,
			doula: appliedDoula,
			order: appliedOrder
		};
		const tags: AppliedTag[] = [];
		const add = (text: string, without: ClientsView, name = appliedName) => {
			tags.push({ text, href: clientsPath({ ...view, ...without }), name });
		};
		if (isShowingOwing) add('Who owe money', { owing: false });
		if (isShowingNoContract) add('Active, with no Contract', { noContract: false });
		if (isShowingMine) add('Mine', { mine: false });
		if (appliedDoula) add(`Doula: ${doulaNameOf(appliedDoula)}`, { doula: '' });
		if (isShowingEveryone) add(`Including ${NO_CARE_YET}`, { all: false });
		if (appliedName) add(`Name: ${appliedName}`, {}, '');
		return tags;
	});

	async function removeTag(event: MouseEvent, tag: AppliedTag) {
		event.preventDefault();
		clients.abandon();
		isApplying = true;
		try {
			await goto(tag.href, { state: { clientsName: tag.name } });
		} finally {
			isApplying = false;
		}
	}

	// The longest the count line can say without a typed name: a Doula's (or the signed-in
	// Doula's) first page of active Clients who owe money and have no Contract. The bar
	// draws it unseen, so the line reserves one line where that fits on one and two where
	// it does not.
	const longestSummary = $derived(
		[
			clientsResultSummary(
				99,
				true,
				false,
				false,
				true,
				true,
				{ doulaName: doulas.map((doula) => doula.label).toSorted((a, b) => b.length - a.length)[0] ?? '' },
				9999
			),
			clientsResultSummary(99, true, false, true, true, true, {}, 9999)
		].toSorted((a, b) => b.length - a.length)[0]
	);

	// What the rows on screen were asked for. While a newer filter loads they
	// still answer to the last one, so the count line and the empty state never
	// describe rows that are not there.
	let shown = $state.raw(appliedView());
	// The first page's total for what is shown (#2867); later pages carry none.
	let shownTotal = $state<number>();
	const shownDoulaName = $derived(doulaNameOf(shown.doula));
	const summary = $derived(
		isDoulaUnknown
			? UNKNOWN_DOULA
			: clientsResultSummary(
			clients.items.length,
			clients.hasMore,
			shown.showAll,
			shown.showMine,
			shown.showOwing,
			shown.showNoContract,
			{ doulaName: shownDoulaName, name: shown.name, order: shown.order },
			shownTotal
		)
	);

	// ADR-0017: a pending Engagement Request shows on its Client's row.
	// She can hold one of each kind at once, hence the join rather than a
	// single value.
	function pendingRequestText(client: ClientListItem): string {
		const kinds = client.pendingRequestKinds ?? [];
		if (kinds.length === 0) return '';
		const labels = kinds.map((kind) => (kind === 'birth' ? 'Birth' : 'Postpartum'));
		return `${labels.join(' & ')} request pending`;
	}

	// A line's status: its care phase and its Contract, plain words. The money
	// is not here: the Money column says it once, with the badge that carries its
	// meaning (an amber "owed", a red "12 days overdue"), and a screen reader
	// hears it once there.
	const contractFacts: Record<string, string> = {
		draft: 'Contract draft',
		sent: 'Contract sent',
		signed: 'Contract signed',
		voided: 'Contract voided'
	};

	const NO_CONTRACT = 'No Contract yet';

	// #2022: one open Engagement's status -- its care phase and its Contract.
	// No Contract is a plain fact in every care phase, never a badge: it was
	// a warning on most of a typical page, and "Only active Clients with no
	// Contract" is the filter that finds them. A pending Request is the one
	// badge in this cell (ADR-0017).
	function standingOf(line: OpenEngagement) {
		return {
			status: line.invoiceStatus ?? '',
			daysOverdue: line.daysOverdue ?? 0,
			paymentProcessing: line.paymentProcessing ?? false,
			refundedCents: line.refundedCents ?? 0
		};
	}

	function lineFacts(line: OpenEngagement): string[] {
		// #1512: the care phase in the word every Staff screen shows.
		const facts = [engagementStatusText(line.engagementStatus)];
		const contract = line.contractStatus && contractFacts[line.contractStatus];
		// Under "Only active Clients with no Contract" every line would say so:
		// the filter has said it once, in the count line.
		if (contract) facts.push(contract);
		else if (!shown.showNoContract) facts.push(NO_CONTRACT);
		return facts;
	}

	// The Status cell's words, in the order the cell draws them: the kind,
	// then the facts. They name the cell to
	// `DataTable`, and they decide whether a record card keeps the row at
	// all (#2022).
	function statusText(client: ClientListItem): string {
		const lines = (client.openEngagements ?? []).map((line) => `${kindLabel(line.kind)}: ${lineFacts(line).join(', ')}`);
		const pending = pendingRequestText(client);
		const text = [...lines, ...(pending ? [pending] : [])].join('; ');
		return text || (client.hasWork ? CARE_ENDED : NOT_YET_IN_CARE);
	}

	// A Client whose every Engagement has ended still has work (list.go's
	// hasWorkExpr), so her row says why it holds no line rather than
	// sitting blank. #458: one with none says so in the same cell, which
	// is why the switch that lists her needs no Work column.
	const CARE_ENDED = 'Care ended';
	const NOT_YET_IN_CARE = 'Not yet in care';

	// #2232 (RA-G15): who covers each open Engagement, so the list answers
	// it with no Engagement opened. A column of its own, not words in the
	// Status line, which then wrapped a typical row's money onto a second
	// line at 1440px. "No Doula yet" is the Engagement page's own words
	// (`doulaSummaryItem`), and always true here: a line is never a
	// completed Engagement.
	function doulaNamesText(line: OpenEngagement): string {
		const names = line.doulaNames ?? [];
		return names.length === 0 ? NO_DOULA_YET_LABEL : names.join(', ');
	}

	// One entry per open Engagement. Where a Client holds more than one,
	// each names its kind, so a name is never read against the wrong one.
	// Two Doulas on one Engagement run on one line, comma-joined, and wrap only
	// when the column has no room left.
	type DoulaLine = { engagementId: string; prefix: string; text: string };
	function doulaLines(client: ClientListItem): DoulaLine[] {
		const lines = client.openEngagements ?? [];
		return lines.map((line) => {
			const prefix = lines.length > 1 ? `${kindLabel(line.kind)}: ` : '';
			return { engagementId: line.engagementId, prefix, text: `${prefix}${doulaNamesText(line)}` };
		});
	}

	// The money on each open Engagement, in the Status cell's order, wherever the
	// BFF's role gate let a field through the wire (ADR-0006/ADR-0008): the
	// Invoice's amount and its state for an Owner, an Admin or an employed Doula,
	// her own fee for a contractor. A field this Reader may not see is absent,
	// so there is nothing here to hide. Where a Client holds more than one line
	// each names its kind; a line with no money draws a dash, and a Client none
	// of whose lines hold any draws nothing at all.
	type MoneyLine = {
		engagementId: string;
		prefix: string;
		lead: string;
		amount: string;
		// An Invoice's state, said once, in the Invoices list's words and badges.
		state?: ReturnType<typeof invoiceMoneyState>;
	};
	const NO_MONEY = 'No amount';

	function moneyLines(client: ClientListItem): MoneyLine[] {
		const lines = client.openEngagements ?? [];
		const held = lines.map((line): Omit<MoneyLine, 'engagementId' | 'prefix'> => {
			if (line.invoiceStatus) {
				const amount = line.invoiceAmountCents === undefined ? '' : formatAmount(line.invoiceAmountCents);
				return { lead: '', amount, state: invoiceMoneyState(standingOf(line)) };
			}
			if (line.feeCents !== undefined) return { lead: 'Your fee', amount: formatAmount(line.feeCents) };
			return { lead: '', amount: '' };
		});
		if (held.every((money) => money.amount === '' && !money.state?.word)) return [];
		return lines.map((line, index) => {
			const prefix = lines.length > 1 ? `${kindLabel(line.kind)}: ` : '';
			return { engagementId: line.engagementId, prefix, ...held[index]! };
		});
	}

	// When each open Engagement's birth is due, in the Status cell's order, so
	// the list reads as the order it is in. Where a Client holds more than
	// one, each names its kind.
	const NO_DUE_DATE = 'No due date';

	// An Engagement with no date draws a dash on screen and says "No due date"
	// to a screen reader only: beside a red Overdue badge, those words on screen
	// read as being about the invoice.
	type DueLine = {
		engagementId: string;
		prefix: string;
		lead: string;
		day: string;
		// A live birth's weeks since, "3 weeks postpartum"; the birth day stays in `day`.
		postpartum: string;
		// The calendar day the birth happened, `YYYY-MM-DD`, for a `<time datetime>`.
		bornOn: string;
		date: string;
		text: string;
	};

	// Every value says which date it is, so the column is headed "Date" and a
	// screen reader hears one phrase per line, never a label that may be wrong:
	// "Due Nov 9, 2026" while the birth is ahead; once the pregnancy has ended
	// the due date forecasts nothing, so "3 weeks postpartum" for a live birth
	// (the birth day is kept for a screen reader) and the neutral
	// "Ended Oct 30, 2026" for a loss or an outcome nobody learned, which never
	// says "postpartum" (docs/personas/loss-client.md). A line with no day
	// recorded reads nothing.
	// The word and the day apart, so the word can be muted and the days line
	// up in a column of their own.
	function dueParts(line: OpenEngagement): { lead: string; day: string; postpartum: string; bornOn: string } {
		if (line.pregnancyEndedOn) {
			const day = formatCalendarDay(line.pregnancyEndedOn);
			if (line.birthOutcome === 'live_birth') {
				return { lead: '', day, postpartum: formatPostpartum(line.pregnancyEndedOn, practiceTimeZone()), bornOn: line.pregnancyEndedOn };
			}
			return { lead: 'Ended', day, postpartum: '', bornOn: '' };
		}
		return line.dueDate
			? { lead: 'Due', day: formatCalendarDay(line.dueDate), postpartum: '', bornOn: '' }
			: { lead: '', day: '', postpartum: '', bornOn: '' };
	}

	function dueLines(client: ClientListItem): DueLine[] {
		const lines = client.openEngagements ?? [];
		return lines.map((line) => {
			const prefix = lines.length > 1 ? `${kindLabel(line.kind)}: ` : '';
			const { lead, day, postpartum, bornOn } = dueParts(line);
			const date = postpartum ? `${postpartum}, born ${day}` : (day && lead ? `${lead} ${day}` : day);
			return { engagementId: line.engagementId, prefix, lead, day, postpartum, bornOn, date, text: `${prefix}${date || NO_DUE_DATE}` };
		});
	}

	// #2022: Email and Portal invite are on the Client hub, where the
	// Blocked email addresses link (#785) went with them, and a pending
	// Request is a badge in Status. What is left fits a desktop.
	// Name, Money, Date and Doula are each as wide as their longest value and
	// never wrap; Status, the column with the most to say, holds its track and
	// wraps only where the frame runs out (`columnWidths="one-line"`). A Client
	// with two Engagements stacks hers by design.
	const columnSet = {
		name: { label: 'Name', accessor: (client: ClientListItem) => client.name },
		status: { label: 'Status', accessor: statusText, content: statusCell, grows: true },
		money: {
			label: 'Money',
			numeric: true,
			// Empty on purpose: the record card draws each Engagement's money in its
			// Status line, beside the Engagement it belongs to (the field-per-column
			// record would wrap its line to a second row on a phone).
			accessor: () => '',
			content: moneyCell
		},
		date: {
			// Left-aligned like the words around it: dates are read, not summed,
			// and right-aligned they ran into the Doula's name beside them.
			label: 'Date',
			accessor: (client: ClientListItem) =>
				dueLines(client)
					.map((line) => line.text)
					.join('; '),
			content: dueCell
		},
		doula: {
			label: 'Doula',
			accessor: (client: ClientListItem) =>
				doulaLines(client)
					.map((line) => line.text)
					.join('; '),
			content: doulaCell
		}
	};

	// A column that would say the same thing on every row, or nothing on any,
	// is left out (the table and the record card draw the same set). Money is
	// there when a Client on the page has some, which no Client under "Only
	// active Clients with no Contract" does. Under one Doula, the count line
	// names her, so her column would repeat her name down the page.
	const hasMoney = $derived(clients.items.some((client) => moneyLines(client).length > 0));
	const hasDoulaColumn = $derived(shown.doula === '');
	// A column left out gives its room to Status, the column that grows, so
	// the columns after it start where they started. The Doula is the last
	// column, so nothing after it moves.
	const columns = $derived([
		columnSet.name,
		columnSet.status,
		...(hasMoney ? [columnSet.money] : []),
		columnSet.date,
		...(hasDoulaColumn ? [columnSet.doula] : [])
	]);

	// The column the rows on screen are in order by (`aria-sort`, #2869): the
	// Name column for the name orders, Money for owed, and Date, which says the
	// due or ended day, for the list's own.
	const orderedBy = $derived(
		{
			'': { column: columnSet.date.label, direction: 'ascending' },
			name: { column: columnSet.name.label, direction: 'ascending' },
			'name-desc': { column: columnSet.name.label, direction: 'descending' },
			owed: { column: columnSet.money.label, direction: 'descending' }
		}[shown.order] as { column: string; direction: 'ascending' | 'descending' }
	);

	// The Client detail hub (#494) -- DataTable links only its first
	// column, per rowHref's own contract.
	function clientHref(client: ClientListItem): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId: client.clientId
		});
	}

	// Reloads page one against the current filter -- on first mount, on a
	// toggle click, and on a browser back/forward that lands on a
	// different `all` value, since `isShowingEveryone` is derived from
	// `page.url` and every one of those re-runs this effect.
	$effect(() => {
		void loadFirstPage(appliedView());
	});

	/*
	 * "Load more" is the only fetch that *merges* into what is on screen,
	 * so it is the only one a stale response can corrupt: a page-two
	 * request already in flight when the reader flips the toggle would
	 * append the old filter's Clients onto the new filter's list, and
	 * nothing afterwards would take them back out. `clients.reset` below
	 * abandons any such response -- the guard now lives in PaginatedList,
	 * where every paging list gets it rather than only this one.
	 *
	 * loadFirstPage needs no guard of its own. It *replaces* the list
	 * wholesale rather than appending, so two of them racing can only
	 * leave a whole, self-consistent result set from a filter one flip out
	 * of date -- which the effect's next run corrects -- never a list
	 * mixing both.
	 */
	let latestLoad = 0;
	async function loadFirstPage(view: ReturnType<typeof appliedView>) {
		const thisLoad = ++latestLoad;
		// The rows already on screen stay up, marked busy, until the new
		// page lands; a skeleton stands in only for the very first load.
		isRefreshing = untrack(() => isLoaded);
		error = '';
		try {
			// Back to this list from a Client: as many pages as were open, under
			// the same filters, so the row she came from is there to land on.
			const wanted = pagesToRestore(untrack(() => page.state.clientsPages));
			let first = await loadClients(apiFetchWithSession, page.params.practiceId!, view);
			const total = first.clientCount;
			let pages = 1;
			while (pages < wanted && first.hasMore && first.nextCursor) {
				const next = await loadClients(apiFetchWithSession, page.params.practiceId!, { ...view, cursor: first.nextCursor });
				if (thisLoad !== latestLoad) return;
				first = { items: [...first.items, ...next.items], nextCursor: next.nextCursor, hasMore: next.hasMore };
				pages += 1;
			}
			if (thisLoad !== latestLoad) return;
			clients.reset(first);
			pagesLoaded = pages;
			loadedKey = JSON.stringify(view);
			shown = view;
			shownTotal = total;
			isDoulaUnknown = false;
			isLoaded = true;
			isRefreshing = false;
			if (heldReturn) void restorePlace();
		} catch (error_) {
			if (thisLoad !== latestLoad) return;
			heldReturn = undefined;
			// A Doula id the BFF refuses as malformed can never load, so Try again
			// would only fail again: say what is wrong and offer the way out.
			if (view.doula && error_ instanceof RefusalError && error_.status === 400) {
				clients.reset({ items: [], hasMore: false });
				pagesLoaded = 1;
				loadedKey = JSON.stringify(view);
				shown = view;
				shownTotal = undefined;
				isDoulaUnknown = true;
				isLoaded = true;
				isRefreshing = false;
				return;
			}
			error = LOAD_FAILED;
			isLoaded = false;
			isRefreshing = false;
		}
	}

	// How many pages the list holds, kept in this history entry's state so a
	// Back to the list can load as many again (`listReturn.ts`).
	let pagesLoaded = 1;
	async function loadMore() {
		const before = clients.items.length;
		await clients.loadMore();
		if (clients.items.length === before) return;
		pagesLoaded += 1;
		if (!isLeaving) replaceState('', { clientsName: page.state.clientsName, clientsPages: pagesLoaded });
	}

	function searchHref(): string {
		return resolve('/practices/[practiceId]/clients/search', {
			practiceId: page.params.practiceId!
		});
	}

	// #539 (ADR-0017): "No Clients yet." is silent about *why* to a
	// contractor Doula, who never gets to make one -- the same screen
	// tells her plainly that work reaches her as an Offer instead.
	//
	// #1706: the default filter is Clients in care (ADR-0017), so an
	// empty default view at a Practice that holds a Client record (the
	// `all=true` count +page.ts read) means nobody is in care yet, not that
	// nobody exists -- and "No Clients yet." would contradict the "Find or
	// add a Client" link above it. With the switch on there is no
	// filter left to point at, so the plain message stays.
	// Everyone else has the bar's own "Clear filters", in view at every width, so
	// the empty state does not offer it a second time.
	const emptyAction = $derived(
		isContractor ? { label: 'How to add Clients of your own', href: searchHref() } : undefined
	);

	const emptyMessage = $derived.by(() => {
		if (isContractor) return 'Work reaches you as an Offer, so there are no Clients here yet.';
		// Narrowed: what was looked for, in the count line's own words.
		if (shown.showMine || shown.showOwing || shown.showNoContract || shown.doula || shown.name) {
			return summary;
		}
		if (hasAnyClient && !shown.showAll) {
			return `No Client is in care yet. Turn on "Also show ${NO_CARE_YET}" to list them.`;
		}
		return 'No Clients yet.';
	});
</script>

{#snippet statusCell(client: ClientListItem, view: DataTableView)}
	<!--
		#264, #2022: one <li> per open Engagement -- ADR-0017's "a Client can
		hold more than one concurrent open Engagement" is why this is a list,
		so a second (or third) line is never dropped -- then the pending
		Request, which a Client can hold before she has any Engagement.
		A Client with neither reads "Care ended" or "Not yet in care".
	-->
	{@const pending = pendingRequestText(client)}
	{@const money = moneyLines(client)}
	{#if (client.openEngagements ?? []).length > 0 || pending}
		<!--
			The name is the row's one link; each Engagement is a link on the
			Client hub it opens (#1511), so the kind here is a word.
		-->
		<ul class="status-list">
			{#each client.openEngagements ?? [] as line, lineIndex (line.engagementId)}
				{@const facts = lineFacts(line)}
				{@const owed = money[lineIndex]}
				<li>
					<cluster-l space="var(--space-1)" align="center">
						<span class="kind">{kindLabel(line.kind)}</span>
						<!-- A screen reader hears a comma after the kind, as it does
						     between the facts; nothing is drawn. -->
						<span class="visually-hidden">, </span>
						<span class="facts">
							<!-- The dot is drawn and the comma is read, so a screen
							     reader hears "Intake, Contract signed", never the
							     facts run together. The dot rides with the fact before
							     it and the line breaks after it, so a dot ends a line
							     and never starts one. -->
							{#each facts as fact, index (fact)}
								{#if index < facts.length - 1}<span class="no-break"><span>{fact}</span><span class="visually-hidden">, </span><span class="separator" aria-hidden="true">·</span></span><wbr />{:else}<span>{fact}</span>{/if}
							{/each}
						</span>
					</cluster-l>
					{#if view === 'record' && owed && (owed.amount || owed.state?.word)}{@render recordMoney(owed)}{/if}
				</li>
			{/each}
			{#if pending}
				<!-- ADR-0017: a pending Request waits on the Practice. Blue means
				     pending app-wide; amber is kept for what needs someone to act. -->
				<li><Badge label={pending} variant="info" /></li>
			{/if}
		</ul>
	{:else}
		<span class="facts">{client.hasWork ? CARE_ENDED : NOT_YET_IN_CARE}</span>
	{/if}
{/snippet}

<!-- A card's own Money line, under its Status: an Engagement's amount and state, drawn only where it
     has some. The table draws them in its own column. -->
{#snippet recordMoney(owed: MoneyLine)}
	<div class="record-money">
		<span class="visually-hidden">Money: </span>{#if owed.state}<MoneyAmount amount={owed.amount} state={owed.state} />{:else}<span class="no-break">{owed.lead ? `${owed.lead} ` : ''}{owed.amount}</span>{/if}
	</div>
{/snippet}

{#snippet dueCell(client: ClientListItem, view: DataTableView)}
	<!-- One line per open Engagement, in the Status cell's order. Nothing for a Client with none. -->
	{@const lines = dueLines(client)}
	{@const doulas = hasDoulaColumn && view === 'table' ? doulaLines(client) : []}
	{#if lines.length === 1}
		{@render dueValue(lines[0])}{@render foldedDoula(doulas[0])}
	{:else if lines.length > 1}
		<ul class="status-list">
			{#each lines as line, index (line.engagementId)}
				<li>{@render dueValue(line)}{@render foldedDoula(doulas[index])}</li>
			{/each}
		</ul>
	{/if}
{/snippet}

<!-- Between the floor and the width the Doula column fits, the column is not drawn and her name sits under the Date, so the table keeps every column's words. Not drawn at all from the wide band, and the table's own Doula cell is not drawn below it, so a screen reader hears her once. -->
{#snippet foldedDoula(line: DoulaLine | undefined)}
	{#if line}
		<span class="folded-doula"><span class="visually-hidden">Doula: </span><span class="doula-names">{line.text}</span></span>
	{/if}
{/snippet}

{#snippet dueValue(line: DueLine)}
	{line.prefix}{#if line.postpartum}<time class="date postpartum" datetime={line.bornOn}>{line.postpartum}<span class="visually-hidden">, born {line.day}</span></time>{:else if line.day}<span class="date"><span class="lead">{line.lead}</span> <span>{line.day}</span></span>{:else}<span aria-hidden="true">—</span><span class="visually-hidden">{NO_DUE_DATE}</span>{/if}
{/snippet}

{#snippet doulaCell(client: ClientListItem)}
	<!--
		#2232: one line per open Engagement, in the Status cell's order, so
		a Client with two reads as two. Nothing for a Client with none, and
		the accessor is empty for her too, so her record card leaves the
		row out.
	-->
	{@const lines = doulaLines(client)}
	{#if lines.length === 1}
		{@render doulaNames(lines[0])}
	{:else if lines.length > 1}
		<ul class="status-list">
			{#each lines as line (line.engagementId)}
				<li>{@render doulaNames(line)}</li>
			{/each}
		</ul>
	{/if}
{/snippet}

{#snippet doulaNames(line: DoulaLine)}
	<span class="doula-names">{line.text}</span>
{/snippet}

{#snippet moneyCell(client: ClientListItem)}
	<!-- One line per open Engagement, in the Status cell's order. Nothing for a Client whose lines hold no money. The record card has no Money field: its Status line carries each amount. -->
	{@const lines = moneyLines(client)}
	{#if lines.length === 1}
		{@render moneyValue(lines[0])}
	{:else if lines.length > 1}
		<ul class="status-list">
			{#each lines as line (line.engagementId)}
				<li>{@render moneyValue(line)}</li>
			{/each}
		</ul>
	{/if}
{/snippet}

<!-- The amount and its state (`MoneyAmount`, the Invoices list's too); a fee is the words "Your fee" and the amount. -->
{#snippet moneyValue(line: MoneyLine)}
	{line.prefix}{#if !line.amount}<span aria-hidden="true">—</span><span class="visually-hidden">{NO_MONEY}</span>{#if line.state}<MoneyAmount amount="" state={line.state} />{/if}{:else if line.state}<MoneyAmount amount={line.amount} state={line.state} isTracked />{:else}<span class="no-break"><span class="lead">{line.lead}</span>&nbsp;{line.amount}</span>{/if}
{/snippet}

{#snippet actions()}
	<!--
		Once the Practice holds a Client record, this link lands on the
		search (#498, #539, ADR-0017), not on intake directly, so a
		returning Client is found rather than typed a second time. A miss
		there hands the reader on to intake. Named for both errands it
		serves -- finding a returning Client is the one a contractor
		Doula's own attached-Clients list already covers, which is why she
		does not see this at all (#539).

		While the Practice holds no Client record there is nobody to find,
		so it opens the name question and says only what it does (#1609,
		ADR-0017's amendment of 2026-10-02). +page.ts decides which, from
		the count of records, not from the filtered rows below.
	-->
	{#if hasAnyClient === undefined}
		<!-- The link's own box, drawn invisible and out of reach, so the page does not
		     move when the read lands and the real link takes its place. -->
		<span class="reserve" aria-hidden="true" inert>
			<Link href={searchHref()} label="Find or add a Client" variant="start" />
		</span>
	{:else if hasAnyClient}
		<Link href={searchHref()} label="Find or add a Client" variant="start" />
	{:else}
		<Link href={startHref(page.params.practiceId!, 'clients')} label="Add a Client" variant="start" />
	{/if}
{/snippet}

{#snippet alsoShow(isReserve: boolean)}
	<LabeledField label={`Also show ${NO_CARE_YET}`} orientation="inline">
		{#snippet children(control)}
			<Checkbox name="all" value="true" checked={isAllChecked} disabled={isReserve} onChange={handleToggleAll} {...control} />
		{/snippet}
	</LabeledField>
{/snippet}

<!-- Drawn only while a filter is applied, so the default list loses no room. The × comes first, as on the MOJ filter's tags; the name says what pressing it does. -->
{#snippet applied()}
	{#each appliedTags as tag (tag.text)}
		<li>
			<Link
				href={tag.href}
				label={tag.text}
				variant="chip"
				icon="x"
				ariaLabel={`Remove filter: ${tag.text.replace(': ', ' ')}`}
				onClick={(event) => removeTag(event, tag)}
			/>
		</li>
	{/each}
{/snippet}

{#snippet content()}
	<!--
		The default view is Clients in care (ADR-0017); this switches to
		everyone, including a Client whose only Request was refused -- nothing
		new is built for her, she is simply no longer filtered out.

		#2068: the filters sit in the list's filter bar, directly above the
		list they narrow (Law of Proximity). The bar now submits, with one
		Apply: it holds a text field and a select as well as the checkboxes,
		and GOV.UK's filter pattern (ADR-0021) applies them together rather
		than moving the list under a person typing. The form is a real GET, so
		its field names are the address the page reads, with no script running.
		What is hidden follows the applied address, not the unapplied controls.
	-->
	<!-- The bar and the list it narrows are one group, held closer than
	     the page's sections; the bar's own rows sit closer still, so the table
	     reads as apart from the controls and the count as part of the group. -->
	<stack-l space="var(--space-related)">
		<!-- #2603: a contractor Doula is attached only through an Offer she accepted (ADR-0008), so the switch could never add a row to her list. -->
		{#if !isContractor}
			<FilterBar
				bind:this={filterBar}
				action={clientsPath({})}
				onSubmit={applyFilters}
				legend="Choose which Clients to show"
				status={isReady ? summary : ''}
				statusRuler={longestSummary}
				empty={isReady && clients.items.length === 0}
				clearHref={isFiltered ? clientsPath({}) : undefined}
				applied={appliedTags.length > 0 ? applied : undefined}
				appliedCount={appliedTags.length}
				hasUntaggedFilter={appliedOrder !== ''}
				foldsFieldsOnly
				foldLabel={!isImplyingCare || doulas.length > 0 ?'More filters and order' : 'Order'}
				order={{
					options: CLIENT_ORDER_OPTIONS,
					value: draftOrder,
					onChange: (value) => {
						touched.push('order');
						draftOrder = value as ClientOrder;
					}
				}}
				{isBusy}
				statusId={RESULT_ID}
			>
				<!--
					Two rows that do not depend on each other: the switches, then
					the fields with Apply and "Clear filters" after them. A switch
					drawn or hidden never moves a field, and Apply never moves.
				-->
				{#snippet switches()}
					<!-- One group, so "Clients" is said once, by the page, and each box reads as the end of "Show only". The legend is drawn on the boxes' own row. -->
					<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- named boxes of a GET form beside the bar's own fields: CheckboxGroup stacks one option to a line and carries its value as an array -->
					<fieldset>
						<legend>Show only</legend>
						<div class="show-only-boxes">
							<LabeledField label="Who owe money" orientation="inline">
								{#snippet children(control)}
									<Checkbox name="owes" value="true" checked={isOwingChecked} onChange={handleToggleOwing} {...control} />
								{/snippet}
							</LabeledField>
							<!-- The BFF leaves out a Client whose only Engagement is still in Intake, where no Contract is expected. -->
							<LabeledField label="Active, with no Contract" orientation="inline">
								{#snippet children(control)}
									<Checkbox name="noContract" value="true" checked={isNoContractChecked} onChange={handleToggleNoContract} {...control} />
								{/snippet}
							</LabeledField>
							<!-- Beside the Doula select it is the other half of: whose Clients. The BFF ANDs the two, so mine and a Doula is a narrower list. -->
							{#if canNarrowToMine && (hasMine || isShowingMine)}
								<LabeledField label="Mine" orientation="inline">
									{#snippet children(control)}
										<Checkbox name="mine" value="true" checked={isMineChecked} onChange={handleToggleMine} {...control} />
									{/snippet}
								</LabeledField>
							{/if}
						</div>
					</fieldset>
					<!-- Last, so drawing it or not moves nothing. The default list is ADR-0017's: current care, past care, or a pending Request. The BFF ANDs this with a narrowing that implies care (mine, owing, no Contract, a Doula), so ticking it beside one changes nothing and it is not drawn. -->
					{#if !isImplyingCare}
						<div class="also-wide">
							{@render alsoShow(false)}
						</div>
					{:else}
						<!-- Unseen and out of reach; FilterBar keeps its box where the bar is wide, so the switches, the fields and Apply keep their place. -->
						<div data-holds-space aria-hidden="true" inert>
							{@render alsoShow(true)}
						</div>
					{/if}
				{/snippet}
				<!-- Name and Doula share a line in a narrow column, each taking half
				     of it, and sit at their own widths where there is room. -->
				<div class="field name" bind:this={nameField}>
					<LabeledField label="Name" width={20}>
						{#snippet children({ id, describedBy, invalid, width })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								{width}
								type="search"
								maxlength={100}
								autocomplete="off"
								value={draftName}
								onInput={(value) => {
									touched.push('name');
									draftName = value;
								}}
							/>
						{/snippet}
					</LabeledField>
				</div>
				<!-- Folds with the Order select below 42rem. Where the bar is wide the fold is not drawn, and the box above sits on the switches row as it always has; this copy is the narrow bar's. Both are the one form field, `all`. -->
				{#snippet folded()}
					{#if doulas.length > 0}
						<!-- Only an Owner or an Admin reads the roster this lists (ADR-0008). Drawn while "Only my Clients" is on too, so Apply never moves: the BFF ANDs the filters, so mine and a Doula is a narrower list. -->
						<div class="field doula">
							<LabeledField label="Doula" width={30}>
								{#snippet children({ id, describedBy, invalid, width })}
									<Select {id} {describedBy} {invalid} {width} name="doula" options={doulaOptions} bind:value={draftDoula} onChange={() => {
										touched.push('doula');
									}} />
								{/snippet}
							</LabeledField>
						</div>
					{/if}
					{#if !isImplyingCare}
						<div class="also-narrow">
							{@render alsoShow(false)}
						</div>
					{/if}
				{/snippet}
			</FilterBar>
		{/if}

		{#if error}
			<stack-l space="var(--space-related)">
				<Notice message={error} variant="error" />
				<Button label="Try again" onClick={() => loadFirstPage(appliedView())} />
			</stack-l>
		{:else if isReady}
			<!--
				The floor is this route's own (`.clients-table`, below).

				#539, #501 (ADR-0017): hiding "Find or add a Client" above took
				away this door's only link for a contractor Doula, so it comes
				back as the empty list's action (#2015) -- once she has an
				attached Client, the narrowed list is already her route to it.
				Everyone else has "Find or add a Client" above the list.
			-->
			<!-- Five columns, four of them as wide as their values and one that takes the rest. The
			     table is a table from the width the busiest typical row needs, and records below it. -->
			<div class="clients-table" class:has-doula={hasDoulaColumn}>
			<DataTable
				{columns}
				rows={clients.items}
				rowHref={clientHref}
				rowId={(client, view) => rowDomId(client.clientId, view)}
				hasMore={clients.hasMore}
				onLoadMore={loadMore}
				isLoadingMore={clients.isLoadingMore}
				loadMoreError={clients.loadMoreError}
				{emptyMessage}
				isEmptyMessageSpoken={isContractor || emptyMessage !== summary}
				caption="Clients"
				columnWidths="one-line"
				{orderedBy}
				recordLayout="compact"
				describedBy={isContractor ? undefined : RESULT_ID}
				{isBusy}
				{emptyAction}
			/>
			</div>
		{:else}
			<Skeleton variant="row" lines={8} label="Loading Clients" />
		{/if}
	</stack-l>
{/snippet}

<ListPage title="Clients" actions={isContractor ? undefined : actions} {content} />

<style>
	@layer components {
		/*
		 * The Status cell's own list (#264, #2022). The markup is written in
		 * this file, so this file's scope reaches it and DataTable is left
		 * owning the CELL rather than what a caller puts in it (#740). The
		 * cell's vertical room is DataTable's own (`td.content`).
		 */
		.status-list {
			display: grid;
			gap: var(--space-1);
			margin: 0;
		}

		/* Three widths, each measured on the typical screens
		   `route-continuum.svelte.spec.ts` sweeps, the busiest being the demo
		   Practice's (two Doulas, an "Ended" date, an overdue badge and a
		   Postpartum line), on the drag surface in macOS/Chromium. A container
		   query serves every Client list, so each width is the busiest's.

		   Below the first (55rem): records. From it: the table, with the Doula
		   folded under the Date and the Status free to wrap to two lines, and every
		   row two lines tall so the rows are equal. From the second (75.2rem, the
		   discovered `min-width` query, the width every column needs on one line):
		   the Doula has her own column and every row is one line.
		   `DataTable.svelte`'s floor comment says why a floor can be the route's. */
		.clients-table {
			container: clients-table / inline-size;
			/* The room a cell keeps either side, so two columns are two insets apart. */
			--data-table-cell-inset: var(--space-2);
			/* The Name column's least width, a long two-word name, so what follows it
			   starts at one place whichever Clients the page holds; a longer name
			   widens it. */
			--name-track: 15ch;
			/* The Status column's width (`grows`): its busiest line on one line, and no
			   wider, so a wide window leaves no gap between the status words and the Money. */
			--data-table-grows-track: 32ch;
		}

		.clients-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: grid;
		}

		.clients-table :global(.table-view :is(th, td):first-child) {
			min-inline-size: var(--name-track);
		}

		/* Records fill the container, in as many columns as it holds. */
		.clients-table :global(.record-view) {
			grid-template-columns: repeat(auto-fill, minmax(min(100%, 38ch), 1fr));
			gap: var(--space-related);
			align-items: start;
		}

		.clients-table :global(.record-view .record) {
			max-inline-size: none;
			margin-block-start: 0;
		}

		.folded-doula {
			display: block;
			color: var(--color-on-surface-variant);
		}

		@container clients-table (width >= 55rem) {
			.clients-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}

			.clients-table :global(.table-view td.grows) {
				overflow-wrap: normal;
			}

			/* The Doula's own column is not drawn; her name is under the Date. */
			.has-doula :global(.table-view :is(th, td):last-child) {
				display: none;
			}

			.clients-table :global(.table-view tbody :is(td, th)) {
				block-size: calc(var(--row-height) + 1lh + var(--space-1));
			}
		}

		@container clients-table (min-width: 75.2rem) {
			.has-doula :global(.table-view :is(th, td):last-child) {
				display: table-cell;
			}

			.clients-table :global(.table-view tbody :is(td, th)) {
				block-size: var(--row-height);
			}

			.folded-doula {
				display: none;
			}
		}

		/* The header ends where the amounts do (`MoneyAmount`'s two tracks). */
		.clients-table :global(th.numeric) {
			padding-inline-end: calc(var(--data-table-cell-inset) + var(--money-state-track) + var(--space-1));
		}

		/* A card's fields come in one order: the name, the Status, the Money where
		   the Client has some, then the Date and the Doula a line each, since two
		   fields on one line wrap to a second at some widths and not at others. A
		   field a Client has nothing in is not drawn, so cards differ in height. */
		.clients-table :global(.record-view .compact-meta) {
			display: flex;
			flex-direction: column;
		}

		.clients-table :global(.record-view .compact-separator) {
			display: none;
		}

		.record-money {
			display: flex;
			align-items: center;
			gap: var(--space-1);
		}

		.reserve {
			display: contents;
			visibility: hidden;
		}

		/* The boxes wrap among themselves beside FilterBar's floated legend. */
		.show-only-boxes {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-related);
		}

		/* A live birth's "3 weeks postpartum": one phrase, not the two tracks of a day. */
		.date.postpartum {
			display: inline;
		}

		.kind {
			font-weight: var(--font-weight-semibold);
		}

		.facts,
		.lead {
			color: var(--color-on-surface-variant);
		}

		/* The dot between two facts, spaced by a token: whitespace inside
		   the span would be trimmed. */
		.separator {
			padding-inline: var(--space-1);
		}

		/* The lead word and the day in two tracks, so every day in the column
		   starts at one place whichever word leads it. The lead track holds the
		   longest ("Ended") with room to spare. The words stay one phrase to a
		   screen reader: the space between them is in the text. */
		.date {
			display: inline-grid;
			grid-template-columns: 5.4ch auto;
			column-gap: var(--space-1);
			white-space: nowrap;
		}

		/* A date, or an amount with its word, is never broken across two lines. */
		.no-break {
			white-space: nowrap;
		}

		/* Name and Doula: in a narrow column each takes half of the line, and
		   where there is room each stands at its own width from the start, so the
		   applied filters' tags after Apply join the line only where room is left
		   beside the fields at their full width. Apply wraps under them only when
		   the line is full. */
		.field {
			--field-width: 22ch;
			flex: 1 1 min(var(--field-width), calc(50% - var(--space-related) / 2));
			max-inline-size: var(--field-width);
		}

		.doula {
			--field-width: 34ch;
		}

		/* The "Also show" box is drawn twice, once for each bar: on the switches row where the bar is wide, in the fold where it is not. */
		.also-wide {
			display: contents;
		}

		.also-narrow {
			display: none;
		}

		/* In the fold the fields are a column, so a field's flex basis would be a height. */
		@container filter-bar (width < 42rem) {
			.also-wide {
				display: none;
			}

			.also-narrow {
				display: block;
			}

			.doula {
				flex: none;
			}
		}
	}
</style>
