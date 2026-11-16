<script lang="ts">
	import { untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import {
		CONTRACT_VIEW_TAGS,
		careWhen,
		CONTRACTS_RESULT_RULER,
		NO_DOULA_FALLBACK,
		contractColumnOrders,
		contractOrderOf,
		contractOrderOptions,
		contractPeriodOf,
		contractOrderedBy,
		contractStatusWord,
		contractsQueryString,
		contractsResultSummary,
		contractsViewOf,
		contractStepper,
		daysWaitingWords,
		nameWithInitial,
		EVERY_CONTRACT_ORDER_OPTION,
		downloadSignedContractPdf,
		remindContract,
		signedContractFilename,
		signedPdfDownloadedWords,
		loadPracticeContracts,
		loadPracticeVoidRequests,
		orderForView,
		voidRequestsHeading,
		type AwaitingVoidRequest,
		type ContractListItem,
		type ContractOrder,
		type ContractsListData,
		type ContractsPage,
		type ContractsView,
		type VoidRequestsPage
	} from '#lib/contract.js';
	import { contractsDownloadedWords, requestContractsExport } from '#lib/contractExport.js';
	import { CsvDownload } from '#lib/csvDownload.svelte.js';
	import { formatCalendarDayThisYear } from '#lib/dates.js';
	import { kindLabel, NO_DOULA_YET_LABEL } from '#lib/engagementRequest.js';
	import { calendarDayIn, formatAmount } from '#lib/invoice.js';
	import { focusResultAfterChange } from '#lib/listFocus.svelte.js';
	import { keepListReturn, tallyListReturns } from '#lib/listReturn.js';
	import { holdListSearch } from '#lib/listOrigin.svelte.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { canNarrowToMine, isAmbientContractor, isOwnerOrAdmin, type LabeledValue } from '#lib/roles.js';
	import { rowLinkLabeler } from '#lib/rowLinkLabel.js';
	import { sentPeriodDays, sentPeriodKey, sentPeriodWords, type SentDays, type SentPeriod } from '#lib/sentPeriod.js';
	import { SentPeriodDraft } from '#lib/sentPeriodDraft.svelte.js';
	import { typedIn } from '#lib/typedIn.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import type { EmptyStateAction } from '#lib/components/molecules/EmptyState.svelte';
	import DoulaNames from '#lib/components/molecules/DoulaNames.svelte';
	import DownloadCsvButton from '#lib/components/molecules/DownloadCsvButton.svelte';
	import DownloadCsvNotices from '#lib/components/molecules/DownloadCsvNotices.svelte';
	import SentPeriodFields from '#lib/components/molecules/SentPeriodFields.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import ReadFailure from '#lib/components/molecules/ReadFailure.svelte';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PracticeSession } from '../+layout.js';
	import ShowOnly from '#lib/components/molecules/ShowOnly.svelte';
	import VoidRequestList from './VoidRequestList.svelte';

	let {
		data,
		isReading = false,
		hasReadFailed = false,
		onRetry = async () => {},
		isQueueExpected = false,
		queue,
		hasQueueFailed = false,
		doulas = [],
		isRosterLanded = true
	}: {
		data?: ContractsListData;
		isReading?: boolean;
		hasReadFailed?: boolean;
		onRetry?: () => Promise<void>;
		isQueueExpected?: boolean;
		queue?: VoidRequestsPage;
		hasQueueFailed?: boolean;
		doulas?: LabeledValue[];
		isRosterLanded?: boolean;
	} = $props();

	const LOAD_FAILED = 'Your Contracts did not load.';
	const SEARCH_FAILED = 'Your search did not run.';
	const LOAD_MORE_FAILED = 'More Contracts did not load. Try again.';
	const QUEUE_FAILED = 'The void requests did not load.';
	const QUEUE_MORE_FAILED = 'More void requests did not load. Try again.';
	const PDF_FAILED = 'The Signed PDF did not download.';
	const RESULT_ID = 'contracts-result';
	const ANY_DOULA = 'Any Doula';

	const basePath = $derived(resolve('/practices/[practiceId]/contracts', { practiceId: page.params.practiceId! }));
	const session = $derived((page.data as { session: PracticeSession }).session);
	const timeZone = $derived(session.timezone);
	const isAdmin = $derived(isOwnerOrAdmin(session));
	const isContractor = $derived(isAmbientContractor(session));
	const canMine = $derived(canNarrowToMine(session));
	const canPickDoula = $derived(isAdmin && (doulas.length >= 2 || !isRosterLanded));

	const view = $derived(contractsViewOf(page.url.searchParams));
	const order = $derived(contractOrderOf(page.url.searchParams, view));
	const isMine = $derived(canMine && page.url.searchParams.get('mine') === 'true');
	const doula = $derived(page.url.searchParams.get('doula') ?? '');
	const period = $derived(contractPeriodOf(page.url.searchParams, view));
	const appliedSearch = $derived((page.state.contractsSearch ?? '').trim());
	$effect(() => {
		holdListSearch('contracts', appliedSearch);
	});

	const mountedAt = new Date();
	const now = $derived(data?.now ?? mountedAt);

	type ReadKey = { view: ContractsView; order: ContractOrder; isMine: boolean; doula: string; search: string; period?: SentPeriod };
	const isSameKey = (a: ReadKey | undefined, b: ReadKey) =>
		a?.view === b.view &&
		a.order === b.order &&
		a.isMine === b.isMine &&
		a.doula === b.doula &&
		a.search === b.search &&
		sentPeriodKey(a.period) === sentPeriodKey(b.period);
	const wanted = $derived<ReadKey>({ view, order, isMine, doula, search: appliedSearch, period });
	const wantedDays = $derived(sentPeriodDays(period, timeZone, now));

	let shown = $state.raw<ReadKey>();
	let book = $state.raw<ContractsPage>();
	let shownDays = $state.raw<SentDays>();
	let failedKey = $state.raw<ReadKey>();
	let isApplying = $state(false);

	const contracts = new PaginatedList<ContractListItem>({
		first: { items: [], hasMore: false },
		loadPage: async (cursor) => {
			const under = shown!;
			try {
				const next = await loadPracticeContracts(
					apiFetchWithSession,
					page.params.practiceId!,
					{ view: under.view, order: under.order, isMine: under.isMine, doula: under.doula, search: under.search, days: shownDays },
					cursor
				);
				if (shown === under) book = next;
				return next;
			} catch {
				throw new Error(LOAD_MORE_FAILED);
			}
		},
		failureMessage: LOAD_MORE_FAILED
	});

	function adopt(first: ContractsPage, key: ReadKey, days: SentDays | undefined) {
		shown = key;
		shownDays = days;
		contracts.reset(first);
		book = first;
		returns.pagesLoaded = 1;
		if (isQueueSettled && returns.held) void returns.restore(returns.held);
	}

	const returns = tallyListReturns(contracts, rowLink);
	keepListReturn({
		id: 'contracts-list',
		pagesLoaded: () => returns.pagesLoaded,
		rowOf: (to) => {
			if (to.hash.startsWith('#contract-')) return to.hash.slice('#contract-'.length);
			if (to.hash.startsWith('#void-request-')) return `vr:${to.hash.slice('#void-request-'.length)}`;
			return;
		},
		restore: (back) => {
			if (readFailure) return;
			returns.held = back;
			if (isQueueSettled && !isReadPending && book && isSameKey(shown, wanted)) void returns.restore(back);
		}
	});
	beforeNavigate(returns.leave);

	const rowDomId = (contractId: string, tableView: DataTableView) => `contract-row-${contractId}-${tableView}`;

	function rowLink(rowId: string): HTMLElement | undefined {
		if (rowId.startsWith('vr:')) {
			return document.querySelector<HTMLElement>(`#void-request-row-${CSS.escape(rowId.slice(3))} a`) ?? undefined;
		}
		return (
			(['table', 'record'] as const)
				.map((tableView) => document.querySelector<HTMLElement>(`#${CSS.escape(rowDomId(rowId, tableView))} a`))
				.find((link) => link?.checkVisibility()) ?? undefined
		);
	}

	// jscpd:ignore-start -- the landed read and the latest-search guard are the Invoices list's, over this list's own key; they share its reactive state, so a helper would take five closures
	let adopted: ContractsListData | undefined;
	$effect(() => {
		const landed = data;
		if (!landed || appliedSearch !== '') return;
		untrack(() => {
			if (landed === adopted && shown?.search === '') return;
			adopted = landed;
			adopt(landed, { view: landed.view, order: landed.order, isMine: landed.isMine, doula: landed.doula, search: '', period: landed.period }, landed.days);
		});
	});

	let latestSearch = 0;
	let isSearching = $state(false);
	$effect(() => {
		const key = wanted;
		if (key.search === '') {
			latestSearch += 1;
			untrack(() => (isSearching = false));
			return;
		}
		const thisSearch = ++latestSearch;
		untrack(() => void readSearch(key, thisSearch));
	});
	// jscpd:ignore-end
	async function readSearch(key: ReadKey, thisSearch: number) {
		isSearching = true;
		try {
			const days = sentPeriodDays(key.period, timeZone, now);
			const first = await loadPracticeContracts(apiFetchWithSession, page.params.practiceId!, { ...key, days });
			if (thisSearch === latestSearch) adopt(first, key, days);
		// jscpd:ignore-start -- the latest-search guard shares its state with this screen's own reactive reads; a helper would take five closures
		} catch {
			if (thisSearch === latestSearch) {
				failedKey = key;
				returns.held = undefined;
			}
		} finally {
			if (thisSearch === latestSearch) isSearching = false;
		}
	}
	// jscpd:ignore-end
	const isReadPending = $derived(appliedSearch === '' ? isReading : isSearching);
	$effect(() => {
		if (hasReadFailed) returns.held = undefined;
	});
	const hasSearchFailed = $derived(isSameKey(failedKey, wanted) && !isSameKey(shown, wanted));
	const readFailure = $derived(
		hasReadFailed && appliedSearch === '' ? LOAD_FAILED : (hasSearchFailed ? SEARCH_FAILED : '')
	);
	const isBusy = $derived(isApplying || (shown !== undefined && !isSameKey(shown, wanted) && readFailure === ''));
	const tryAgain = () => (appliedSearch === '' ? onRetry() : readSearch(wanted, ++latestSearch));

	const queueList = new PaginatedList<AwaitingVoidRequest>({
		first: { items: [], hasMore: false },
		loadPage: async (cursor) => {
			try {
				return await loadPracticeVoidRequests(apiFetchWithSession, page.params.practiceId!, cursor);
			} catch {
				throw new Error(QUEUE_MORE_FAILED);
			}
		},
		failureMessage: QUEUE_MORE_FAILED
	});
	let retriedQueue = $state.raw<VoidRequestsPage>();
	let queueTotal = $state(0);
	const landedQueue = $derived(queue ?? retriedQueue);
	$effect(() => {
		const landed = landedQueue;
		if (!landed) return;
		untrack(() => {
			queueList.reset(landed);
			queueTotal = landed.total;
		});
	});
	const isQueueFailed = $derived(isQueueExpected && landedQueue === undefined && hasQueueFailed);
	const isQueueSettled = $derived(!isQueueExpected || landedQueue !== undefined || hasQueueFailed);
	async function retryQueue() {
		try {
			retriedQueue = await loadPracticeVoidRequests(apiFetchWithSession, page.params.practiceId!);
		} catch {
			retriedQueue = undefined;
		}
	}
	$effect(() => {
		if (!isQueueSettled || !book || !returns.held || !isSameKey(shown, wanted)) return;
		const back = returns.held;
		untrack(() => void returns.restore(back));
	});

	const doulaNameOf = (id: string) => doulas.find((each) => each.value === id)?.label ?? NO_DOULA_FALLBACK;
	const isDoulaPending = $derived(!isRosterLanded && doula !== '');
	const doulaChoices = $derived(
		isDoulaPending ? [{ value: doula, label: '' }] : [{ value: '', label: ANY_DOULA }, ...doulas]
	);

	let draftView = $state<ContractsView>('not-signed');
	let draftOrder = $state<ContractOrder>('');
	let isMineDrafted = $state(false);
	let draftDoula = $state('');
	let draftSearch = $state('');
	const draftPeriod = new SentPeriodDraft();
	let searchBox = $state<HTMLElement>();
	let isSearchTouched = false;
	let filterBar = $state<{ focusStatus: () => void }>();
	let changeApplies = 0;
	function seedDrafts() {
		draftView = view;
		draftOrder = order;
		isMineDrafted = isMine;
		draftDoula = doula;
		draftPeriod.seed(period);
		if (!isSearchTouched) draftSearch = appliedSearch;
	}
	seedDrafts();
	beforeNavigate(() => {
		isSearchTouched = false;
	});
	focusResultAfterChange({
		seed: seedDrafts,
		wantedKey: () => JSON.stringify(wanted),
		isLanded: () => isSameKey(shown, wanted),
		isOwnChange: () => changeApplies > 0,
		focus: () => filterBar?.focusStatus()
	});

	type Address = { view: ContractsView; order: ContractOrder; isMine: boolean; doula: string; period?: SentPeriod | undefined };
	const hrefFor = (address: Address) => `${basePath}${contractsQueryString(address)}`;
	const current = $derived<Address>({ view, order, isMine, doula, period });
	const drawnHref = () =>
		hrefFor({
			view: draftView,
			order: orderForView(draftOrder, draftView),
			isMine: canMine && isMineDrafted,
			doula: canPickDoula ? draftDoula : doula,
			period: draftPeriod.period
		});

	async function showFilter(href: string, search: string, change?: { replace: boolean }) {
		contracts.abandon();
		isApplying = true;
		if (change) changeApplies += 1;
		try {
			await goto(href, {
				state: { contractsSearch: search },
				...(change && { replaceState: change.replace, reset: false })
			});
		} finally {
			isApplying = false;
			if (change) changeApplies -= 1;
		}
	}

	async function applyFilters(event: SubmitEvent) {
		event.preventDefault();
		if (await draftPeriod.refusesToApply()) return;
		await showFilter(drawnHref(), typedIn(searchBox, draftSearch));
	}

	// jscpd:ignore-start -- FilterBar's onChangeApply contract, which each list answers over its own address and search
	function applyOnChange({ replace, changed }: { replace: boolean; changed: readonly string[] }) {
		if (draftPeriod.isOnlyOpeningDays(changed) || draftPeriod.hasProblem()) return false;
		void showFilter(drawnHref(), typedIn(searchBox, draftSearch), { replace });
		return true;
	}
	// jscpd:ignore-end

	// A heading's press is the Order select's pick: the bar as drawn, with the order it asks for.
	function sortBy(picked: string) {
		draftOrder = orderForView(picked as ContractOrder, draftView);
		applyOnChange({ replace: false, changed: ['order'] });
	}

	const VIEW_OPTIONS: readonly { value: ContractsView; label: string }[] = [
		{ value: 'not-signed', label: 'Not signed' },
		{ value: 'draft', label: 'Draft' },
		{ value: 'sent', label: 'Sent' },
		{ value: 'signed', label: 'Signed' },
		{ value: 'voided', label: 'Voided' },
		{ value: 'all', label: 'All' }
	];

	type AppliedTag = { text: string; href: string; search: string };
	const appliedTags = $derived.by(() => {
		const tags: AppliedTag[] = [];
		const search = appliedSearch;
		const without = (change: Partial<Address>) => hrefFor({ ...current, ...change });
		const viewTag = CONTRACT_VIEW_TAGS[view];
		if (viewTag) tags.push({ text: viewTag, href: without({ view: 'not-signed', order: orderForView(order, 'not-signed') }), search });
		if (isMine) tags.push({ text: 'Mine', href: without({ isMine: false }), search });
		if (doula && isRosterLanded) tags.push({ text: `Doula: ${doulaNameOf(doula)}`, href: without({ doula: '' }), search });
		if (wantedDays) tags.push({ text: `Sent ${sentPeriodWords(wantedDays)}`, href: without({ period: undefined }), search });
		if (search) tags.push({ text: `Matching "${search}"`, href: hrefFor(current), search: '' });
		return tags;
	});

	function removeTag(event: MouseEvent, tag: AppliedTag) {
		event.preventDefault();
		void showFilter(tag.href, tag.search);
	}

	const isNarrowed = $derived(view !== 'not-signed' || order !== '' || isMine || doula !== '' || period !== undefined || appliedSearch !== '');

	const resultSummary = $derived(
		shown && book
			? contractsResultSummary({
					view: shown.view,
					order: shown.order,
					shown: contracts.items.length,
					hasMore: contracts.hasMore,
					total: book.total,
					isMine: shown.isMine,
					doulaName: shown.doula ? doulaNameOf(shown.doula) : '',
					search: shown.search,
					days: shownDays
				})
			: ''
	);

	const isBookEmpty = $derived(
		shown !== undefined &&
			shown.view === 'all' &&
			!shown.isMine &&
			shown.doula === '' &&
			shown.search === '' &&
			shown.period === undefined &&
			contracts.items.length === 0
	);
	const emptyBookWords = $derived(
		isContractor ? 'No Contract on your Engagements yet.' : resultSummary
	);

	const emptyListAction = $derived.by((): EmptyStateAction => {
		const search = shown?.search ?? '';
		if (search !== '' && shown?.view !== 'all') {
			const allHref = hrefFor({
				view: 'all',
				order: orderForView(shown?.order ?? '', 'all'),
				isMine: shown?.isMine ?? false,
				doula: shown?.doula ?? '',
				period: shown?.period
			});
			return { label: `Search all Contracts for "${search}"`, onClick: () => void showFilter(allHref, search) };
		}
		if (shown?.period) {
			return {
				label: 'Show Contracts sent any time',
				href: hrefFor({ view: shown.view, order: shown.order, isMine: shown.isMine, doula: shown.doula })
			};
		}
		if (shown?.isMine) {
			return { label: "Show everyone's Contracts", href: hrefFor({ view: shown.view, order: shown.order, isMine: false, doula: shown.doula }) };
		}
		return { label: 'Show all Contracts', href: hrefFor({ view: 'all', order: '', isMine: false, doula: '' }) };
	});

	/*
	 * #2954: the export reads the list's applied view, search, order, Mine,
	 * Doula and period, as a POST so the search, which can be a Client's name,
	 * stays out of the address, and it needs script to hand the file over.
	 */
	const csv = new CsvDownload<ReadKey | undefined>();
	const csvWords = $derived(csv.wordsWhile((key) => key === shown && isSameKey(shown, wanted)));
	const isExportDrawn = $derived(csv.isScripted && isAdmin);
	const hasExportRows = $derived(shown !== undefined && readFailure === '' && contracts.items.length > 0);
	const exportCount = $derived(contracts.hasMore ? book?.total : contracts.items.length);
	const exportDescription = $derived(
		exportCount === undefined
			? 'A spreadsheet of every Contract this list shows.'
			: `A spreadsheet of the ${exportCount} ${exportCount === 1 ? 'Contract' : 'Contracts'} this list shows.`
	);
	async function downloadCsv() {
		if (!shown) return;
		const { view, order, isMine, doula, search } = shown;
		await csv.run(
			shown,
			() => requestContractsExport(apiFetchWithSession, page.params.practiceId!, { view, order, isMine, doula, search, days: shownDays }),
			contractsDownloadedWords
		);
	}

	let downloadingId = $state('');
	let pdfRefusal = $state('');
	let downloaded = $state.raw<{ key: ReadKey | undefined; words: string }>();
	const downloadedWords = $derived(downloaded && downloaded.key === shown && isSameKey(shown, wanted) ? downloaded.words : '');
	async function downloadPdf(contract: ContractListItem) {
		downloadingId = contract.contractId;
		pdfRefusal = '';
		downloaded = undefined;
		const key = shown;
		try {
			triggerBlobDownload(
				await downloadSignedContractPdf(apiFetchWithSession, page.params.practiceId!, contract.engagementId),
				signedContractFilename(contract.clientName)
			);
			downloaded = { key, words: signedPdfDownloadedWords(contract.clientName) };
		} catch {
			pdfRefusal = PDF_FAILED;
		} finally {
			downloadingId = '';
		}
	}

	const REMIND_FAILED = 'The reminder did not send.';
	let remindingId = $state('');
	let remindRefusal = $state('');
	let remindedWords = $state('');
	async function remind(contract: ContractListItem) {
		remindingId = contract.contractId;
		remindRefusal = '';
		remindedWords = '';
		try {
			const reminded = await remindContract(apiFetchWithSession, page.params.practiceId!, contract.engagementId);
			contract.remindedOn = calendarDayIn(timeZone, new Date(reminded.reminded?.at ?? Date.now()));
			for (const item of contracts.items) {
				if (item.clientId !== contract.clientId || item.status !== 'sent') continue;
				item.canRemind = false;
				item.canRemindOn = reminded.canRemindOn;
			}
			remindedWords = `Reminder on its way to ${contract.clientName}.`;
		} catch (error_) {
			remindRefusal = error_ instanceof Error ? error_.message : REMIND_FAILED;
		} finally {
			remindingId = '';
		}
	}

	const INSET = 'var(--data-table-cell-inset)';
	const STATUS_STATE_TRACK = '9.5ch';
	const STATUS_DAY_TRACK = '7ch';
	const STATUS_OPENED_TRACK = '13ch';
	const NO_ONE_ON_RECORD = 'No one on record';
	const VOID_REQUESTED = 'void requested';
	const formatDay = (day: string) => formatCalendarDayThisYear(day, timeZone, now);
	const canDownload = (contract: ContractListItem) => contract.status === 'signed' && contract.hasSignedPdf;

	function statusText(contract: ContractListItem): string {
		const day = formatDay(contract.steppedOn);
		if (contract.status === 'draft') return `Draft made ${day}`;
		if (contract.status === 'sent') {
			const sent = contract.daysWaiting ? `Sent ${day}, waiting ${daysWaitingWords(contract.daysWaiting)}` : 'Sent today';
			return `${sent}, ${openedText(contract, 'run-on')}`;
		}
		const word = `${contractStatusWord(contract.status)} ${day}`;
		return contract.voidRequested ? `${word}, ${VOID_REQUESTED}` : word;
	}

	function openedText(contract: ContractListItem, form: 'cell' | 'run-on'): string {
		const words = contract.openedOn ? `opened ${formatDay(contract.openedOn)}` : 'not opened';
		return form === 'cell' ? words.charAt(0).toUpperCase() + words.slice(1) : words;
	}

	function whoText(contract: ContractListItem): string {
		const step = contractStepper(contract);
		return step ? `${step.verb} by ${step.name}` : '';
	}

	const doulaText = (contract: ContractListItem) =>
		contract.doulaNames?.length ? contract.doulaNames.join(', ') : NO_DOULA_YET_LABEL;

	function careText(contract: ContractListItem): string {
		const { lead, day } = careWhen(contract, timeZone, formatDay);
		return day ? `${kindLabel(contract.engagementKind)} · ${lead} ${day}` : kindLabel(contract.engagementKind);
	}

	const isRemindable = (contract: ContractListItem) => contract.status === 'sent';
	const remindedText = (contract: ContractListItem) =>
		contract.remindedOn ? `Reminded ${formatDay(contract.remindedOn)}` : '';
	const reminderText = (contract: ContractListItem) => {
		if (contract.canRemind) return [remindedText(contract), 'Remind the Client'].filter(Boolean).join(', ');
		const next = contract.canRemindOn ? `Remind again ${formatDay(contract.canRemindOn)}` : '';
		return [remindedText(contract), next].filter(Boolean).join(', ');
	};
	const columns = $derived([
		{ label: 'Client', accessor: (contract: ContractListItem) => contract.clientName, grows: true, track: '16ch' },
		{
			label: 'Status',
			track: `calc(${STATUS_STATE_TRACK} + ${STATUS_DAY_TRACK} + ${STATUS_OPENED_TRACK} + 2 * var(--space-1) + 2 * ${INSET})`,
			accessor: statusText,
			content: statusCell
		},
		{
			label: 'Who',
			track: `calc(8ch + 2 * ${INSET})`,
			accessor: whoText,
			content: whoCell
		},
		...(shown?.doula
			? []
			: [
					{
						label: 'Doula',
						track: `calc(16ch + 2 * ${INSET})`,
						accessor: doulaText,
						content: doulaCell
					}
				]),
		{
			label: 'Care',
			track: `calc(20ch + 2 * ${INSET})`,
			accessor: careText,
			content: careCell
		},
		...(isContractor
			? []
			: [
					{
						label: 'Fee',
						recordLabel: 'Fee',
						numeric: true,
						track: `calc(8ch + 2 * ${INSET})`,
						accessor: (contract: ContractListItem) =>
							contract.amountCents === undefined ? '' : formatAmount(contract.amountCents)
					}
				]),
		...(!isContractor && shown?.view === 'all'
			? [
					{
						label: 'Action',
						track: `calc(8ch + 2 * ${INSET})`,
						accessor: (contract: ContractListItem) =>
							canDownload(contract) ? 'Download' : (isRemindable(contract) ? reminderText(contract) : ''),
						content: actionCell
					}
				]
			: [
					...(shown?.view === 'not-signed' || shown?.view === 'sent' || shown?.view === 'all'
						? [
								{
									label: 'Reminder',
									track: `calc(8ch + 2 * ${INSET})`,
									accessor: (contract: ContractListItem) => (isRemindable(contract) ? reminderText(contract) : ''),
									content: reminderCell
								}
							]
						: []),
					...(isContractor || shown?.view !== 'signed'
						? []
						: [
								{
									label: 'Signed PDF',
									track: `calc(8ch + 2 * ${INSET})`,
									accessor: (contract: ContractListItem) => (canDownload(contract) ? 'Download' : ''),
									content: downloadCell
								}
							])
				])
	]);

	function engagementHref(contract: ContractListItem): string {
		const engagement = resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: contract.engagementId
		});
		return `${engagement}#contract-${contract.contractId}`;
	}

	function requestHref(request: AwaitingVoidRequest): string {
		const engagement = resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: request.engagementId
		});
		return `${engagement}#void-request-${request.requestId}`;
	}

	const rowLinkLabel = $derived(
		rowLinkLabeler(
			contracts.items,
			(contract) => contract.clientName,
			(contract) => `${kindLabel(contract.engagementKind)} Contract, ${contract.status}`
		)
	);
