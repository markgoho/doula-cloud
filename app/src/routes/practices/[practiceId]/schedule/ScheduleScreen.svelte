<script lang="ts">
	/*
	 * The Practice-wide schedule (#263). Before it, "who is covering whom,
	 * and when" had no surface at all: a Staff member opened one
	 * Engagement page per Client and held the answer in her head.
	 *
	 * The Clients list's and the Invoices list's sibling (#2747): `ListPage`
	 * for the frame, `FilterBar` for the narrowing and its applied tags,
	 * `DataTable` for the book. The narrowing is `FilterBar`'s real
	 * `<form method="get">`; `applyFilters` upgrades it to a client-side
	 * navigation rather than replacing it (Rule of Least Power), and the
	 * address is the state.
	 */
	import { afterNavigate, beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { tick, untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatScheduledVisit } from '#lib/dates.js';
	import { visitDoulaText } from '#lib/engagementDetail.js';
	import { keepListReturn, restoreList, type ListReturn } from '#lib/listReturn.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { canNarrowToMine, isOwnerOrAdmin } from '#lib/roles.js';
	import { readerTimeZone } from '#lib/zonedTime.js';
	import {
		DEFAULT_SCHEDULE_DAYS,
		SHORT_SCHEDULE_DAYS,
		isScheduleNarrowed,
		loadPracticeSchedule,
		scheduleDaysOf,
		scheduleDaysTag,
		scheduleDayGroup,
		scheduleEmptyNextStep,
		scheduleHref,
		scheduleRangeErrors,
		scheduleResultRuler,
		scheduleResultSummary,
		scheduleViewFromParameters,
		scheduleZoneNote,
		withDefaultDays,
		type ScheduleDaysChoice,
		type ScheduledVisit,
		type ScheduleRangeErrors,
		type ScheduleView
	} from '#lib/visitSchedule.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import type { EmptyStateAction } from '#lib/components/molecules/EmptyState.svelte';
	import DateRangeFields from '#lib/components/molecules/DateRangeFields.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import VisitDoula from '#lib/components/molecules/VisitDoula.svelte';
	import VisitWhen from '#lib/components/molecules/VisitWhen.svelte';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PracticeSession } from '../+layout.js';
	import type { SchedulePageData } from './+page.js';

	/*
	 * `data` is the read once it lands (#2255). Until then the screen draws
	 * what the URL and the session already say -- the filter bar -- and a
	 * row skeleton where the rows will land.
	 */
	let {
		data,
		isReading = false,
		hasReadFailed = false,
		onRetry = async () => {}
	}: {
		data?: SchedulePageData;
		// The page's read is running, so `data` is still the one before it.
		isReading?: boolean;
		// The page's read failed in a way that may work a second time
		// (`+page.svelte`); `onRetry` runs it again.
		hasReadFailed?: boolean;
		onRetry?: () => Promise<void>;
	} = $props();

	// What a person reads when a read fails: no status code and no server
	// text, and that nothing is lost (T8, as on the Clients and Invoices lists).
	const LOAD_FAILED = 'Your schedule did not load. Nothing you saved is lost.';
	const LOAD_MORE_FAILED = 'More Visits did not load. Nothing you saved is lost. Try again.';
	const RESULT_ID = 'schedule-result';
	const ANY_DOULA = 'Any Doula';
	const UNKNOWN_DOULA = 'that Doula';

	const practiceId = $derived(page.params.practiceId!);
	const basePath = $derived(resolve('/practices/[practiceId]/schedule', { practiceId: page.params.practiceId! }));
	const session = $derived((page.data as { session: PracticeSession }).session);
	// The Practice's own zone (ADR-0036): every day and time on this screen is read in it, never the reader's.
	const timeZone = $derived(session.timezone);
	const canMine = $derived(canNarrowToMine(session));
	// The picker's gate is the roster read's (`+page.ts`), so it holds its
	// place before the Doulas land.
	const isDoulaPickerOffered = $derived(isOwnerOrAdmin(session));
	const view = $derived(
		scheduleViewFromParameters(
			page.url.searchParams,
			canMine,
			isDoulaPickerOffered ? undefined : { staffId: session.staffId }
		)
	);
	const read = $derived(data);
	const viewKey = (of: ScheduleView) => scheduleHref('', of);

	/*
	 * The instant "Today" is read from: the landed read's own, so a heading and
	 * the days cannot disagree, and a fresh one when the tab comes back to the
	 * front, so a heading left open past midnight does not keep saying Today.
	 */
	let now = $state(untrack(() => read?.now) ?? new Date());
	function refreshNow() {
		if (document.visibilityState === 'visible') now = new Date();
	}

	const doulas = $derived(read?.doulas ?? []);
	const doulaNameOf = (id: string) => doulas.find((doula) => doula.value === id)?.label ?? UNKNOWN_DOULA;

	/*
	 * Grows from the landed read's first page. `loadPage` reads the landed
	 * read's own filters, so "Load more" continues the narrowing its rows
	 * came from.
	 */
	const schedule = new PaginatedList<ScheduledVisit>({
		first: untrack(() => read?.page) ?? { items: [], hasMore: false },
		loadPage: async (cursor) => {
			try {
				return await loadPracticeSchedule(apiFetchWithSession, page.params.practiceId!, read!.filters, cursor);
			} catch {
				throw new Error(LOAD_MORE_FAILED);
			}
		},
		failureMessage: LOAD_MORE_FAILED
	});

	/*
	 * Leaving this entry keeps how many pages were open, how far down she
	 * was and the Visit she left for (`keepListReturn`); Back reads as many
	 * pages again under the same narrowing, then puts her where she was,
	 * with focus on that row. The place is held until the read for this
	 * entry lands, and any move before it lands drops it.
	 */
	let pagesLoaded = 1;
	let heldReturn: ListReturn | undefined;
	let navigations = 0;
	keepListReturn({
		id: 'schedule-list',
		pagesLoaded: () => pagesLoaded,
		rowOf: (to) => (to.hash.startsWith('#visit-') ? to.hash.slice('#visit-'.length) : undefined),
		restore: (back) => {
			if (hasReadFailed) return;
			heldReturn = back;
			if (!isReading && read && viewKey(read.view) === viewKey(view)) void restoreReturn(back);
		}
	});
	beforeNavigate(() => {
		heldReturn = undefined;
		navigations += 1;
	});

	async function didLoadMore(): Promise<boolean> {
		const before = schedule.items.length;
		await schedule.loadMore();
		if (schedule.items.length === before) return false;
		pagesLoaded += 1;
		return true;
	}

	async function restoreReturn(back: ListReturn) {
		heldReturn = undefined;
		await restoreList(back, {
			pagesLoaded: () => pagesLoaded,
			navigations: () => navigations,
			loadMore: didLoadMore,
			inFlight: () => schedule.inFlight,
			rowLink
		});
	}

	const rowDomId = (visitId: string, of: DataTableView) => `schedule-row-${visitId}-${of}`;

	// Both trees are in the DOM (DataTable's #666); the displayed one wins.
	function rowLink(visitId: string): HTMLElement | undefined {
		return (
			(['table', 'record'] as const)
				.map((of) => document.querySelector<HTMLElement>(`#${rowDomId(visitId, of)} a`))
				.find((link) => link?.checkVisibility()) ?? undefined
		);
	}

	/*
	 * A narrowing change is a navigation, so `load` runs again and hands
	 * this component a fresh first page. `PaginatedList` owns its own
	 * items, so without this it would keep the previous narrowing's rows.
	 * `reset` abandons anything in flight at the same time.
	 */
	$effect(() => {
		if (hasReadFailed) heldReturn = undefined;
	});
	$effect(() => {
		const landed = read;
		if (!landed) return;
		untrack(() => {
			schedule.reset(landed.page);
			now = landed.now;
			pagesLoaded = 1;
			if (heldReturn) void restoreReturn(heldReturn);
		});
	});

	/*
	 * What the controls hold before Apply, put back in step with the address
	 * after every navigation. "Choose days" opens the From and To fields; a
	 * range the address names that is not one is said in place, beside them.
	 */
	let draftChoice = $state<ScheduleDaysChoice>('30');
	let fromValue = $state('');
	let toValue = $state('');
	let isMineChecked = $state(false);
	let draftDoula = $state('');
	let rangeErrors = $state<ScheduleRangeErrors>({});

	function seedDrafts() {
		draftChoice = view.chosen ? 'choose' : (view.preset === SHORT_SCHEDULE_DAYS ? '7' : '30');
		const shown = view.chosen ?? scheduleDaysOf(view, timeZone).days!;
		fromValue = shown.from;
		toValue = shown.to;
		isMineChecked = view.isMine;
		draftDoula = view.doula;
		rangeErrors = view.chosen ? scheduleRangeErrors(view.chosen.from, view.chosen.to) : {};
	}
	seedDrafts();
	let filterBar = $state<{ focusStatus: () => void }>();
	let isFocusPending = $state(false);
	let lastKey: string | undefined;
	afterNavigate(() => {
		seedDrafts();
		// A change of narrowing lands on the result line once its list has
		// loaded, never on first arrival, where the route announcer speaks.
		if (lastKey !== undefined && lastKey !== viewKey(view)) isFocusPending = true;
		lastKey = viewKey(view);
	});
	$effect(() => {
		if (!isFocusPending || !read || viewKey(read.view) !== viewKey(view)) return;
		isFocusPending = false;
		filterBar?.focusStatus();
	});

	// Picking "Choose days" starts its fields on the days the choice before it showed.
	function chooseDays(choice: ScheduleDaysChoice) {
		if (choice === 'choose' && draftChoice !== 'choose') {
			const shown = scheduleDaysOf(
				{ ...view, chosen: undefined, preset: draftChoice === '7' ? SHORT_SCHEDULE_DAYS : DEFAULT_SCHEDULE_DAYS },
				timeZone
			).days!;
			fromValue = shown.from;
			toValue = shown.to;
		}
		draftChoice = choice;
	}

	const draftView = $derived<ScheduleView>({
		preset: draftChoice === '7' ? SHORT_SCHEDULE_DAYS : DEFAULT_SCHEDULE_DAYS,
		chosen: draftChoice === 'choose' ? { from: fromValue, to: toValue } : undefined,
		isMine: canMine && isMineChecked,
		doula: draftDoula
	});

	const isChoosingDays = $derived(draftChoice === 'choose');

	let isApplying = $state(false);
	// A move to another narrowing: a page already being added is abandoned,
	// so it never lands in the new list, and Apply answers the press at once.
	async function show(href: string) {
		schedule.abandon();
		isApplying = true;
		try {
			await goto(href);
		} finally {
			isApplying = false;
		}
	}

	async function applyFilters(event: SubmitEvent) {
		event.preventDefault();
		if (draftChoice === 'choose') {
			const problems = scheduleRangeErrors(fromValue, toValue);
			if (problems.from || problems.to) {
				rangeErrors = problems;
				await tick();
				document.querySelector<HTMLInputElement>(problems.from ? 'input[name="from"]' : 'input[name="to"]')?.focus();
				return;
			}
		}
		rangeErrors = {};
		await show(scheduleHref(basePath, draftView));
	}

	/*
	 * The GOV.UK "selected filters": one removable tag for each narrowing
	 * the address names, each a link to the list minus that one. The default
	 * days draw no tag; "Clear filters" goes to the bare path.
	 */
	type AppliedTag = { text: string; href: string };
	const appliedTags = $derived.by(() => {
		const tags: AppliedTag[] = [];
		const without = (change: Partial<ScheduleView>) => scheduleHref(basePath, { ...view, ...change });
		if (view.chosen) {
			const days = scheduleDaysOf(view, timeZone).days;
			if (days) tags.push({ text: scheduleDaysTag(days, timeZone, now), href: scheduleHref(basePath, withDefaultDays(view)) });
		} else if (view.preset === SHORT_SCHEDULE_DAYS) {
			tags.push({ text: `Next ${SHORT_SCHEDULE_DAYS} days`, href: scheduleHref(basePath, withDefaultDays(view)) });
		}
		if (view.isMine) tags.push({ text: 'Mine', href: without({ isMine: false }) });
		if (read && view.doula) tags.push({ text: `Doula: ${doulaNameOf(view.doula)}`, href: without({ doula: '' }) });
		return tags;
	});

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
		if (!read || read.rangeErrors) return '';
		const shown = read.view;
		const isMine = shown.isMine;
		const isPicked = shown.doula !== '' && !(isMine && shown.doula === session.staffId);
		return scheduleResultSummary(
			schedule.items.length,
			schedule.hasMore,
			read.filters,
			timeZone,
			{ isMine, doulaName: isPicked ? doulaNameOf(shown.doula) : undefined },
			now,
			read.page.visitCount
		);
	});
	const resultRuler = $derived(
		scheduleResultRuler(doulas.map((doula) => doula.label).toSorted((a, b) => b.length - a.length)[0] ?? '')
	);
	// Said once, under the result line, only to a reader whose own zone reads these times differently.
	const zoneNote = $derived(scheduleZoneNote(timeZone, readerTimeZone(), now));
	const isBusy = $derived(isApplying || (read !== undefined && viewKey(read.view) !== viewKey(view) && !hasReadFailed));
	const isEmpty = $derived(read !== undefined && !read.rangeErrors && schedule.items.length === 0);

	// H10: the way on from an empty result, by what narrowed it.
	const emptyAction = $derived.by((): EmptyStateAction | undefined => {
		const step = read ? scheduleEmptyNextStep(read.view) : undefined;
		return step && { label: step.label, href: scheduleHref(basePath, step.view) };
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

	// The roster has not landed and the address names a Doula: the select holds her id, with no name to claim.
	const isDoulaPending = $derived(!read && !hasReadFailed && view.doula !== '');
	const doulaOptions = $derived(isDoulaPending ? [{ value: view.doula, label: '' }] : [{ value: '', label: ANY_DOULA }, ...doulas]);
	const DAYS_OPTIONS: readonly { value: ScheduleDaysChoice; label: string }[] = [
		{ value: '7', label: `Next ${SHORT_SCHEDULE_DAYS} days` },
		{ value: '30', label: `Next ${DEFAULT_SCHEDULE_DAYS} days` },
		{ value: 'choose', label: 'Choose days' }
	];

	/*
	 * Client first, because `DataTable` puts the row's link on its first
	 * column and only there: a link named "Sep 12, 2026, 10:30am" says
	 * nothing about where it goes, and every other Practice-wide list in
	 * the product already puts the way in on the Client's name. The `When`
	 * stays a real `<time datetime>` carrying the exact instant underneath
	 * the words (ADR-0022). The list's ORDER is still the Visit's instant,
	 * which is what #263 asks for; that is the endpoint's `ORDER BY`, not a
	 * column position.
	 */
	const columns = [
		{ label: 'Client', accessor: (visit: ScheduledVisit) => visit.clientName, grows: true },
		{ label: 'When', accessor: (visit: ScheduledVisit) => formatScheduledVisit(visit.scheduledAt, timeZone), content: whenCell },
		{
			label: 'Doula',
			accessor: (visit: ScheduledVisit) => visitDoulaText(visit.staffName, visit.isOffEngagement),
			content: doulaCell
		}
	];

	function engagementHref(visit: ScheduledVisit): string {
		const engagement = resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId,
			engagementId: visit.engagementId
		});
		return `${engagement}#visit-${visit.visitId}`;
	}

	// Two Visits of one Client are two links a list of links tells apart.
	const rowLinkLabel = (visit: ScheduledVisit) => `${visit.clientName}, ${formatScheduledVisit(visit.scheduledAt, timeZone)}`;
