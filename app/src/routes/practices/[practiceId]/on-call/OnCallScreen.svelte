<script lang="ts">
	/*
	 * Who is on call (#1093). The question an agency owner asks every
	 * evening -- who is on call tonight, and is anyone covering two
	 * births at once -- with a hole shown as a hole rather than hidden.
	 *
	 * The Schedule's sibling (#2748): `ListPage` for the frame, `FilterBar`
	 * for the narrowing and its applied tags, `DataTable` for the book. The
	 * narrowing is `FilterBar`'s real `<form method="get">`; `applyFilters`
	 * upgrades it to a client-side navigation rather than replacing it (Rule
	 * of Least Power), and the address is the state.
	 *
	 * What a reader sees is the BFF's answer and nothing more: a contractor
	 * Doula's roster carries only births she is on, decided in the query, so
	 * this screen hides nothing and needs to hide nothing. The roster is one
	 * whole answer for the days chosen, never a page of one: the births that
	 * need cover and each Doula's births are counted over all of it.
	 */
	import { afterNavigate, beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { tick, untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { addDays, daysOf, daysTag, daysWords, isCalendarDay, rangeErrors as rangeErrorsOf, type RangeErrors } from '#lib/dayRange.js';
	import { formatCalendarWeekdayDay } from '#lib/dates.js';
	import { keepListReturn, restoreList, type ListReturn } from '#lib/listReturn.js';
	import {
		COVER_FIRST,
		DEFAULT_ON_CALL_DAYS,
		MOST_ON_CALL_DAYS,
		SHORT_ON_CALL_DAYS,
		TONIGHT_DAYS,
		atOnceLabel,
		daysOnCall,
		daysOnCallText,
		describeCoverage,
		gapSubject,
		isOnCallNarrowed,
		isTonightOnHerLine,
		onCallOrderOptions,
		ownStartWords,
		onCallEmptyNextStep,
		onCallGroup,
		onCallHref,
		onCallLimits,
		onCallResultRuler,
		onCallResultSummary,
		onCallRuleSentence,
		onCallStep,
		onCallViewFromParameters,
		tonightLine,
		withDefaultDays,
		type DayRange,
		type OnCallDaysChoice,
		type OnCallOrder,
		type OnCallView,
		type NoWindowRow,
		type RosterDoula,
		type RosterWindow,
		type UnreachableGap
	} from '#lib/onCall.js';
	import { canNarrowToMine, isOwnerOrAdmin } from '#lib/roles.js';
	import { practiceZoneNote } from '#lib/practiceZone.js';
	import { dayIn } from '#lib/zonedTime.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import type { EmptyStateAction } from '#lib/components/molecules/EmptyState.svelte';
	import CoverageGapWhen from '#lib/components/molecules/CoverageGapWhen.svelte';
	import DateRangeFields from '#lib/components/molecules/DateRangeFields.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PracticeSession } from '../+layout.js';
	import type { OnCallPageData } from './+page.js';

	/*
	 * `data` is the read once it lands (#2255). Until then the screen draws
	 * what the URL and the session already say -- the filter bar -- and a
	 * row skeleton where the births will land.
	 */
	let {
		data,
		isReading = false,
		hasReadFailed = false,
		onRetry = async () => {}
	}: {
		data?: OnCallPageData;
		// The page's read is running, so `data` is still the one before it.
		isReading?: boolean;
		// The page's read failed in a way that may work a second time
		// (`+page.svelte`); `onRetry` runs it again.
		hasReadFailed?: boolean;
		onRetry?: () => Promise<void>;
	} = $props();

	const LOAD_FAILED = 'Who is on call did not load. Nothing you saved is lost.';
	const RESULT_ID = 'on-call-result';
	const ANY_DOULA = 'Any Doula';
	const NEEDS_COVER = 'Needing cover';
	const UNKNOWN_DOULA = 'that Doula';
	const NO_DOULA = 'No Doula yet';
	const TO = ' to ';
	const SEPARATOR = ', ';
	const ON_CALL_LABEL = 'On call ';
	const DUE_LABEL = 'Due ';
	const TONIGHT_LABEL = 'Tonight: ';

	const practiceId = $derived(page.params.practiceId!);
	const basePath = $derived(resolve('/practices/[practiceId]/on-call', { practiceId: page.params.practiceId! }));
	const session = $derived((page.data as { session: PracticeSession }).session);
	// The Practice's own zone (ADR-0036): every day and time on this screen is read in it, never the reader's.
	const timeZone = $derived(session.timezone);
	const canMine = $derived(canNarrowToMine(session));
	// Owner and Admin only: the Doula picker and "Needing cover" are what the BFF serves them alone.
	const isOwnerView = $derived(isOwnerOrAdmin(session));
	const view = $derived(
		onCallViewFromParameters(page.url.searchParams, canMine, isOwnerView ? undefined : { staffId: session.staffId })
	);
	const viewKey = (of: OnCallView) => onCallHref('', of);
	const wantedKey = $derived(viewKey(view));

	/*
	 * The instant "Tonight" is read from: the landed read's own, so a heading
	 * and the days cannot disagree, and a fresh one when the tab comes back to
	 * the front, so a heading left open past midnight does not keep saying it.
	 */
	let now = $state(untrack(() => data?.now) ?? new Date());
	function refreshNow() {
		if (document.visibilityState === 'visible') now = new Date();
	}
	const today = $derived(dayIn(now, timeZone));

	const roster = $derived(data?.roster);
	let knownDoulas = $state.raw<RosterDoula[]>(untrack(() => data?.roster?.doulas ?? []));
	$effect(() => {
		const landed = data;
		if (landed) now = landed.now;
		const doulas = landed?.roster?.doulas;
		if (doulas) untrack(() => (knownDoulas = doulas));
	});
	const doulaNameOf = (id: string) => knownDoulas.find((doula) => doula.staffId === id)?.name ?? UNKNOWN_DOULA;
	const doulaChoices = $derived(knownDoulas.map((doula) => ({ value: doula.staffId, label: doula.name })));

	const readFailure = $derived(hasReadFailed ? LOAD_FAILED : '');
	const isReadLanded = $derived(data !== undefined && viewKey(data.view) === wantedKey);

	/*
	 * Leaving this entry keeps how far down she was and the birth she left
	 * for (`keepListReturn`); Back puts her where she was, with focus on that
	 * row. The place is held until the read for this entry lands, and any move
	 * before it lands drops it.
	 */
	let heldReturn: ListReturn | undefined;
	let navigations = 0;
	keepListReturn({
		id: 'on-call-list',
		pagesLoaded: () => 1,
		rowOf: (to) => to.pathname.split('/engagements/', 2)[1]?.split('/', 1)[0],
		restore: (back) => {
			if (readFailure) return;
			heldReturn = back;
			if (!isReading && isReadLanded) void restoreReturn(back);
		}
	});
	beforeNavigate(() => {
		heldReturn = undefined;
		navigations += 1;
	});

	async function restoreReturn(back: ListReturn) {
		heldReturn = undefined;
		await restoreList(back, {
			pagesLoaded: () => 1,
			navigations: () => navigations,
			loadMore: async () => false,
			inFlight: () => Promise.resolve(),
			rowLink
		});
		if (back.rowId) rowLink(back.rowId)?.closest('tr')?.scrollIntoView({ block: 'nearest' });
	}

	const rowDomId = (engagementId: string, of: DataTableView) => `on-call-row-${engagementId}-${of}`;

	// Both trees are in the DOM (DataTable's #666); the displayed one wins.
	function rowLink(engagementId: string): HTMLElement | undefined {
		return (
			(['table', 'record'] as const)
				.map((of) => document.querySelector<HTMLElement>(`#${rowDomId(engagementId, of)} a`))
				.find((link) => link?.checkVisibility()) ?? undefined
		);
	}

	$effect(() => {
		if (hasReadFailed) heldReturn = undefined;
	});
	$effect(() => {
		if (!data) return;
		untrack(() => {
			if (heldReturn) void restoreReturn(heldReturn);
		});
	});

	/*
	 * What the controls hold before Apply, put back in step with the address
	 * after every navigation. "Choose days" opens the From and To fields; a
	 * range the address names that is not one is said in place, beside them.
	 */
	let draftChoice = $state<OnCallDaysChoice>('30');
	let fromValue = $state('');
	let toValue = $state('');
	let isMineChecked = $state(false);
	let isNeedsCoverChecked = $state(false);
	let draftDoula = $state('');
	let draftOrder = $state<OnCallOrder>('');
	let rangeErrors = $state<RangeErrors>({});

	const presetOf = (choice: OnCallDaysChoice) =>
		choice === 'tonight' ? TONIGHT_DAYS : (choice === '7' ? SHORT_ON_CALL_DAYS : DEFAULT_ON_CALL_DAYS);
	const limits = $derived(onCallLimits(today));
	const choiceOf = (of: OnCallView): OnCallDaysChoice => {
		if (of.chosen) return 'choose';
		if (of.preset === TONIGHT_DAYS) return 'tonight';
		return of.preset === SHORT_ON_CALL_DAYS ? '7' : '30';
	};

	function seedDrafts() {
		draftChoice = choiceOf(view);
		const shown = view.chosen ?? daysOf(view, timeZone, now).days!;
		fromValue = shown.from;
		toValue = shown.to;
		isMineChecked = view.isMine;
		isNeedsCoverChecked = view.needsCover;
		draftDoula = view.doula;
		draftOrder = view.order;
		rangeErrors = view.chosen ? rangeErrorsOf(view.chosen.from, view.chosen.to, limits) : {};
	}
	seedDrafts();
	$effect(() => {
		const refused = data?.rangeErrors;
		if (refused) rangeErrors = refused;
	});
	let filterBar = $state<{ focusStatus: () => void }>();
	let isFocusPending = $state(false);
	let lastKey: string | undefined;
	afterNavigate(() => {
		seedDrafts();
		// A change of narrowing lands on the result line once its list has
		// loaded, never on first arrival, where the route announcer speaks.
		if (changeApplies === 0 && lastKey !== undefined && lastKey !== wantedKey) isFocusPending = true;
		lastKey = wantedKey;
	});
	$effect(() => {
		if (!isFocusPending || !isReadLanded) return;
		isFocusPending = false;
		filterBar?.focusStatus();
	});

	// Picking "Choose days" starts its fields on the days the choice before it showed.
	function chooseDays(choice: OnCallDaysChoice) {
		if (choice === 'choose' && draftChoice !== 'choose') {
			const shown = daysOf({ preset: presetOf(draftChoice) }, timeZone, now).days!;
			fromValue = shown.from;
			toValue = shown.to;
		}
		draftChoice = choice;
	}

	const draftView = $derived<OnCallView>({
		preset: presetOf(draftChoice),
		chosen: draftChoice === 'choose' ? { from: fromValue, to: toValue } : undefined,
		isMine: canMine && isMineChecked,
		doula: draftDoula,
		needsCover: isOwnerView && isNeedsCoverChecked,
		order: draftOrder
	});

	const isChoosingDays = $derived(draftChoice === 'choose');

	let isApplying = $state(false);
	// Moves a change of control began and has not finished: the list lands without taking focus from the control.
	let changeApplies = 0;
	async function show(href: string, change?: { replace: boolean }) {
		isApplying = true;
		if (change) changeApplies += 1;
		try {
			await goto(href, change && { replaceState: change.replace, reset: false });
		} finally {
			isApplying = false;
			if (change) changeApplies -= 1;
		}
	}

	// The screen's own check comes first; the BFF's refusal of the same days is the second.
	function problemsOfDraft(): RangeErrors {
		return draftChoice === 'choose' ? rangeErrorsOf(fromValue, toValue, limits) : {};
	}

	// A box, a radio or a select applies the bar as drawn; "Choose days" alone only opens its fields.
	function applyOnChange({ replace, changed }: { replace: boolean; changed: readonly string[] }) {
		if (draftChoice === 'choose') {
			if (changed.length === 1 && changed[0] === 'days') return false;
			const problems = problemsOfDraft();
			if (problems.from || problems.to) {
				rangeErrors = problems;
				return false;
			}
		}
		rangeErrors = {};
		void show(onCallHref(basePath, draftView), { replace });
		return true;
	}

	async function applyFilters(event: SubmitEvent) {
		event.preventDefault();
		const problems = problemsOfDraft();
		if (problems.from || problems.to) {
			rangeErrors = problems;
			await tick();
			document.querySelector<HTMLInputElement>(problems.from ? 'input[name="from"]' : 'input[name="to"]')?.focus();
			return;
		}
		rangeErrors = {};
		await show(onCallHref(basePath, draftView));
	}

	/*
	 * The GOV.UK "selected filters": one removable tag for each narrowing the
	 * address names, each a link to the list minus that one. The default days
	 * and the order draw no tag; "Clear filters" goes to the bare path.
	 */
	type AppliedTag = { text: string; href: string };
	const appliedTags = $derived.by(() => {
		const tags: AppliedTag[] = [];
		const without = (change: Partial<OnCallView>) => onCallHref(basePath, { ...view, ...change });
		if (view.chosen) {
			const days = daysOf(view, timeZone, now).days;
			if (days) tags.push({ text: daysTag(days, timeZone, now), href: onCallHref(basePath, withDefaultDays(view)) });
		} else if (view.preset === TONIGHT_DAYS) {
			tags.push({ text: 'Tonight', href: onCallHref(basePath, withDefaultDays(view)) });
		} else if (view.preset === SHORT_ON_CALL_DAYS) {
			tags.push({ text: `Next ${SHORT_ON_CALL_DAYS} days`, href: onCallHref(basePath, withDefaultDays(view)) });
		}
		if (view.isMine) tags.push({ text: 'Mine', href: without({ isMine: false }) });
		if (view.needsCover) tags.push({ text: NEEDS_COVER, href: without({ needsCover: false }) });
		if (view.doula) tags.push({ text: `Doula: ${doulaNameOf(view.doula)}`, href: without({ doula: '' }) });
		return tags;
	});

	// The days one step each way from the ones the rows show.
	const earlier = $derived(onCallStep(view, -1, timeZone, now));
	const later = $derived(onCallStep(view, 1, timeZone, now));
	function step(event: MouseEvent, to: OnCallView) {
		event.preventDefault();
		void show(onCallHref(basePath, to));
	}

	function removeTag(event: MouseEvent, tag: AppliedTag) {
		event.preventDefault();
		void show(tag.href);
	}

	/*
	 * What the rows on screen were read under. While a newer narrowing loads
	 * they still answer to the last one, so the result line and the empty
	 * state never describe rows that are not there.
	 */
	const resultSummary = $derived.by(() => {
		if (!roster || !data?.filters) return '';
		const shown = data.view;
		const isPicked = shown.doula !== '' && !(shown.isMine && shown.doula === session.staffId);
		return onCallResultSummary(
			roster,
			data.filters,
			timeZone,
			{
				isMine: shown.isMine,
				doulaName: isPicked ? doulaNameOf(shown.doula) : undefined,
				staffId: data.filters.staffId,
				needsCover: shown.needsCover,
				order: shown.order
			},
			now
		);
	});
	const resultRuler = $derived(onCallResultRuler(knownDoulas.map((doula) => doula.name).toSorted((a, b) => b.length - a.length)[0] ?? ''));
	// Said once, under the result line, and only where a clock time is on the screen.
	const hasClock = $derived(
		roster !== undefined && (roster.windows.some((window) => window.gaps.length > 0) || roster.doulas.some((doula) => doula.unreachable.length > 0))
	);
	const zoneNote = $derived(hasClock ? practiceZoneNote(true, now) : undefined);
	const isBusy = $derived(isApplying || (data !== undefined && !isReadLanded && readFailure === ''));
	const isEmpty = $derived(roster !== undefined && roster.windows.length === 0);

	// H10: the way on from an empty result, by what narrowed it.
	const emptyAction = $derived.by((): EmptyStateAction | undefined => {
		if (!data) return undefined;
		const next = onCallEmptyNextStep(data.view);
		return next ? { label: next.label, href: onCallHref(basePath, next.view) } : undefined;
	});

	// "Try again" runs the read that failed, under the same narrowing.
	let isRetrying = $state(false);
	async function tryAgain() {
		isRetrying = true;
		try {
			await onRetry();
		} finally {
			isRetrying = false;
		}
	}

	const doulaOptions = $derived([{ value: '', label: ANY_DOULA }, ...doulaChoices]);
	const DAYS_OPTIONS: readonly { value: OnCallDaysChoice; label: string }[] = [
		{ value: 'tonight', label: 'Tonight' },
		{ value: '7', label: `Next ${SHORT_ON_CALL_DAYS} days` },
		{ value: '30', label: `Next ${DEFAULT_ON_CALL_DAYS} days` },
		{ value: 'choose', label: 'Choose days' }
	];

	const say = (day: string) => formatCalendarWeekdayDay(day, timeZone, now);
	const isTonightShown = $derived(daysOf(view, timeZone, now).days?.from === today);
	const orderOptions = $derived(onCallOrderOptions(isTonightShown));
	// The rows answer to the days they were read under, which a newer address may not yet be.
	const isGrouped = $derived(data?.filters !== undefined && roster !== undefined && data.filters.from === roster.today);
	const shownOrder = $derived(data?.view.order ?? '');

	const engagementHref = (row: { engagementId: string }) =>
		resolve('/practices/[practiceId]/engagements/[engagementId]', { practiceId, engagementId: row.engagementId });
	const birthHref = (row: { engagementId: string }) => `${engagementHref(row)}#on-call`;
	const gapsByNeed = (row: RosterWindow) =>
		row.gaps.toSorted((a, b) => Number(a.coveringStaffId !== undefined) - Number(b.coveringStaffId !== undefined));
	const coveringNames = $derived(
		new Map(roster?.windows.flatMap((row) => row.gaps.map((gap) => [gap.id, gap.coveringStaffName] as const)))
	);
	const coverWords = (gap: UnreachableGap) =>
		gap.isCovered ? (coveringNames.get(gap.gapId) ? `${coveringNames.get(gap.gapId)} covering` : 'covered') : 'nobody covering';
	const atOnceNames =(doula: RosterDoula, ids: string[]) =>
		ids
			.map((id) => doula.births.find((birth) => birth.engagementId === id)?.clientName)
			.filter((name): name is string => name !== undefined)
			.join(' and ');

	const columns = $derived([
		{ label: 'Client', accessor: (row: RosterWindow) => row.clientName, track: '15.2ch' },
		{
			label: 'On call',
			track: 'calc(26ch + 2 * var(--data-table-cell-inset))',
			accessor: (row: RosterWindow) => row.onCall.map((doula) => doula.name).join(', ') || NO_DOULA,
			content: onCallCell
		},
		{
			label: 'Days on call',
			track: 'calc(24ch + 2 * var(--data-table-cell-inset))',
			accessor: (row: RosterWindow) => daysOnCallText(row.window, today, timeZone, now),
			content: daysCell
		},
		{
			label: 'Due',
			track: 'calc(11ch + 2 * var(--data-table-cell-inset))',
			accessor: (row: RosterWindow) => (row.dueDate ? say(row.dueDate) : ''),
			content: dueCell
		},
		{
			label: 'Cover',
			grows: true,
			track: 'calc(36ch + 2 * var(--data-table-cell-inset))',
			accessor: describeCoverage,
			content: coverCell
		}
	]);

	const noWindowColumns = [
		{ label: 'Client', accessor: (row: NoWindowRow) => row.clientName },
		{
			label: 'Doula',
			accessor: (row: NoWindowRow) => row.doulas?.map((doula) => doula.name).join(', ') || NO_DOULA
		}
	];

	const doulaColumns = [
		{
			label: 'Doula',
			accessor: (doula: RosterDoula) => (doula.staffId === session.staffId ? 'You' : doula.name)
		},
		{
			label: 'On call for',
			accessor: (doula: RosterDoula) =>
				doula.births.map((birth) => (birth.covering ? `${birth.clientName}, covering` : birth.clientName)).join(', ') || 'None',
			content: birthsCell
		},
		{
			label: 'Cannot be reached',
			accessor: (doula: RosterDoula) => doula.unreachable.map((gap) => gap.clientName).join(', '),
			content: unreachableCell
		}
	];

	/*
	 * The boxes keep their `min` and `max`, so the picker disables the days the
	 * roster does not serve; the browser's own refusal of a typed one is a
	 * tooltip in its words, and it stops the form before `onSubmit`, so the
	 * form leaves the check to the screen, which words it beside the fields.
	 */
	function wordsOfOurOwn(node: HTMLElement) {
		const form = node.closest('form');
		form?.setAttribute('novalidate', '');
		return () => form?.removeAttribute('novalidate');
	}

	const isDoulasShown = $derived(
		roster !== undefined && roster.doulas.length > 1 && data !== undefined && !data.view.isMine && data.view.doula === ''
	);
	const listedDoulas = $derived(
		(roster?.doulas ?? []).filter((doula) => doula.births.length > 0 || doula.unreachable.length > 0)
	);
	const idleDoulas = $derived(
		(roster?.doulas ?? []).filter((doula) => doula.births.length === 0 && doula.unreachable.length === 0).map((doula) => doula.name)
	);
	const idleLine = $derived(
		idleDoulas.length === 0
			? ''
			: `On call for no birth in these days: ${idleDoulas.length === 1 ? idleDoulas[0] : `${idleDoulas.slice(0, -1).join(', ')} and ${idleDoulas.at(-1)}`}.`
	);
</script>

{#snippet days(range: DayRange, lead: string)}
	{@const parts = daysOnCall(range, today)}
	{#if 'until' in parts}
		{lead} <time class="day" datetime={parts.until}>{say(parts.until)}</time>
	{:else if 'day' in parts}
		<time class="day" datetime={parts.day}>{say(parts.day)}</time>
	{:else}
		<time class="day" datetime={parts.from}>{say(parts.from)}</time> to <time class="day" datetime={parts.to}>{say(parts.to)}</time>
	{/if}
{/snippet}

{#snippet onCallCell(row: RosterWindow, tableView: DataTableView)}
	{@const isCard = tableView === 'record'}
	<div class="lines">
		{#if isCard}
			{@render coverBlock(row)}
		{/if}
		{#if row.noDoula}
			<Text text={NO_DOULA} tone="muted" />
		{:else}
			<ul class="lines">
				{#each row.onCall as doula (doula.staffId)}
					<li>
						<span class="who">
							<span class="name">{#if isTonightOnHerLine(row)}{TONIGHT_LABEL}{/if}{doula.name}{#if isCard && doula.narrowed}{SEPARATOR}{@render days({ start: doula.from, end: doula.to }, 'until')}{/if}</span>
							{#if doula.atOnce && doula.atOnce.length > 0}
								<Badge label={atOnceLabel(doula.atOnce.length)} variant="warning" />
								{#if isCard}
									{#each doula.atOnce as other (other.engagementId)}
										<span class="line">Also {other.clientName}, {@render days(other.days, 'until')}</span>
									{/each}
								{/if}
							{/if}
						</span>
						{#if doula.narrowed && !isCard}
							<span class="line">{@render days({ start: doula.from, end: doula.to }, 'Until')}</span>
						{/if}
						{#if !isCard}
							{#each doula.atOnce ?? [] as other (other.engagementId)}
								<span class="line">Also {other.clientName}, {@render days(other.days, 'until')}</span>
							{/each}
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
	{#if tonightLine(row) && !isTonightOnHerLine(row)}
		<p class="line tonight">{tonightLine(row)}</p>
	{/if}
{/snippet}

{#snippet daysCell(row: RosterWindow, tableView: DataTableView)}
	<div class="lines">
		<span>
			{#if tableView === 'record'}{ON_CALL_LABEL}{/if}{@render days(row.window, tableView === 'record' ? 'until' : 'Until')}
		</span>
		{#if row.ownStart}
			<span class="line wraps">{ownStartWords(row.ownStart, roster!.rule)}</span>
		{/if}
	</div>
{/snippet}

{#snippet dueCell(row: RosterWindow, tableView: DataTableView)}
	{#if row.dueDate}
		{#if tableView === 'record'}{DUE_LABEL}{/if}<time class="day" datetime={row.dueDate}>{say(row.dueDate)}</time>
	{/if}
{/snippet}

{#snippet coverCell(row: RosterWindow, tableView: DataTableView)}
	{#if tableView === 'record'}
		<span class="moved"></span>
	{:else}
		{@render coverBlock(row)}
	{/if}
{/snippet}

{#snippet coverBlock(row: RosterWindow)}
	<div class="lines cover">
		{#if row.uncovered}
			<Badge label={describeCoverage(row)} variant="warning" />
		{:else}
			<span>{describeCoverage(row)}</span>
		{/if}
		{#if !row.noDoula}
			{#each row.unstaffedDays as hole (hole.start)}
				<span class="line">Nobody on call {@render holeDays(hole)}</span>
			{/each}
			{#each gapsByNeed(row) as gap (gap.id)}
				<span class="line">
					{gapSubject(gap, session.staffId)} cannot be reached <CoverageGapWhen startsAt={gap.startsAt} endsAt={gap.endsAt} {timeZone} {now} />, {gap.coveringStaffName
						? `${gap.coveringStaffName} covering`
						: 'nobody covering'}
				</span>
			{/each}
		{/if}
	</div>
{/snippet}

{#snippet holeDays(range: DayRange)}
	<time class="day" datetime={range.start}>{say(range.start)}</time>{#if range.start !== range.end}{TO}<time class="day" datetime={range.end}>{say(range.end)}</time>{/if}
{/snippet}

{#snippet birthsCell(doula: RosterDoula)}
	<div class="lines">
		<span>
			{#each doula.births as birth, index (birth.engagementId)}{index > 0 ? ', ' : ''}{birth.clientName}{birth.covering ? ', covering' : ''}{:else}None{/each}
		</span>
		{#each doula.atOnce ?? [] as group (group.engagementIds.join(','))}
			<Badge label={atOnceLabel(group.engagementIds.length - 1)} variant="warning" />
			<span class="line">{atOnceNames(doula, group.engagementIds)}, {@render days(group.days, 'until')}</span>
		{/each}
	</div>
{/snippet}

{#snippet unreachableCell(doula: RosterDoula)}
	{#if doula.unreachable.length > 0}
		<ul class="lines">
			{#each doula.unreachable as gap (gap.gapId)}
				<li>
					<CoverageGapWhen startsAt={gap.startsAt} endsAt={gap.endsAt} {timeZone} {now} />, {gap.clientName}, {coverWords(gap)}
				</li>
			{/each}
		</ul>
	{/if}
{/snippet}

<!-- The × comes first, as on the MOJ filter's tags; the name says what pressing it does. -->
{#snippet applied()}
	{#each appliedTags as tag (tag.text)}
		<li>
			<Link
				href={tag.href}
				label={tag.text}
				variant="chip"
				icon="x"
				ariaLabel={`Remove filter: ${tag.text.replaceAll(':', '')}`}
				onClick={(event) => removeTag(event, tag)}
			/>
		</li>
	{/each}
{/snippet}

{#snippet content()}
	<!-- The bar and the list it narrows are one group, held closer than the page's sections, as on the Schedule. -->
	<stack-l space="var(--space-related)">
		<FilterBar
			bind:this={filterBar}
			action={basePath}
			onSubmit={applyFilters}
			onChangeApply={applyOnChange}
			legend="Choose which births to show"
			status={readFailure ? '' : resultSummary}
			statusRuler={resultRuler}
			statusNote={readFailure ? undefined : zoneNote}
			statusId={RESULT_ID}
			empty={isEmpty}
			clearHref={isOnCallNarrowed(view) ? basePath : undefined}
			applied={appliedTags.length > 0 ? applied : undefined}
			appliedCount={appliedTags.length}
			hasUntaggedFilter={view.order !== ''}
			foldsFieldsOnly
			foldLabel={canMine || isOwnerView ? undefined : 'Order'}
			order={{ options: orderOptions, value: view.order, onChange: (value) => (draftOrder = value as OnCallOrder) }}
			{isBusy}
		>
			{#snippet switches()}
				<RadioGroup legend="Days" name="days" options={DAYS_OPTIONS} value={draftChoice} onChange={chooseDays} isInline>
					{#snippet trailing()}
						<!-- Each steps the days the reader sees by their own length; the days it lands on are in its name. Earlier keeps its box at today, so Later never moves. -->
						<span class="steps">
							<span class:held={!earlier}>
								<Link
									href={onCallHref(basePath, earlier?.view ?? view)}
									label="Earlier"
									ariaLabel={earlier ? `Earlier, ${daysWords(earlier.days, timeZone, now)}` : undefined}
									onClick={(event) => earlier && step(event, earlier.view)}
								/>
							</span>
							{#if later}
								<Link
									href={onCallHref(basePath, later.view)}
									label="Later"
									ariaLabel={`Later, ${daysWords(later.days, timeZone, now)}`}
									onClick={(event) => step(event, later.view)}
								/>
							{/if}
						</span>
					{/snippet}
				</RadioGroup>
			{/snippet}
			{#snippet folded()}
				{#if canMine || isOwnerView}
					<!-- One group, so each box reads as the end of "Show only". The legend is drawn on the boxes' own row. -->
					<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- a named box of a GET form beside the bar's own fields: CheckboxGroup stacks one option to a line and carries its value as an array -->
					<fieldset>
						<legend>Show only</legend>
						<div class="show-only-boxes">
							{#if canMine}
								<LabeledField label="Mine" orientation="inline">
									{#snippet children(control)}
										<Checkbox name="mine" value="true" checked={isMineChecked} onChange={(checked) => (isMineChecked = checked)} {...control} />
									{/snippet}
								</LabeledField>
							{/if}
							{#if isOwnerView}
								<LabeledField label={NEEDS_COVER} orientation="inline">
									{#snippet children(control)}
										<Checkbox
											name="cover"
											value="needed"
											checked={isNeedsCoverChecked}
											onChange={(checked) => (isNeedsCoverChecked = checked)}
											{...control}
										/>
									{/snippet}
								</LabeledField>
							{/if}
						</div>
					</fieldset>
				{/if}
				{#if isOwnerView}
					<div class="field doula">
						<LabeledField label="Doula" width={30}>
							{#snippet children({ id, describedBy, invalid, width })}
								<Select {id} {describedBy} {invalid} {width} name="doula" options={doulaOptions} disabled={knownDoulas.length === 0} bind:value={draftDoula} />
							{/snippet}
						</LabeledField>
					</div>
				{/if}
			{/snippet}
			{#if isChoosingDays}
				<div class="range-slot" {@attach wordsOfOurOwn}>
					<DateRangeFields
						bind:from={fromValue}
						bind:to={toValue}
						fromError={rangeErrors.from}
						toError={rangeErrors.to}
						hint={`Both days are included. Up to ${MOST_ON_CALL_DAYS} days, starting today or later.`}
						earliest={today}
						latest={isCalendarDay(fromValue) ? addDays(fromValue, MOST_ON_CALL_DAYS - 1) : undefined}
					/>
				</div>
			{/if}
		</FilterBar>

		<!-- T8: in place of the list, so no row from another narrowing is read as this one's; the bar keeps the narrowing. -->
		{#if readFailure}
			<stack-l space="var(--space-related)">
				<Notice message={readFailure} variant="error" />
				<Button label="Try again" onClick={tryAgain} loading={isRetrying} />
			</stack-l>
		{:else if data}
			{#if roster && !data.rangeErrors}
				<div class="births">
					<DataTable
						{columns}
						rows={roster.windows}
						rowHref={birthHref}
						rowId={(row, tableView) => rowDomId(row.engagementId, tableView)}
						hasMore={false}
						emptyMessage={resultSummary}
						{emptyAction}
						isEmptyMessageSpoken={false}
						caption="Births on call"
						describedBy={RESULT_ID}
						columnWidths="one-line"
						recordLayout="compact"
						recordColumns="one"
						rowGroup={isGrouped ? (row) => onCallGroup(row, shownOrder, roster.today, timeZone, now) : undefined}
						groupHeadingLevel={2}
						orderedBy={shownOrder === COVER_FIRST ? undefined : { column: 'Due', direction: 'ascending' }}
						{isBusy}
					/>
				</div>

				<div class="rule">
					<Text text={onCallRuleSentence(roster.rule)} tone="muted" />
					{#if isOwnerView}
						<Link href={resolve('/practices/[practiceId]/settings/on-call', { practiceId })} label="Change the on-call rule" />
					{/if}
				</div>

				{#if roster.noWindow.length > 0}
					<section>
						<Heading level={2} text="No days on call yet" />
						<Text text="These births have no due date, so they have no on-call days. Add the due date on the birth's page." tone="muted" />
						<DataTable
							columns={noWindowColumns}
							rows={roster.noWindow}
							rowHref={engagementHref}
							hasMore={false}
							emptyMessage="None."
							caption={`Births with no days on call yet (${roster.noWindow.length})`}
						/>
					</section>
				{/if}

				{#if isDoulasShown}
					<section>
						<Heading level={2} text="Doulas" />
						{#if listedDoulas.length > 0}
							<DataTable
								columns={doulaColumns}
								rows={listedDoulas}
								hasMore={false}
								emptyMessage="None."
								caption={`Doulas (${listedDoulas.length})`}
							/>
						{/if}
						{#if idleLine}
							<Text text={idleLine} tone="muted" />
						{/if}
					</section>
				{/if}
			{/if}
		{:else}
			<Skeleton variant="row" lines={6} label="Loading who is on call" />
		{/if}
	</stack-l>
{/snippet}

<svelte:document onvisibilitychange={refreshNow} />

<ListPage title="On call" {content} />

<style>
	@layer components {
		/* The boxes wrap among themselves beside FilterBar's floated legend. */
		.show-only-boxes {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-related);
		}

		/* The Doula takes half of a narrow line and stands at its own width where there is room, as on the Schedule. */
		.field {
			--field-width: 34ch;
			flex: 1 1 min(var(--field-width), calc(50% - var(--space-related) / 2));
			max-inline-size: var(--field-width);
		}

		/* On a card the cover is drawn under the Client's name, in the On call field; its own field, left empty, is not drawn. */
		.births :global(.compact-field:has(.moved)) {
			display: none;
		}

		/* A birth's row is several lines tall and its link sits mid-row, so the restored row as a whole, not only its link, clears the stuck heading (and a group's heading above it). */
		.births :global(.table-view > tbody > tr[id]) {
			scroll-margin-block-start: calc(var(--row-height) + var(--border-thin));
		}

		.births :global(.table-view > tbody > tr.opens-group[id]) {
			scroll-margin-block-start: calc(2 * var(--row-height) + var(--border-thin));
		}

		.range-slot {
			display: contents;
		}

		.steps {
			display: inline-flex;
			gap: var(--space-related);
		}

		/* At today there is no earlier day: Earlier keeps its box, unseen and out of the tab order, so Later stays where it was. */
		.held {
			visibility: hidden;
		}

		section {
			display: flex;
			flex-direction: column;
			gap: var(--space-3);
		}

		.rule {
			display: flex;
			flex-wrap: wrap;
			align-items: baseline;
			gap: var(--space-1) var(--space-related);
		}

		.lines {
			margin: 0;
			padding: 0;
			list-style: none;
			display: flex;
			flex-direction: column;
			align-items: start;
			gap: var(--space-1);
		}

		.who {
			display: inline-flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-2);
		}

		/* What a cell adds under its first line: the days a Doula holds, the other birth, what is missing. */
		.line {
			display: block;
			color: var(--color-on-surface-variant);
			font-size: var(--text-small-size);
		}

		/* A sentence wraps inside the column's own width; only a day or a range of days holds one line. */
		.wraps {
			white-space: normal;
			inline-size: 24ch;
			max-inline-size: 100%;
		}

		p.line {
			margin: 0;
		}

		.day {
			white-space: nowrap;
			font-variant-numeric: tabular-nums;
		}
	}
</style>