</script>

{#snippet statusCell(contract: ContractListItem, tableView: DataTableView)}
	<span
		class="status"
		class:tracked={tableView === 'table' && !contract.voidRequested}
		style:--status-state-track={STATUS_STATE_TRACK}
		style:--status-day-track={STATUS_DAY_TRACK}
		style:--status-opened-track={STATUS_OPENED_TRACK}
	>
		{#if contract.status === 'draft'}
			<Badge variant="warning" label="Draft" />
			<span class="day made"><span class="lead">made</span> <time datetime={contract.steppedOn}>{formatDay(contract.steppedOn)}</time></span>
		{:else if contract.status === 'sent' && tableView === 'table'}
			<Badge variant="info" label="Sent" />
			<span class="day"
				>{#if contract.daysWaiting}<time class="visually-hidden" datetime={contract.steppedOn}>{formatDay(contract.steppedOn)}</time><span
						class="visually-hidden">, waiting </span
					>{daysWaitingWords(contract.daysWaiting)}{:else}today{/if}</span
			>
			<span class="opened">{openedText(contract, 'cell')}</span>
		{:else if contract.status === 'sent'}
			<Badge variant="info" label="Sent" />
			{#if contract.daysWaiting}
				<span class="day"><time datetime={contract.steppedOn}>{formatDay(contract.steppedOn)}</time><span aria-hidden="true">,</span><span class="visually-hidden">, waiting</span></span>
				<span class="wait">{daysWaitingWords(contract.daysWaiting)}<span aria-hidden="true">,</span> {openedText(contract, 'run-on')}</span>
			{:else}
				<span class="day">today<span aria-hidden="true">,</span> {openedText(contract, 'run-on')}</span>
			{/if}
		{:else}
			<span>{contractStatusWord(contract.status)}</span>
			<span class="day"><time datetime={contract.steppedOn}>{formatDay(contract.steppedOn)}</time></span>
			{#if contract.voidRequested}
				<Badge variant="warning" label="Void requested" />
			{/if}
		{/if}
	</span>
{/snippet}

{#snippet whoCell(contract: ContractListItem, tableView: DataTableView)}
	{@const step = contractStepper(contract)}
	{#if step && tableView === 'table'}
		<span class="no-break"
			><span aria-hidden="true">{nameWithInitial(step.name)}</span><span class="visually-hidden">{whoText(contract)}</span></span
		>
	{:else if step}
		{whoText(contract)}
	{:else}
		<span aria-hidden="true">—</span><span class="visually-hidden">{NO_ONE_ON_RECORD}</span>
	{/if}
{/snippet}

{#snippet doulaCell(contract: ContractListItem, tableView: DataTableView)}
	{@const names = contract.doulaNames ?? []}
	{#if names.length === 0}
		{doulaText(contract)}
	{:else}
		{#if tableView === 'record'}<span aria-hidden="true">Doula&nbsp;</span>{/if}
		<DoulaNames {names} ids={contract.doulaIds} readerId={session.staffId} isTable={tableView === 'table'} />
	{/if}
{/snippet}

{#snippet careCell(contract: ContractListItem)}
	{@const due = careWhen(contract, timeZone, formatDay)}
	<span class="no-break"
		>{kindLabel(contract.engagementKind)}{#if due.day}<span class="visually-hidden">, </span><span class="separator" aria-hidden="true">·</span><span class="lead">{due.lead}</span> {due.day}{:else}<span class="visually-hidden">, no due date</span>{/if}</span
	>
{/snippet}

{#snippet remindWord()}<span aria-hidden="true">Remind</span>{/snippet}

{#snippet reminderCell(contract: ContractListItem, tableView: DataTableView)}
	{#if isRemindable(contract)}
		<span class="reminder">
			{#if contract.remindedOn && !(tableView === 'table' && contract.canRemind)}
				<span class:visually-hidden={tableView === 'table'}>{remindedText(contract)}</span>
			{/if}
			{#if contract.canRemind}
				{#if tableView === 'table'}
					<Button
						variant="link"
						iconOnly
						visual={remindWord}
						label={`Remind the Client, ${contract.clientName}${contract.remindedOn ? `, last reminded ${formatDay(contract.remindedOn)}` : ''}`}
						loading={remindingId === contract.contractId}
						onClick={() => remind(contract)}
					/>
				{:else}
					<Button
						variant="secondary"
						size="sm"
						label="Remind the Client"
						loading={remindingId === contract.contractId}
						onClick={() => remind(contract)}
					/>
				{/if}
			{:else if contract.canRemindOn}
				{#if tableView === 'table'}
					<span aria-hidden="true">Tomorrow</span>
					<span class="visually-hidden">Remind the Client {formatDay(contract.canRemindOn)}</span>
				{:else}
					<span>Remind again {formatDay(contract.canRemindOn)}</span>
				{/if}
			{/if}
		</span>
	{/if}
{/snippet}

{#snippet actionCell(contract: ContractListItem, tableView: DataTableView)}
	{#if canDownload(contract)}
		{@render downloadCell(contract, tableView)}
	{:else}
		{@render reminderCell(contract, tableView)}
	{/if}
{/snippet}

{#snippet downloadWord()}<span aria-hidden="true">Download</span>{/snippet}

{#snippet downloadCell(contract: ContractListItem, tableView: DataTableView)}
	{#if canDownload(contract)}
		{#if tableView === 'table'}
			<Button
				variant="link"
				iconOnly
				visual={downloadWord}
				label={`Download signed Contract (PDF), ${contract.clientName}`}
				loading={downloadingId === contract.contractId}
				onClick={() => downloadPdf(contract)}
			/>
		{:else}
			<Button
				variant="secondary"
				size="sm"
				label="Download signed Contract (PDF)"
				loading={downloadingId === contract.contractId}
				onClick={() => downloadPdf(contract)}
			/>
		{/if}
	{/if}
{/snippet}

{#snippet applied()}
	{#each appliedTags as tag (tag.text)}
		<li>
			<Link
				href={tag.href}
				label={tag.text}
				variant="chip"
				icon="x"
				ariaLabel={`Remove filter: ${tag.text.replaceAll(/[":]/g, '')}`}
				onClick={(event) => removeTag(event, tag)}
			/>
		</li>
	{/each}
{/snippet}

{#snippet intro()}
	<Text text="Each Contract you can open, and where it stands. A row opens its Engagement." tone="muted" />
{/snippet}

{#snippet queueSection()}
	{#if isQueueFailed}
		<ReadFailure message={QUEUE_FAILED} onRetry={retryQueue} />
	{:else if queueTotal > 0}
		<section class="queue" aria-labelledby="void-requests-heading"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
			<cluster-l space="var(--space-2)" align="baseline">
				<Heading level={2} variant="section" id="void-requests-heading" text={voidRequestsHeading(queueTotal)} />
				<Text step="meta" tone="muted" text="Open one to void its Contract or decline it, both on the Engagement." />
			</cluster-l>
			<VoidRequestList requests={queueList.items} hrefFor={requestHref} />
			{#if queueList.hasMore}
				<Button
					variant="secondary"
					label="Load more void requests"
					loading={queueList.isLoadingMore}
					onClick={() => queueList.loadMore()}
				/>
			{/if}
			{#if queueList.loadMoreError}
				<Notice message={queueList.loadMoreError} variant="error" />
			{/if}
		</section>
	{:else}
		<Text text="No void request is waiting for a decision." tone="muted" />
	{/if}
{/snippet}

{#snippet searchField()}
	<div class="search" bind:this={searchBox}>
		<LabeledField label="Client" width={20}>
			{#snippet children({ id, describedBy, invalid, width })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					{width}
					type="search"
					maxlength={100}
					autocomplete="off"
					value={draftSearch}
					onInput={(value) => {
						isSearchTouched = true;
						draftSearch = value;
					}}
				/>
			{/snippet}
		</LabeledField>
	</div>
{/snippet}

{#snippet content()}
	{#if !isQueueSettled}
		<Skeleton variant="row" lines={6} label="Loading your Contracts" />
	{:else}
		{#if isQueueExpected}
			{@render queueSection()}
		{/if}
		<DownloadCsvNotices refusal={csv.refusal} downloaded={csvWords} />
		{#if pdfRefusal}
			<Notice variant="error" message={pdfRefusal} />
		{/if}
		{#if remindRefusal}
			<Notice variant="error" message={remindRefusal} />
		{/if}
		{#if remindedWords}
			<Notice variant="status" message={remindedWords} isFocusedOnAppear />
		{/if}
		<stack-l class="bar-and-list" space="var(--space-related)">
			{#if !isBookEmpty}
				<FilterBar
					bind:this={filterBar}
					action={basePath}
					onSubmit={applyFilters}
					onChangeApply={applyOnChange}
					legend="Choose which Contracts to show"
					status={readFailure ? '' : [resultSummary, downloadedWords].filter(Boolean).join(' ')}
					statusRuler={CONTRACTS_RESULT_RULER}
					statusId={RESULT_ID}
					empty={shown !== undefined && contracts.items.length === 0}
					clearHref={isNarrowed ? basePath : undefined}
					applied={appliedTags.length > 0 ? applied : undefined}
					appliedCount={appliedTags.length}
					hasUntaggedFilter={order !== '' || view === 'all'}
					foldsFieldsOnly
					foldsSwitches
					foldLabel="Filters and order"
					order={{
						options: contractOrderOptions(draftView),
						alsoSizedBy: EVERY_CONTRACT_ORDER_OPTION,
						value: draftOrder,
						onChange: (value) => (draftOrder = value as ContractOrder)
					}}
					{isBusy}
				>
					{#snippet switches()}
						<RadioGroup
							legend="Show"
							name="show"
							options={VIEW_OPTIONS}
							value={draftView}
							onChange={(chosen) => {
								draftView = chosen;
								draftOrder = orderForView(draftOrder, chosen);
							}}
							isInline
						/>
						{#if canMine}
							<ShowOnly>
								<LabeledField label="Mine" orientation="inline">
									{#snippet children(control)}
										<Checkbox name="mine" value="true" checked={isMineDrafted} onChange={(checked) => (isMineDrafted = checked)} {...control} />
									{/snippet}
								</LabeledField>
							</ShowOnly>
						{/if}
					{/snippet}
					{@render searchField()}
					{#snippet folded()}
						{#if draftView !== 'draft'}
							<SentPeriodFields
								bind:choice={draftPeriod.choice}
								bind:from={draftPeriod.from}
								bind:to={draftPeriod.to}
								fromError={draftPeriod.errors.from}
								toError={draftPeriod.errors.to}
							/>
						{/if}
						{#if canPickDoula}
							<div class="field doula">
								<LabeledField label="Doula" width={30}>
									{#snippet children({ id, describedBy, invalid, width })}
										<Select
											{id}
											{describedBy}
											{invalid}
											{width}
											name="doula"
											options={doulaChoices}
											disabled={!isRosterLanded}
											bind:value={draftDoula}
										/>
									{/snippet}
								</LabeledField>
							</div>
						{/if}
					{/snippet}
				</FilterBar>
			{/if}

			{#if readFailure}
				<ReadFailure message={readFailure} onRetry={tryAgain} />
			{:else if shown}
				<div class="contracts-table">
					<DataTable
						columns={columns}
						rows={contracts.items}
						rowHref={engagementHref}
						rowId={(contract, tableView) => rowDomId(contract.contractId, tableView)}
						{rowLinkLabel}
						hasMore={contracts.hasMore}
						onLoadMore={returns.didLoadMore}
						isLoadingMore={contracts.isLoadingMore}
						loadMoreError={contracts.loadMoreError}
						emptyMessage={isBookEmpty ? emptyBookWords : resultSummary}
						emptyAction={isBookEmpty ? undefined : emptyListAction}
						isEmptyMessageSpoken={isBookEmpty}
						caption="Contracts"
						describedBy={isBookEmpty ? undefined : RESULT_ID}
						columnWidths="one-line"
						recordLayout="compact"
						orderedBy={contractOrderedBy(shown.order, shown.view)}
						sortOrders={contractColumnOrders(shown.view)}
						onSort={sortBy}
						{isBusy}
					/>
				</div>
			{:else}
				<Skeleton variant="row" lines={6} label="Loading your Contracts" />
			{/if}
		</stack-l>
	{/if}
{/snippet}

{#snippet actions()}
	{#if isExportDrawn}
		<DownloadCsvButton isExporting={csv.isExporting} hasRows={hasExportRows} description={exportDescription} onDownload={downloadCsv} />
	{/if}
{/snippet}

<ListPage title="Contracts" {intro} {content} {actions} />

<style>
	@layer components {
		.queue {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
			align-items: start;
		}

		.contracts-table {
			container: contracts-table / inline-size;
		}

		.contracts-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: grid;
		}

		@container contracts-table (min-width: 75rem) {
			.contracts-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}

		.contracts-table :global(th.numeric) {
			padding-inline-end: var(--data-table-cell-inset);
		}

		.status {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1);
		}

		:global(td) .status:not(.tracked) {
			flex-wrap: nowrap;
			white-space: nowrap;
		}

		.status.tracked {
			display: grid;
			justify-items: start;
			grid-template-columns: var(--status-state-track) var(--status-day-track) var(--status-opened-track);
			white-space: nowrap;
			font-variant-numeric: tabular-nums;
		}

		:global(td) .no-break {
			flex-wrap: nowrap;
			white-space: nowrap;
		}

		.reminder {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1);
		}

		:global(td) .reminder {
			flex-wrap: nowrap;
			white-space: nowrap;
		}

		.status.tracked .made {
			grid-column: 2 / span 2;
		}

		.lead {
			color: var(--color-on-surface-variant);
		}

		.separator {
			padding-inline: var(--space-1);
			color: var(--color-on-surface-variant);
		}

		.field {
			--field-width: 34ch;
			flex: 1 1 min(var(--field-width), calc(50% - var(--space-related) / 2));
			max-inline-size: var(--field-width);
		}

		.search {
			flex: 0 1 max-content;
			max-inline-size: max-content;
		}

		@container filter-bar (max-width: 30rem) {
			.search {
				flex: 1 1 7rem;
				max-inline-size: none;
				min-inline-size: 0;
			}

			.search :global(input) {
				max-inline-size: none;
			}

			.search :global(label) {
				white-space: nowrap;
			}
		}
	}
</style>