</script>

{#snippet whenCell(visit: ScheduledVisit, tableView: DataTableView)}
	<VisitWhen at={visit.scheduledAt} {timeZone} isTracked={tableView === 'table'} />
{/snippet}

{#snippet doulaCell(visit: ScheduledVisit, tableView: DataTableView)}
	<VisitDoula name={visit.staffName} isOffEngagement={visit.isOffEngagement} isTracked={tableView === 'table'} />
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
	<!-- The bar and the list it narrows are one group, held closer than the page's sections, as on the Clients list. -->
	<stack-l space="var(--space-related)">
		<FilterBar
			bind:this={filterBar}
			action={basePath}
			onSubmit={applyFilters}
			legend="Choose which Visits to show"
			status={hasReadFailed ? '' : resultSummary}
			statusRuler={resultRuler}
			statusNote={hasReadFailed ? undefined : zoneNote}
			statusId={RESULT_ID}
			empty={isEmpty}
			clearHref={isScheduleNarrowed(view) ? basePath : undefined}
			applied={appliedTags.length > 0 ? applied : undefined}
			appliedCount={appliedTags.length + (isDoulaPending ? 1 : 0)}
			foldsFieldsOnly
			{isBusy}
		>
			<!-- The Clients list's two rows: the days with their own visible question, then the fields and Apply. -->
			{#snippet switches()}
				<RadioGroup legend="Days" name="days" options={DAYS_OPTIONS} value={draftChoice} onChange={chooseDays} isInline />
				{#if canMine}
					<!-- One group, so each box reads as the end of "Show only". The legend is drawn on the boxes' own row. -->
					<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- a named box of a GET form beside the bar's own fields: CheckboxGroup stacks one option to a line and carries its value as an array -->
					<fieldset>
						<legend>Show only</legend>
						<div class="show-only-boxes">
							<LabeledField label="Mine" orientation="inline">
								{#snippet children(control)}
									<Checkbox name="mine" value="true" checked={isMineChecked} onChange={(checked) => (isMineChecked = checked)} {...control} />
								{/snippet}
							</LabeledField>
						</div>
					</fieldset>
				{/if}
			{/snippet}
			{#if isChoosingDays}
				<DateRangeFields
					bind:from={fromValue}
					bind:to={toValue}
					fromError={rangeErrors.from}
					toError={rangeErrors.to}
					hint="Both days are included."
				/>
			{/if}
			{#if isDoulaPickerOffered}
				<div class="field doula">
					<LabeledField label="Doula" width={30}>
						{#snippet children({ id, describedBy, invalid, width })}
							<Select
								{id}
								{describedBy}
								{invalid}
								{width}
								name="doula"
								options={doulaOptions}
								disabled={!read || read.isRosterUnavailable}
								bind:value={draftDoula}
							/>
						{/snippet}
					</LabeledField>
				</div>
			{/if}
		</FilterBar>

		<!-- T8: in place of the list, so no row from another narrowing is read as this one's; the bar keeps the narrowing. -->
		{#if hasReadFailed}
			<stack-l space="var(--space-related)">
				<Notice message={LOAD_FAILED} variant="error" />
				<Button label="Try again" onClick={tryAgain} loading={isRetrying} />
			</stack-l>
		{:else if read}
			{#if !read.rangeErrors}
				<div class="schedule-table">
					<DataTable
						{columns}
						rows={schedule.items}
						rowHref={engagementHref}
						rowId={(visit, tableView) => rowDomId(visit.visitId, tableView)}
						{rowLinkLabel}
						hasMore={schedule.hasMore}
						onLoadMore={didLoadMore}
						isLoadingMore={schedule.isLoadingMore}
						loadMoreError={schedule.loadMoreError}
						emptyMessage={resultSummary}
						{emptyAction}
						isEmptyMessageSpoken={false}
						caption="Visits"
						describedBy={RESULT_ID}
						columnWidths="one-line"
						recordLayout="compact"
						rowGroup={(visit) => scheduleDayGroup(visit.scheduledAt, timeZone, now)}
						groupHeadingLevel={2}
						{isBusy}
					/>
				</div>
			{/if}
		{:else}
			<Skeleton variant="row" lines={6} label="Loading your schedule" />
		{/if}
	</stack-l>
{/snippet}

<svelte:document onvisibilitychange={refreshNow} />

<ListPage title="Schedule" {content} />

<style>
	@layer components {
		/* The boxes wrap among themselves beside FilterBar's floated legend. */
		.show-only-boxes {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-related);
		}

		/* The Doula takes half of a narrow line and stands at its own width where there is room, as on the Clients list. */
		.field {
			--field-width: 34ch;
			flex: 1 1 min(var(--field-width), calc(50% - var(--space-related) / 2));
			max-inline-size: var(--field-width);
		}

		/* The Client's track, the Clients list's Name track (a long two-word name). */
		.schedule-table {
			/* The room a cell keeps either side, as on the Clients and Invoices lists. */
			--data-table-cell-inset: var(--space-2);
			/* The Client's track: the longest name the seed holds, so no narrowing is the one a name widens. */
			--data-table-grows-track: 15.2ch;
		}

		/* The Doula's track: a long name with the "No longer on this work" badge, so
		   the table is as wide with one Doula's Visits as with all of them. */
		.schedule-table :global(.table-view tr:not(.group-heading) > :is(th, td):last-child) {
			min-inline-size: 42ch;
		}

		/* Visits are a list in time order, so below the table they are one card
		   to a line, read down and never across, as the Clients list's are. */
		.schedule-table :global(.record-view) {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
